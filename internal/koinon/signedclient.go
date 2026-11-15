package koinon

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"sync"
	"time"

	"github.com/rwcii/koinon-voice/internal/signed"
)

// SignedOptions configure a signed client of one Koinon node (ADR 004; Koinon contract
// 15ec0ff, Decisions 5.1 and 5.3). It is not wired into board or dashboard yet; the per-host
// connectors stay until the transition of ADR 004, Decision 1 ends.
type SignedOptions struct {
	// Koinon is the koinon binary; Args are extra words after `mcp --maintainer --signed`,
	// such as --state-dir.
	Koinon string
	Args   []string
	// NodePin is the trusted pin of the node, from enrollment on the node itself; it is never
	// taken from a computers answer.
	NodePin string
	// Client is the client label of the session.
	Client string
	// Key signs every envelope; only signed envelopes reach the process.
	Key signed.Key
	// Auths holds the authorization ID of each enrolled computer, by pin (Koinon contract
	// 05c531c, Decision 1.7); Voice acts only on these pins (D5.5).
	Auths map[string]string
	// Now is the clock for created; nil is time.Now.
	Now func() time.Time
	// Version is Koinon Voice's version for MCP initialize.
	Version string
	// Grace is how long a stopping process may take before it is killed.
	Grace time.Duration
}

// Signed is one signed MCP session with a node.
type Signed struct {
	opts   SignedOptions
	client *Client
	stop   func()
}

// StartSigned runs `koinon mcp --maintainer --signed` as a child process, writes a signed
// session envelope as the first line of its standard input, and initializes MCP.
func StartSigned(ctx context.Context, opts SignedOptions) (*Signed, error) {
	args := append([]string{"mcp", "--maintainer", "--signed"}, opts.Args...)
	cmd := exec.Command(opts.Koinon, args...)
	setGroup(cmd)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stderr := &tail{max: 4096}
	cmd.Stderr = stderr
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start %s: %w", opts.Koinon, err)
	}
	exited := make(chan struct{})
	go func() {
		_ = cmd.Wait()
		close(exited)
	}()
	grace := opts.Grace
	if grace == 0 {
		grace = 2 * time.Second
	}
	var once sync.Once
	stop := func() {
		once.Do(func() {
			stdin.Close()
			select {
			case <-exited:
			case <-time.After(grace):
			}
			killGroup(cmd)
			<-exited
		})
	}
	s, err := OpenSigned(ctx, opts, stdout, stdin)
	if err != nil {
		stop()
		if r := stderrRefusal(stderr.lastLine()); r != nil {
			return nil, r
		}
		return nil, err
	}
	s.stop = stop
	return s, nil
}

// OpenSigned opens a signed session on the given streams: the session envelope first, then
// MCP initialize.
func OpenSigned(ctx context.Context, opts SignedOptions, r io.Reader, w io.Writer) (*Signed, error) {
	s := &Signed{opts: opts}
	if err := signed.CheckPin(opts.NodePin); err != nil {
		return nil, err
	}
	session, err := s.envelope("session", opts.NodePin, map[string]any{"client": opts.Client}, "")
	if err != nil {
		return nil, err
	}
	line, err := json.Marshal(session)
	if err != nil {
		return nil, err
	}
	if _, err := w.Write(append(line, '\n')); err != nil {
		return nil, fmt.Errorf("%w: session: %v", ErrClosed, err)
	}
	s.client = NewClient(r, w)
	if err := s.client.Initialize(ctx, opts.Version); err != nil {
		return nil, err
	}
	return s, nil
}

// Close ends the session and its process.
func (s *Signed) Close() {
	if s.stop != nil {
		s.stop()
	}
}

// Done is closed when the session's stream ends.
func (s *Signed) Done() <-chan struct{} { return s.client.Done() }

// envelope signs a new operation with a new request ID, created from the clock, and the
// target's authorization; auth overrides it for an outcome of an earlier write.
func (s *Signed) envelope(op, target string, args map[string]any, auth string) (signed.Envelope, error) {
	if auth == "" {
		auth = s.opts.Auths[target]
	}
	if auth == "" {
		return signed.Envelope{}, fmt.Errorf("this install has no enrollment for the computer %s; run koinon-voice key enroll --target %s", target, target)
	}
	now := time.Now
	if s.opts.Now != nil {
		now = s.opts.Now
	}
	id, err := signed.NewRequestID()
	if err != nil {
		return signed.Envelope{}, err
	}
	e := signed.Envelope{V: signed.Version, Target: target, Op: op, Args: args, Request: id, Auth: auth, Created: now().Unix()}
	return signed.Sign(s.opts.Key.Private, e)
}

