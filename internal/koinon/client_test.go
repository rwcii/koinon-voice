package koinon

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
	"time"
)

// pipeServer is a scripted server on the other end of a client's pipes.
type pipeServer struct {
	in  *bufio.Reader
	out io.WriteCloser
}

func newPipeClient(t *testing.T) (*Client, *pipeServer) {
	t.Helper()
	reqR, reqW := io.Pipe()
	ansR, ansW := io.Pipe()
	c := NewClient(ansR, reqW)
	t.Cleanup(func() { ansW.Close(); reqR.Close() })
	return c, &pipeServer{in: bufio.NewReader(reqR), out: ansW}
}

// request reads the next request and returns its ID and method.
func (s *pipeServer) request(t *testing.T) (int64, string) {
	t.Helper()
	line, err := s.in.ReadString('\n')
	if err != nil {
		t.Errorf("read request: %v", err)
		return 0, ""
	}
	var m struct {
		ID     int64  `json:"id"`
		Method string `json:"method"`
	}
	if err := json.Unmarshal([]byte(line), &m); err != nil {
		t.Errorf("request %q: %v", line, err)
	}
	return m.ID, m.Method
}

func (s *pipeServer) send(t *testing.T, v any) {
	t.Helper()
	raw, _ := json.Marshal(v)
	if _, err := s.out.Write(append(raw, '\n')); err != nil {
		t.Errorf("write: %v", err)
	}
}

func toolResult(text string, isError bool) map[string]any {
	return map[string]any{"content": []map[string]any{{"type": "text", "text": text}}, "isError": isError}
}

func TestClientAnswersAndNotices(t *testing.T) {
	c, s := newPipeClient(t)
	ctx := context.Background()
	done := make(chan error, 1)
	var page boardPage
	go func() { done <- c.Tool(ctx, "board_list", map[string]any{"filter": "active"}, &page) }()
	id, method := s.request(t)
	if method != "tools/call" {
		t.Fatalf("method %q", method)
	}
	// A notice before the answer reaches the notices, not the call.
	s.send(t, map[string]any{"jsonrpc": "2.0", "method": "notifications/resources/updated", "params": map[string]string{"uri": DecisionsURI}})
	s.send(t, map[string]any{"jsonrpc": "2.0", "method": "notifications/resources/updated", "params": map[string]string{"uri": DecisionsURI}})
	s.send(t, map[string]any{"jsonrpc": "2.0", "id": id, "result": toolResult(`{"ok":true,"result":{"counts":{"waiting":1},"items":[{"reference":"p-1","revision":2}],"more":false,"next_after":null}}`, false)})
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if page.Counts.Waiting != 1 || len(page.Items) != 1 || page.Items[0].Revision != 2 {
		t.Errorf("page %+v", page)
	}
	select {
	case <-c.Notice():
	case <-time.After(time.Second):
		t.Fatal("no notice")
	}
	if got := c.TakeNotices(); len(got) != 1 || got[0] != DecisionsURI {
		t.Errorf("notices %v", got)
	}
	if got := c.TakeNotices(); len(got) != 0 {
		t.Errorf("notices again %v", got)
	}
}

func TestClientRefusalsAndErrors(t *testing.T) {
	c, s := newPipeClient(t)
	ctx := context.Background()
	done := make(chan error, 1)
	go func() {
		done <- c.Tool(ctx, "decision_verdict", map[string]any{"reference": "p-1"}, nil)
	}()
	id, _ := s.request(t)
	s.send(t, map[string]any{"jsonrpc": "2.0", "id": id, "result": toolResult(`{"code":"stale_revision","details":{"current_revision":3},"error":"the decision item changed after the page showed it","ok":false}`, true)})
	err := <-done
	var r *Refusal
	if !errors.As(err, &r) || r.Code != "stale_revision" {
		t.Fatalf("refusal %v", err)
	}
	if rev, ok := r.CurrentRevision(); !ok || rev != 3 {
		t.Errorf("current revision %d %v", rev, ok)
	}

	go func() { done <- c.Subscribe(ctx, DecisionsURI) }()
	id, _ = s.request(t)
	s.send(t, map[string]any{"jsonrpc": "2.0", "id": id, "error": map[string]any{"code": -32603, "message": "daemon_unavailable", "data": map[string]string{"code": "daemon_unavailable"}}})
	err = <-done
	var e *RPCError
	if !errors.As(err, &e) || e.Code != -32603 || Code(err) != "daemon_unavailable" {
		t.Errorf("rpc error %v", err)
	}
}

func TestClientDeadline(t *testing.T) {
	c, s := newPipeClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- c.Tool(ctx, "peers", map[string]any{}, nil) }()
	s.request(t)
	if err := <-done; !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("deadline: %v", err)
	}
	c.mu.Lock()
	n := len(c.calls)
	c.mu.Unlock()
	if n != 0 {
		t.Errorf("%d calls left in flight", n)
	}
}

func TestClientEnds(t *testing.T) {
	for name, output := range map[string]string{
		"junk":   "Usage: koinon [flags]\n",
		"closed": "",
		"long":   strings.Repeat("x", 300) + "\n",
	} {
		t.Run(name, func(t *testing.T) {
			old := MaxLine
			MaxLine = 200
			t.Cleanup(func() { MaxLine = old })
			c, s := newPipeClient(t)
			done := make(chan error, 1)
			go func() { done <- c.Tool(context.Background(), "peers", map[string]any{}, nil) }()
			s.request(t)
			if output != "" {
				s.out.Write([]byte(output))
			} else {
				s.out.Close()
			}
			err := <-done
			want := ErrProtocol
			if name == "closed" {
				want = ErrClosed
			}
			if !errors.Is(err, want) {
				t.Errorf("error %v, want %v", err, want)
			}
			select {
			case <-c.Done():
			case <-time.After(time.Second):
				t.Fatal("client did not stop")
			}
			if err := c.Tool(context.Background(), "peers", map[string]any{}, nil); !errors.Is(err, want) {
				t.Errorf("after the end: %v", err)
			}
			if name == "junk" && strings.Contains(c.Err().Error(), "Usage") {
				t.Error("the error repeats the junk")
			}
		})
	}
}
