package sidecar

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// HelloTimeout bounds the wait for a helper's hello.
var HelloTimeout = 10 * time.Second

// ErrExited is a helper that ended while the bridge still needed it: a crash, or an exit
// mid-request.
var ErrExited = errors.New("helper exited")

// ErrDeadline is a request past its deadline. The bridge killed the helper's process group;
// the next request needs a new helper.
var ErrDeadline = errors.New("helper missed its deadline")

// ErrVersion is a helper that speaks another protocol version.
var ErrVersion = errors.New("helper protocol version")

// Error is an error the helper reported for one request, such as code "unavailable" with
// the engine's reason.
type Error struct {
	Code    string
	Message string
}

func (e *Error) Error() string {
	if e.Message == "" {
		return e.Code
	}
	return e.Code + ": " + e.Message
}

// Event is one event a helper sent for a request.
type Event struct {
	Name string
	Data json.RawMessage
	// At is when the bridge read the event.
	At time.Time
}

// Hello is what the helper said it is.
type Hello struct {
	Helper       string
	Version      string
	Capabilities []string
	Info         json.RawMessage
}

// Has reports whether the helper offers a capability.
func (h Hello) Has(capability string) bool {
	for _, c := range h.Capabilities {
		if c == capability {
			return true
		}
	}
	return false
}

// Helper is a running helper process.
type Helper struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	hello  Hello
	stderr *tail

	writeMu sync.Mutex

	mu      sync.Mutex
	nextID  uint64
	calls   map[uint64]*Stream
	dead    error
	done    chan struct{} // closed when the process has been reaped
	exitErr error
}

// Start runs the helper at path and waits for its hello. A missing or non-executable path,
// a helper that exits or writes junk first, no hello within HelloTimeout or another protocol
// version fails Start and leaves no process behind.
func Start(ctx context.Context, path string, args ...string) (*Helper, error) {
	cmd := exec.Command(path, args...)
	setGroup(cmd)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	h := &Helper{cmd: cmd, stdin: stdin, stderr: &tail{max: 4096}, calls: map[uint64]*Stream{}, done: make(chan struct{})}
	cmd.Stderr = h.stderr
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start helper %s: %w", path, err)
	}
	go func() {
		h.exitErr = cmd.Wait()
		close(h.done)
	}()

	r := bufio.NewReader(stdout)
	hello := make(chan error, 1)
	go func() {
		f, err := readFrame(r)
		switch {
		case err != nil:
			hello <- err
		case f.msg == nil || f.msg.Type != "hello":
			hello <- fmt.Errorf("%w: the first frame is not a hello", ErrProtocol)
		case f.msg.Protocol != Version:
			hello <- fmt.Errorf("%w %d, want %d", ErrVersion, f.msg.Protocol, Version)
		default:
			h.hello = Hello{Helper: f.msg.Helper, Version: f.msg.HelperVer, Capabilities: f.msg.Capabilities, Info: f.msg.Info}
			hello <- nil
		}
	}()
	timer := time.NewTimer(HelloTimeout)
	defer timer.Stop()
	select {
	case err = <-hello:
	case <-timer.C:
		err = fmt.Errorf("no hello from the helper within %s", HelloTimeout)
	case <-ctx.Done():
		err = ctx.Err()
	}
	if err != nil {
		h.kill()
		<-h.done
		if errors.Is(err, io.EOF) {
			err = h.exitError()
		}
		return nil, fmt.Errorf("helper %s: %w", path, err)
	}
	go h.read(r)
	return h, nil
}

// Hello returns the helper's hello.
func (h *Helper) Hello() Hello { return h.hello }

// Err is why the helper can take no more requests, or nil while it can.
func (h *Helper) Err() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.dead
}

// Close ends the helper: it closes stdin, gives the helper a moment to exit and then kills
// its process group.
func (h *Helper) Close() error {
	h.fail(fmt.Errorf("%w: closed", ErrExited))
	h.stdin.Close()
	select {
	case <-h.done:
	case <-time.After(2 * time.Second):
	}
	h.kill()
	<-h.done
	return nil
}

// Stream is one request in flight. Send writes input frames for a request that takes them,
// CloseInput ends its input, and Wait returns its result.
type Stream struct {
	h       *Helper
	id      uint64
	onEvent func(Event)
	once    sync.Once
	done    chan struct{}
	out     outcome

	// eventMu is held while onEvent runs, so every event happens before Wait returns.
	eventMu  sync.Mutex
	finished bool
}

type outcome struct {
	data json.RawMessage
	err  error
}