// Send sends one signed envelope through the tool `signed` and decodes the target's result
// into out. A refusal is a *Refusal with its code and details.
func (s *Signed) Send(ctx context.Context, e signed.Envelope, out any) error {
	return s.client.Tool(ctx, "signed", map[string]any{"envelope": e}, out)
}

// Read signs and sends a new read.
func (s *Signed) Read(ctx context.Context, target, op string, args map[string]any, out any) error {
	if args == nil {
		args = map[string]any{}
	}
	e, err := s.envelope(op, target, args, "")
	if err != nil {
		return err
	}
	return s.Send(ctx, e, out)
}

// BoardList reads one page of a target's board.
func (s *Signed) BoardList(ctx context.Context, target, filter string, after int64, limit int) (boardPage, error) {
	args := map[string]any{"filter": filter, "limit": limit}
	if after > 0 {
		args["after"] = after
	}
	var p boardPage
	err := s.Read(ctx, target, "board_list", args, &p)
	return p, err
}

// DecisionGet reads one item of a target.
func (s *Signed) DecisionGet(ctx context.Context, target, reference string) (Item, error) {
	var it Item
	err := s.Read(ctx, target, "decision_get", map[string]any{"reference": reference}, &it)
	return it, err
}

// Computer is the node or one of its paired computers, as computers gives it.
type Computer struct {
	Pin    string `json:"pin"`
	Name   string `json:"name"`
	State  string `json:"state"`
	Reason string `json:"reason"`
	Since  int64  `json:"since"`
}

// Computers reads the node's computers.
func (s *Signed) Computers(ctx context.Context) (json.RawMessage, error) {
	var raw json.RawMessage
	err := s.Read(ctx, s.opts.NodePin, "computers", nil, &raw)
	return raw, err
}

// Write is one write, signed once. Every retry sends this same envelope; a new request ID
// is only for a new operation.
type Write struct {
	envelope signed.Envelope
}

// Envelope returns a copy of the write's signed envelope; changing it does not change the
// write.
func (w Write) Envelope() signed.Envelope {
	e := w.envelope
	e.Args = cloneArgs(e.Args)
	return e
}

// NewWrite signs a new write over its own copy of args, so the caller's map can change
// afterwards without changing the signed write.
func (s *Signed) NewWrite(target, op string, args map[string]any) (Write, error) {
	e, err := s.envelope(op, target, cloneArgs(args), "")
	return Write{envelope: e}, err
}

// cloneArgs copies v1 arguments, with their nested objects.
func cloneArgs(m map[string]any) map[string]any {
	if m == nil {
		return nil
	}
	c := make(map[string]any, len(m))
	for k, v := range m {
		switch x := v.(type) {
		case map[string]any:
			c[k] = cloneArgs(x)
		case []string:
			c[k] = append([]string(nil), x...)
		default:
			c[k] = v
		}
	}
	return c
}

// NewVerdict signs a verdict on the item as the maintainer was shown it: the snapshot
// digest is computed from shown, never copied from an answer.
func (s *Signed) NewVerdict(target string, shown Item, verdict, note string) (Write, error) {
	digest, err := signed.Snapshot{
		Reference: shown.Reference, Revision: shown.Revision, Kind: shown.Kind, Title: shown.Title,
		Where: shown.Where, Context: shown.Context, Recommendation: shown.Recommendation,
		AgentText: shown.AgentText, Commands: shown.Commands,
	}.Digest()
	if err != nil {
		return Write{}, err
	}
	return s.NewWrite(target, "decision_verdict", map[string]any{
		"reference": shown.Reference, "revision": shown.Revision, "snapshot": digest,
		"verdict": verdict, "note": note,
	})
}

// WriteState is what Voice knows about a write after one sending.
type WriteState string

const (
	// Recorded: the target recorded the write and answered.
	Recorded WriteState = "recorded"
	// Refused: the target refused it with a code; it did not run.
	Refused WriteState = "refused"
	// NotRun: the node had no connection to the target and sent nothing (computer_offline).
	NotRun WriteState = "not_run"
	// Unknown: the envelope may have reached the target and the answer was lost
	// (outcome_unknown, or the session ended); Voice asks outcome.
	Unknown WriteState = "unknown"
)

// WriteResult is a write's state with its refusal or error.
type WriteResult struct {
	State   WriteState
	Refusal *Refusal
	Err     error
}

// Submit sends the write's envelope and classifies the answer (Koinon contract D4.5).
func (s *Signed) Submit(ctx context.Context, w Write, out any) WriteResult {
	err := s.Send(ctx, w.Envelope(), out)
	if err == nil {
		return WriteResult{State: Recorded}
	}
	var r *Refusal
	if errors.As(err, &r) {
		switch r.Code {
		case "computer_offline":
			return WriteResult{State: NotRun, Refusal: r}
		case "outcome_unknown":
			return WriteResult{State: Unknown, Refusal: r}
		}
		return WriteResult{State: Refused, Refusal: r}
	}
	return WriteResult{State: Unknown, Err: err}
}

