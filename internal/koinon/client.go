package koinon

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"sync"
)

// ProtocolVersion is the MCP version the client asks for; Koinon v0.5.0 answers with it.
const ProtocolVersion = "2025-06-18"

// MaxLine bounds one JSON-RPC line from the server, for clients made after it is set.
var MaxLine = 8 << 20

// ErrClosed is a stream that ended: the process exited or its output closed.
var ErrClosed = errors.New("koinon mcp output closed")

// ErrProtocol is output that is not JSON-RPC.
var ErrProtocol = errors.New("koinon mcp protocol")

// ErrStalled is a process that stopped reading its input: a write did not finish before
// the request's deadline. The client stops, so its session ends and the process is killed.
var ErrStalled = errors.New("koinon mcp stopped reading its input")

// Client speaks MCP as newline-delimited JSON-RPC 2.0 on a reader and a writer. It is safe
// for concurrent use. Notices arrive between answers; Notice signals them and TakeNotices
// returns each changed URI once.
type Client struct {
	w       io.Writer
	wmu     sync.Mutex
	maxLine int

	mu       sync.Mutex
	nextID   int64
	calls    map[int64]chan rpcMessage
	err      error
	done     chan struct{}
	notified map[string]bool
	notice   chan struct{}
}

type rpcMessage struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      *int64          `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *struct {
		Code    int             `json:"code"`
		Message string          `json:"message"`
		Data    json.RawMessage `json:"data"`
	} `json:"error,omitempty"`
}

// NewClient reads answers and notices from r until it ends, and writes requests to w.
func NewClient(r io.Reader, w io.Writer) *Client {
	c := &Client{
		w:        w,
		maxLine:  MaxLine,
		calls:    map[int64]chan rpcMessage{},
		done:     make(chan struct{}),
		notified: map[string]bool{},
		notice:   make(chan struct{}, 1),
	}
	go c.read(r)
	return c
}

// Done is closed when the client can take no more requests; Err then says why.
func (c *Client) Done() <-chan struct{} { return c.done }

// Err is why the client stopped, or nil while it runs.
func (c *Client) Err() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.err
}

// Notice receives a value when a notice has arrived since the last TakeNotices.
func (c *Client) Notice() <-chan struct{} { return c.notice }

// TakeNotices returns the URIs changed since the last call, in order, each once.
func (c *Client) TakeNotices() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	uris := make([]string, 0, len(c.notified))
	for u := range c.notified {
		uris = append(uris, u)
	}
	clear(c.notified)
	sort.Strings(uris)
	return uris
}

// Call sends a request and waits for its answer, its JSON-RPC error (*RPCError), the end
// of the stream or ctx.
func (c *Client) Call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	raw, err := json.Marshal(params)
	if err != nil {
		return nil, err
	}
	answer := make(chan rpcMessage, 1)
	c.mu.Lock()
	if c.err != nil {
		err := c.err
		c.mu.Unlock()
		return nil, err
	}
	c.nextID++
	id := c.nextID
	c.calls[id] = answer
	c.mu.Unlock()
	defer func() {
		c.mu.Lock()
		delete(c.calls, id)
		c.mu.Unlock()
	}()
	if err := c.send(ctx, rpcMessage{JSONRPC: "2.0", ID: &id, Method: method, Params: raw}); err != nil {
		return nil, err
	}
	select {
	case m := <-answer:
		if m.Error != nil {
			return nil, &RPCError{Code: m.Error.Code, Message: m.Error.Message, Data: m.Error.Data}
		}
		return m.Result, nil
	case <-c.done:
		return nil, c.Err()
	case <-ctx.Done():
		return nil, fmt.Errorf("%s: %w", method, ctx.Err())
	}
}

// Notify sends a notification, which has no answer.
func (c *Client) Notify(ctx context.Context, method string, params any) error {
	raw, err := json.Marshal(params)
	if err != nil {
		return err
	}
	return c.send(ctx, rpcMessage{JSONRPC: "2.0", Method: method, Params: raw})
}

// send writes a message, and stops the client with ErrStalled when the write is still
// blocked at ctx's end. The blocked write ends when its process's input is closed.
func (c *Client) send(ctx context.Context, m rpcMessage) error {
	written := make(chan error, 1)
	go func() { written <- c.write(m) }()
	select {
	case err := <-written:
		return err
	case <-c.done:
		return c.Err()
	case <-ctx.Done():
		c.fail(fmt.Errorf("%w: %s", ErrStalled, m.Method))
		return c.Err()
	}
}

// Initialize opens the MCP session.
func (c *Client) Initialize(ctx context.Context, version string) error {
	_, err := c.Call(ctx, "initialize", map[string]any{
		"protocolVersion": ProtocolVersion,
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]string{"name": "koinon-voice", "version": version},
	})
	if err != nil {
		return err
	}
	return c.Notify(ctx, "notifications/initialized", map[string]any{})
}

// Tool calls a tool and decodes Koinon's result into out. A refusal is a *Refusal.
func (c *Client) Tool(ctx context.Context, name string, args any, out any) error {
	raw, err := c.Call(ctx, "tools/call", map[string]any{"name": name, "arguments": args})
	if err != nil {
		return err
	}
	var r struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError"`
	}
	if err := json.Unmarshal(raw, &r); err != nil || len(r.Content) != 1 {
		return fmt.Errorf("%w: %s answered without one text content", ErrProtocol, name)
	}
	return decodeAnswer(name, r.Content[0].Text, r.IsError, out)
}