// Open sends a request. Events reach onEvent, which may be nil, in order on the bridge's
// reader. Past ctx's deadline the bridge kills the helper (ErrDeadline).
func (h *Helper) Open(ctx context.Context, method string, params any, onEvent func(Event)) (*Stream, error) {
	raw, err := json.Marshal(params)
	if err != nil {
		return nil, err
	}
	h.mu.Lock()
	if h.dead != nil {
		err := h.dead
		h.mu.Unlock()
		return nil, err
	}
	h.nextID++
	s := &Stream{h: h, id: h.nextID, onEvent: onEvent, done: make(chan struct{})}
	h.calls[s.id] = s
	h.mu.Unlock()

	m := Message{Type: "request", ID: s.id, Method: method, Params: raw}
	if d, ok := ctx.Deadline(); ok {
		m.DeadlineMS = max(time.Until(d).Milliseconds(), 1)
	}
	if err := h.write(m); err != nil {
		h.remove(s.id)
		return nil, err
	}
	go func() {
		select {
		case <-ctx.Done():
			err := ctx.Err()
			if errors.Is(err, context.DeadlineExceeded) {
				err = fmt.Errorf("%w: %w", ErrDeadline, err)
			}
			// A request past its deadline may have the helper wedged; the bridge does not
			// wait to find out (ADR 002, Decision 1).
			h.fail(err)
			h.kill()
		case <-s.done:
		}
	}()
	return s, nil
}

// Call sends a request without input and waits for its result.
func (h *Helper) Call(ctx context.Context, method string, params any, onEvent func(Event)) (json.RawMessage, error) {
	s, err := h.Open(ctx, method, params, onEvent)
	if err != nil {
		return nil, err
	}
	return s.Wait()
}

// Send writes one input frame for the request.
func (s *Stream) Send(params any) error {
	raw, err := json.Marshal(params)
	if err != nil {
		return err
	}
	return s.h.write(Message{Type: "input", ID: s.id, Params: raw})
}

// CloseInput tells the helper no more input is coming.
func (s *Stream) CloseInput() error {
	return s.h.write(Message{Type: "input", ID: s.id, End: true})
}

// Wait returns the request's result, the helper's error for it, or why the helper is gone.
func (s *Stream) Wait() (json.RawMessage, error) {
	<-s.done
	return s.out.data, s.out.err
}

func (s *Stream) finish(o outcome) {
	s.once.Do(func() {
		s.eventMu.Lock()
		s.finished = true
		s.eventMu.Unlock()
		s.out = o
		close(s.done)
	})
}

func (s *Stream) event(e Event) {
	s.eventMu.Lock()
	defer s.eventMu.Unlock()
	if !s.finished && s.onEvent != nil {
		s.onEvent(e)
	}
}

func (h *Helper) write(m Message) error {
	h.mu.Lock()
	dead := h.dead
	h.mu.Unlock()
	if dead != nil {
		return dead
	}
	h.writeMu.Lock()
	defer h.writeMu.Unlock()
	if err := writeMessage(h.stdin, m); err != nil {
		return fmt.Errorf("%w: write: %v", ErrExited, err)
	}
	return nil
}

func (h *Helper) remove(id uint64) {
	h.mu.Lock()
	delete(h.calls, id)
	h.mu.Unlock()
}

// read dispatches frames until the helper's stdout ends or breaks the protocol.
func (h *Helper) read(r io.Reader) {
	for {
		f, err := readFrame(r)
		if err != nil {
			if errors.Is(err, io.EOF) {
				// Wait for the exit status so the error can name it.
				select {
				case <-h.done:
				case <-time.After(time.Second):
				}
				h.fail(h.exitError())
			} else {
				h.fail(err)
			}
			h.kill()
			return
		}
		if f.msg == nil {
			continue // no Phase 0 request receives audio frames
		}
		m := f.msg
		h.mu.Lock()
		s := h.calls[m.ID]
		h.mu.Unlock()
		if s == nil {
			continue // a late frame for a request already finished
		}
		switch m.Type {
		case "event":
			s.event(Event{Name: m.Event, Data: m.Data, At: time.Now()})
		case "result":
			h.remove(m.ID)
			s.finish(outcome{data: m.Data})
		case "error":
			h.remove(m.ID)
			s.finish(outcome{err: &Error{Code: m.Code, Message: m.Message}})
		default:
			h.fail(fmt.Errorf("%w: unexpected message type %q", ErrProtocol, m.Type))
			h.kill()
			return
		}
	}
}

// fail marks the helper dead and fails every request in flight with err.
func (h *Helper) fail(err error) {
	h.mu.Lock()
	if h.dead == nil {
		h.dead = err
	}
	calls := h.calls
	h.calls = map[uint64]*Stream{}
	dead := h.dead
	h.mu.Unlock()
	for _, s := range calls {
		s.finish(outcome{err: dead})
	}
}

func (h *Helper) exitError() error {
	select {
	case <-h.done:
	default:
		return fmt.Errorf("%w: stdout closed", ErrExited)
	}
	msg := "exit status 0"
	if h.exitErr != nil {
		msg = h.exitErr.Error()
	}
	if s := strings.TrimSpace(h.stderr.String()); s != "" {
		msg += "; stderr: " + s
	}
	return fmt.Errorf("%w: %s", ErrExited, msg)
}

func (h *Helper) kill() {
	killGroup(h.cmd)
}

// tail keeps the last max bytes written, for error messages.
type tail struct {
	mu  sync.Mutex
	max int
	b   []byte
}

func (t *tail) Write(p []byte) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.b = append(t.b, p...)
	if len(t.b) > t.max {
		t.b = t.b[len(t.b)-t.max:]
	}
	return len(p), nil
}

func (t *tail) String() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return string(t.b)
}
