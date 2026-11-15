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

// CallTimeout bounds an ordinary call; a changes call waits its wait_seconds more.
const CallTimeout = 30 * time.Second

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
	if err := h.renew(ctx, ""); err != nil {
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

	mu    sync.Mutex // guards token; renewMu serializes renewals
	token string
	gen   int
	renMu sync.Mutex
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
	var r *Refusal
	if !errors.As(err, &r) || r.Code != "maintainer_token_expired" {
		return err
	}
	if err := h.renew(ctx, "", gen); err != nil {
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

// renew signs a new session and stores its token. Callers that saw generation gen renew it
// once together: a renewal that already happened since then is used as it is.
func (h *httpTransport) renew(ctx context.Context, _ string, seen ...int) error {
	h.renMu.Lock()
	defer h.renMu.Unlock()
	h.mu.Lock()
	if len(seen) == 1 && h.gen != seen[0] {
		h.mu.Unlock()
		return nil
	}
	h.mu.Unlock()
	e, err := h.session()
	if err != nil {
		return err
	}
	// The session answer is flat, as /v1/maintainer/session is: {"ok": true, "token", ...}.
	var answer struct {
		Token string `json:"token"`
	}
	if err := h.post(ctx, "/v1/maintainer/signed/session", "", e, &flat{&answer}); err != nil {
		return err
	}
	if answer.Token == "" {
		return fmt.Errorf("%w: the session answer has no token", ErrProtocol)
	}
	h.mu.Lock()
	h.token, h.gen = answer.Token, h.gen+1
	h.mu.Unlock()
	return nil
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
		return fmt.Errorf("%w: %s", ErrClosed, msg)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 && resp.StatusCode < 400 {
		return fmt.Errorf("%w: the node answered a redirect (%d), which is not followed", ErrProtocol, resp.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, MaxAnswer+1))
	if err != nil {
		return fmt.Errorf("%w: reading the answer: %s", ErrClosed, redactToken(err.Error(), token))
	}
	if len(b) > MaxAnswer {
		return fmt.Errorf("%w: the answer is larger than %d bytes", ErrProtocol, MaxAnswer)
	}
	if mt, _, err := mime.ParseMediaType(resp.Header.Get("Content-Type")); err != nil || mt != "application/json" {
		return fmt.Errorf("%w: the node answered HTTP %d without JSON", ErrProtocol, resp.StatusCode)
	}
	if f, ok := out.(*flat); ok {
		err = decodeFlat(path, b, resp.StatusCode >= 400, f.v)
	} else {
		err = decodeAnswer(path, string(b), resp.StatusCode >= 400, out)
	}
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

func decodeFlat(what string, b []byte, isError bool, out any) error {
	var ok struct {
		OK bool `json:"ok"`
	}
	if err := json.Unmarshal(b, &ok); err != nil {
		return fmt.Errorf("%w: %s answered text that is not JSON", ErrProtocol, what)
	}
	if isError || !ok.OK {
		return decodeAnswer(what, string(b), true, nil)
	}
	if err := json.Unmarshal(b, out); err != nil {
		return fmt.Errorf("%w: %s: %v", ErrProtocol, what, err)
	}
	return nil
}
