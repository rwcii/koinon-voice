package koinon

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/rwcii/koinon-voice/internal/signed"
)

// ErrNotDispatched is a call that failed before any byte of the request could reach the node:
// no connection was made. Only a first sending of a write may count it as not run.
var ErrNotDispatched = errors.New("the request was not sent: no connection to the node")

// The Serve route's bounds (Koinon contract dfb2ed7, Answer shapes): a request of at most
// 512 KiB and an answer of at most 1 MiB, the session's included.
const (
	MaxRequest = 512 << 10
	MaxAnswer  = 1 << 20
)

// CallTimeout bounds an ordinary call; a changes call waits its wait_seconds more. A node
// takes up to 15 seconds to connect to a paired computer and 15 more for its answer (Koinon
// #693 chunk 2, PROTOCOL.md "Other computers"), so the bound is above those 30 seconds.
const CallTimeout = 45 * time.Second

// OpenSignedHTTP opens a signed session with a node's Serve route (Koinon contract 4e324d5,
// D5.2), for a computer where no Koinon node runs. base is the node's Remote address, an
// https URL with no user, query or fragment. client may be nil; it is copied, never changed,
// and the copy follows no redirect.
func OpenSignedHTTP(ctx context.Context, opts SignedOptions, base string, client *http.Client) (*Signed, error) {
	u, err := checkBase(base)
	if err != nil {
		return nil, err
	}
	if err := signed.CheckPin(opts.NodePin); err != nil {
		return nil, err
	}
	c := http.Client{}
	if client != nil {
		c = *client
	}
	c.Timeout = 0 // each call has its own deadline
	c.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	s := &Signed{opts: opts}
	tctx, cancel := context.WithCancel(context.Background())
	h := &httpTransport{base: u, client: &c, ctx: tctx, cancel: cancel, done: make(chan struct{})}
	h.session = func() (signed.Envelope, error) {
		return s.envelope("session", opts.NodePin, map[string]any{"client": opts.Client}, "")
	}
	s.t = h
	sctx, cancel := context.WithTimeout(ctx, CallTimeout)
	defer cancel()
	if err := h.renew(sctx, -1); err != nil {
		h.Close()
		return nil, err
	}
	return s, nil
}

func checkBase(base string) (*url.URL, error) {
	u, err := url.Parse(base)
	if err != nil {
		return nil, errors.New("the node's Remote address is not a URL")
	}
	switch {
	case u.Scheme != "https":
		return nil, errors.New("the node's Remote address must be https")
	case u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Opaque != "":
		return nil, errors.New("the node's Remote address must be https://HOST, with no user, query or fragment")
	case u.Path != "" && u.Path != "/":
		return nil, errors.New("the node's Remote address must have no path")
	}
	u.Path = ""
	return u, nil
}

// httpTransport is the Serve form: a bearer token from a signed session, then one envelope
// per request.
type httpTransport struct {
	base    *url.URL
	client  *http.Client
	session func() (signed.Envelope, error)
	ctx     context.Context
	cancel  context.CancelFunc
	done    chan struct{}
	once    sync.Once

	mu       sync.Mutex // guards token, gen and inflight
	token    string
	gen      int
	inflight *renewal
}

// renewal is one session renewal that concurrent callers share.
type renewal struct {
	done chan struct{}
	err  error
}

func (h *httpTransport) Done() <-chan struct{} { return h.done }

func (h *httpTransport) Close() {
	h.once.Do(func() {
		h.cancel()
		close(h.done)
	})
}

// Call sends one envelope. A node answer of maintainer_token_expired, which the node gives
// before it dispatches anything, renews the session once and sends the same envelope again;
// no other answer or failure is retried here.
func (h *httpTransport) Call(ctx context.Context, e signed.Envelope, out any) error {
	ctx, cancel := h.callContext(ctx, e)
	defer cancel()
	h.mu.Lock()
	token, gen := h.token, h.gen
	h.mu.Unlock()
	err := h.post(ctx, "/v1/maintainer/signed", token, e, out)
	if !errors.Is(err, errTokenExpired) {
		return err
	}
	if err := h.renew(ctx, gen); err != nil {
		return err
	}
	h.mu.Lock()
	token = h.token
	h.mu.Unlock()
	return h.post(ctx, "/v1/maintainer/signed", token, e, out)
}