// Outcome is the target's answer to outcome for one write.
type Outcome struct {
	State   string          `json:"state"`
	Result  json.RawMessage `json:"result"`
	Code    string          `json:"code"`
	Error   string          `json:"error"`
	Details json.RawMessage `json:"details"`
	Now     int64           `json:"now"`
}

// AskOutcome signs a new outcome read for the write's request and created, under the write's
// own authorization (D4.3): a target enrolled again since then refuses it (signature_refused,
// reason authorization_changed).
func (s *Signed) AskOutcome(ctx context.Context, w Write) (Outcome, error) {
	var o Outcome
	e, err := s.envelope("outcome", w.envelope.Target, map[string]any{
		"request": w.envelope.Request, "created": w.envelope.Created,
	}, w.envelope.Auth)
	if err != nil {
		return o, err
	}
	err = s.Send(ctx, e, &o)
	return o, err
}

// stderrRefusal reads the one JSON line that a refused koinon process writes on standard
// error, such as signature_refused.
func stderrRefusal(text string) *Refusal {
	for _, line := range bytes.Split([]byte(text), []byte("\n")) {
		var a struct {
			Code    string          `json:"code"`
			Error   string          `json:"error"`
			Detail  string          `json:"detail"`
			Details json.RawMessage `json:"details"`
		}
		if json.Unmarshal(line, &a) == nil && a.Code != "" {
			msg := a.Error
			if msg == "" {
				msg = a.Detail
			}
			return &Refusal{Code: a.Code, Message: msg, Details: a.Details}
		}
	}
	return nil
}

// ReceiptDays is how long a target keeps a write's receipt (Koinon contract D4.1).
const ReceiptDays = 7

// WindowSeconds is how long an envelope is valid against a target's clock (D3.3).
const WindowSeconds = 5 * 60

// Reconciled is what reconciliation found for a write whose answer was lost.
type Reconciled struct {
	State WriteState
	// Final is true when the state cannot change: recorded, refused, or not run because
	// the target's clock is past the write's window with no receipt.
	Final   bool
	Refusal *Refusal
	Err     error
}

// Reconcile finds the outcome of a write whose answer was lost (Koinon contract 4e324d5,
// D4.3, D1.7):
//
//   - recorded or refused: final;
//   - refused with signature_refused, reason authorization_changed (the key was enrolled
//     again since the write): unknown, and not sent again; the maintainer reads the target's
//     record;
//   - unknown_request with the target's now past created+5 minutes: final, did not run;
//   - unknown_request within the window: the same envelope is sent once more; if that answer
//     is lost too, the caller reconciles again after the window;
//   - past the receipts' 7 days: unknown.
//
// It never signs the write again under a new request ID.
func (s *Signed) Reconcile(ctx context.Context, w Write, out any) Reconciled {
	created := w.envelope.Created
	o, err := s.AskOutcome(ctx, w)
	if err != nil {
		var r *Refusal
		if errors.As(err, &r) && authorizationChanged(r) {
			return Reconciled{State: Unknown, Final: true, Refusal: r}
		}
		return Reconciled{State: Unknown, Err: err}
	}
	switch o.State {
	case "recorded":
		if out != nil && len(o.Result) > 0 {
			if err := json.Unmarshal(o.Result, out); err != nil {
				return Reconciled{State: Recorded, Final: true, Err: err}
			}
		}
		return Reconciled{State: Recorded, Final: true}
	case "refused":
		return Reconciled{State: Refused, Final: true, Refusal: &Refusal{Code: o.Code, Message: o.Error, Details: o.Details}}
	case "unknown_request":
		if o.Now > created+ReceiptDays*24*60*60 {
			return Reconciled{State: Unknown, Final: true}
		}
		if o.Now > created+WindowSeconds {
			return Reconciled{State: NotRun, Final: true}
		}
		r := s.Submit(ctx, w, out)
		if r.Refusal != nil && authorizationChanged(r.Refusal) {
			// The key was enrolled again after the outcome read: the first sending may have
			// committed under the earlier authorization, which the new one cannot read.
			return Reconciled{State: Unknown, Final: true, Refusal: r.Refusal}
		}
		return Reconciled{State: r.State, Final: r.State == Recorded || r.State == Refused, Refusal: r.Refusal, Err: r.Err}
	}
	return Reconciled{State: Unknown, Err: fmt.Errorf("%w: outcome state %q", ErrProtocol, o.State)}
}

// authorizationChanged is a refusal because the write's authorization is no longer current.
func authorizationChanged(r *Refusal) bool {
	return r.Code == "signature_refused" && r.Reason() == "authorization_changed"
}
