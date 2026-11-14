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
	// Clock gives every envelope's created from the key's durable high-water (Koinon contract
	// 072870b, Decision 1.7); it is required.
	Clock signed.Clock
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
	session, err := s.envelope("session", opts.NodePin, map[string]any{"client": opts.Client}, true)
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

// envelope signs a new operation: a new request ID when request is true, and created from
// the clock.
func (s *Signed) envelope(op, target string, args map[string]any, request bool) (signed.Envelope, error) {
	created, err := s.opts.Clock.Next(s.opts.Key.ID(), false)
	if err != nil {
		return signed.Envelope{}, err
	}
	e := signed.Envelope{V: signed.Version, Target: target, Op: op, Args: args, Created: created}
	if request {
		id, err := signed.NewRequestID()
		if err != nil {
			return signed.Envelope{}, err
		}
		e.Request = id
	}
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
	e, err := s.envelope(op, target, args, true)
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

// Envelope returns the write's signed envelope.
func (w Write) Envelope() signed.Envelope { return w.envelope }

// NewWrite signs a new write.
func (s *Signed) NewWrite(target, op string, args map[string]any) (Write, error) {
	e, err := s.envelope(op, target, args, true)
	return Write{envelope: e}, err
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
	err := s.Send(ctx, w.envelope, out)
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
	State  string          `json:"state"`
	Result json.RawMessage `json:"result"`
	Code   string          `json:"code"`
	Now    int64           `json:"now"`
}

// AskOutcome signs a new outcome read for the write's request and created (D4.3). It reports
// the target's answer; deciding whether an absent receipt is final waits for the contract's
// G18b.
func (s *Signed) AskOutcome(ctx context.Context, w Write) (Outcome, error) {
	var o Outcome
	err := s.Read(ctx, w.envelope.Target, "outcome", map[string]any{
		"request": w.envelope.Request, "created": w.envelope.Created,
	}, &o)
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