// callContext joins the caller's context with the transport's, and bounds the call: a
// changes wait gets its wait_seconds more.
func (h *httpTransport) callContext(ctx context.Context, e signed.Envelope) (context.Context, context.CancelFunc) {
	d := CallTimeout
	if e.Op == "changes" {
		if w, ok := e.Args["wait_seconds"].(int); ok {
			d += time.Duration(w) * time.Second
		}
		if w, ok := e.Args["wait_seconds"].(int64); ok {
			d += time.Duration(w) * time.Second
		}
	}
	ctx, cancel := context.WithTimeout(ctx, d)
	stop := context.AfterFunc(h.ctx, cancel)
	return ctx, func() { stop(); cancel() }
}

// renew signs a new session and stores its token. Callers that saw generation seen share
// one renewal: one that already happened since then is used as it is, and one in flight is
// waited for, within each caller's own context. seen -1 is the first session.
func (h *httpTransport) renew(ctx context.Context, seen int) error {
	h.mu.Lock()
	if seen >= 0 && h.gen != seen {
		h.mu.Unlock()
		return nil
	}
	if f := h.inflight; f != nil {
		h.mu.Unlock()
		select {
		case <-f.done:
			return f.err
		case <-ctx.Done():
			return fmt.Errorf("%w: waiting for the session: %v", errConnectionLost, ctx.Err())
		}
	}
	f := &renewal{done: make(chan struct{})}
	h.inflight = f
	h.mu.Unlock()

	token, err := h.newSession(ctx)
	h.mu.Lock()
	if err == nil {
		h.token, h.gen = token, h.gen+1
	}
	h.inflight = nil
	f.err = err
	h.mu.Unlock()
	close(f.done)
	return err
}

func (h *httpTransport) newSession(ctx context.Context) (string, error) {
	e, err := h.session()
	if err != nil {
		return "", err
	}
	// The session answer is flat, as /v1/maintainer/session is: {"ok": true, "token", ...}.
	var answer struct {
		Token string `json:"token"`
	}
	if err := h.post(ctx, "/v1/maintainer/signed/session", "", e, &flat{&answer}); err != nil {
		if errors.Is(err, errTokenExpired) {
			return "", fmt.Errorf("%w: the session route answered maintainer_token_expired", ErrProtocol)
		}
		return "", err
	}
	if answer.Token == "" {
		return "", fmt.Errorf("%w: the session answer has no token", ErrProtocol)
	}
	return answer.Token, nil
}