// ReadResource reads a resource and decodes Koinon's result into out.
func (c *Client) ReadResource(ctx context.Context, uri string, out any) error {
	raw, err := c.Call(ctx, "resources/read", map[string]string{"uri": uri})
	if err != nil {
		return err
	}
	var r struct {
		Contents []struct {
			Text string `json:"text"`
		} `json:"contents"`
	}
	if err := json.Unmarshal(raw, &r); err != nil || len(r.Contents) != 1 {
		return fmt.Errorf("%w: %s read without one content", ErrProtocol, uri)
	}
	return decodeAnswer(uri, r.Contents[0].Text, false, out)
}

// Subscribe asks for notices on a resource.
func (c *Client) Subscribe(ctx context.Context, uri string) error {
	_, err := c.Call(ctx, "resources/subscribe", map[string]string{"uri": uri})
	return err
}

// decodeAnswer reads Koinon's JSON answer: {"ok": true, "result": ...} or a refusal with
// its code.
func decodeAnswer(what, text string, isError bool, out any) error {
	var a struct {
		OK      bool            `json:"ok"`
		Result  json.RawMessage `json:"result"`
		Code    string          `json:"code"`
		Error   string          `json:"error"`
		Details json.RawMessage `json:"details"`
	}
	if err := json.Unmarshal([]byte(text), &a); err != nil {
		return fmt.Errorf("%w: %s answered text that is not JSON", ErrProtocol, what)
	}
	if isError || !a.OK {
		if a.Code == "" {
			a.Code = "refused"
		}
		return &Refusal{Code: a.Code, Message: a.Error, Details: a.Details}
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(a.Result, out); err != nil {
		return fmt.Errorf("%w: %s result: %v", ErrProtocol, what, err)
	}
	return nil
}

func (c *Client) write(m rpcMessage) error {
	line, err := json.Marshal(m)
	if err != nil {
		return err
	}
	c.wmu.Lock()
	defer c.wmu.Unlock()
	select {
	case <-c.done:
		return c.Err()
	default:
	}
	if _, err := c.w.Write(append(line, '\n')); err != nil {
		c.fail(fmt.Errorf("%w: write: %v", ErrClosed, err))
		return c.Err()
	}
	return nil
}

func (c *Client) read(r io.Reader) {
	br := bufio.NewReaderSize(r, 64<<10)
	for {
		line, err := readLine(br, c.maxLine)
		if err != nil {
			c.fail(err)
			return
		}
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		var m rpcMessage
		if err := json.Unmarshal(line, &m); err != nil {
			// The line is not repeated: it is output nobody asked for.
			c.fail(fmt.Errorf("%w: a line of %d bytes is not JSON", ErrProtocol, len(line)))
			return
		}
		switch {
		case m.ID != nil:
			c.mu.Lock()
			answer := c.calls[*m.ID]
			c.mu.Unlock()
			if answer != nil {
				answer <- m
			}
		case m.Method == "notifications/resources/updated":
			var p struct {
				URI string `json:"uri"`
			}
			if json.Unmarshal(m.Params, &p) == nil && p.URI != "" {
				c.mu.Lock()
				c.notified[p.URI] = true
				c.mu.Unlock()
				select {
				case c.notice <- struct{}{}:
				default:
				}
			}
		}
	}
}

// readLine reads one line of at most max bytes.
func readLine(br *bufio.Reader, max int) ([]byte, error) {
	var line []byte
	for {
		chunk, err := br.ReadSlice('\n')
		line = append(line, chunk...)
		if len(line) > max {
			return nil, fmt.Errorf("%w: a line is longer than %d bytes", ErrProtocol, max)
		}
		switch {
		case err == nil:
			return line, nil
		case errors.Is(err, bufio.ErrBufferFull):
			continue
		case errors.Is(err, io.EOF):
			return nil, ErrClosed
		default:
			return nil, fmt.Errorf("%w: %v", ErrClosed, err)
		}
	}
}

// fail stops the client with err unless it has stopped already.
func (c *Client) fail(err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.err != nil {
		return
	}
	c.err = err
	close(c.done)
}