// post sends one envelope as JSON and reads Koinon's answer. The token appears in no error.
func (h *httpTransport) post(ctx context.Context, path, token string, e signed.Envelope, out any) error {
	body, err := json.Marshal(e)
	if err != nil {
		return err
	}
	if len(body) > MaxRequest {
		return fmt.Errorf("%w: the envelope is larger than %d bytes", ErrNotDispatched, MaxRequest)
	}
	u := *h.base
	u.Path = path
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	var connected bool
	var cmu sync.Mutex
	req = req.WithContext(httptrace.WithClientTrace(ctx, &httptrace.ClientTrace{
		GotConn: func(httptrace.GotConnInfo) { cmu.Lock(); connected = true; cmu.Unlock() },
	}))
	resp, err := h.client.Do(req)
	if err != nil {
		cmu.Lock()
		c := connected
		cmu.Unlock()
		msg := redactToken(err.Error(), token)
		if !c && ctx.Err() == nil {
			return fmt.Errorf("%w: %s", ErrNotDispatched, msg)
		}
		return fmt.Errorf("%w: %s", errConnectionLost, msg)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 && resp.StatusCode < 400 {
		return fmt.Errorf("%w: the node answered a redirect (%d), which is not followed", ErrProtocol, resp.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, MaxAnswer+1))
	if err != nil {
		return fmt.Errorf("%w: reading the answer: %s", errConnectionLost, redactToken(err.Error(), token))
	}
	if len(b) > MaxAnswer {
		return fmt.Errorf("%w: the answer is larger than %d bytes", ErrProtocol, MaxAnswer)
	}
	if mt, _, err := mime.ParseMediaType(resp.Header.Get("Content-Type")); err != nil || mt != "application/json" {
		return fmt.Errorf("%w: the node answered HTTP %d without JSON", ErrProtocol, resp.StatusCode)
	}
	err = decodeHTTP(path, resp.StatusCode, b, out)
	var r *Refusal
	if errors.As(err, &r) && token != "" {
		r.Message = redactToken(r.Message, token)
		r.Details = json.RawMessage(redactToken(string(r.Details), token))
	}
	return err
}

func redactToken(s, token string) string {
	if token == "" {
		return s
	}
	return strings.ReplaceAll(s, token, "[token]")
}

// flat marks an answer whose fields sit beside "ok" instead of in "result".
type flat struct{ v any }

// errTokenExpired is the node's HTTP 409 maintainer_token_expired, which it answers before it
// reads the envelope (Koinon contract dfb2ed7, Answer shapes): the only answer that renews.
// errConnectionLost is ErrClosed for the Serve form, whose stream is an HTTP connection to
// the node rather than the output of koinon mcp.
var errConnectionLost error = connectionLost{}

type connectionLost struct{}

func (connectionLost) Error() string {
	return "the connection to the node ended before its answer arrived"
}
func (connectionLost) Is(target error) bool { return target == ErrClosed }

var errTokenExpired = errors.New("maintainer_token_expired")

// expiredError marks a refusal as errTokenExpired. It renders from the refusal on each call,
// so the redaction that post applies to the refusal also holds for this error's text.
type expiredError struct{ r *Refusal }

func (e *expiredError) Error() string   { return errTokenExpired.Error() + ": " + e.r.Error() }
func (e *expiredError) Unwrap() []error { return []error{errTokenExpired, e.r} }

// decodeHTTP reads an answer of the Serve route strictly: one object with a boolean ok; on
// HTTP 200, ok true with a result (or, for the session, its flat fields); on HTTP 409 or
// 400, ok false with a code and an error; on HTTP 429, ok false with too_many_calls, the
// session's limit of actions in flight, refused before the envelope is read (Koinon #693
// chunk 3). Any other status or shape is a protocol error, never a recorded or refused write.
func decodeHTTP(what string, status int, b []byte, out any) error {
	var a struct {
		OK      *bool           `json:"ok"`
		Result  json.RawMessage `json:"result"`
		Code    string          `json:"code"`
		Error   string          `json:"error"`
		Details json.RawMessage `json:"details"`
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	if err := dec.Decode(&a); err != nil || a.OK == nil || dec.Decode(&struct{}{}) != io.EOF {
		return fmt.Errorf("%w: %s answered HTTP %d without a Koinon answer object", ErrProtocol, what, status)
	}
	if !*a.OK {
		busy := status == http.StatusTooManyRequests && a.Code == "too_many_calls"
		if (status != http.StatusConflict && status != http.StatusBadRequest && !busy) || a.Code == "" || a.Error == "" {
			return fmt.Errorf("%w: %s answered a refusal with HTTP %d", ErrProtocol, what, status)
		}
		r := &Refusal{Code: a.Code, Message: a.Error, Details: a.Details}
		if a.Code == "maintainer_token_expired" && status == http.StatusConflict {
			return &expiredError{r}
		}
		return r
	}
	if status != http.StatusOK {
		return fmt.Errorf("%w: %s answered ok with HTTP %d", ErrProtocol, what, status)
	}
	if f, ok := out.(*flat); ok {
		if err := json.Unmarshal(b, f.v); err != nil {
			return fmt.Errorf("%w: %s: %v", ErrProtocol, what, err)
		}
		return nil
	}
	if len(a.Result) == 0 || string(a.Result) == "null" {
		return fmt.Errorf("%w: %s answered ok without a result", ErrProtocol, what)
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(a.Result, out); err != nil {
		return fmt.Errorf("%w: %s result: %v", ErrProtocol, what, err)
	}
	return nil
}
