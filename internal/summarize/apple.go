package summarize

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"github.com/rwcii/koinon-voice/internal/sidecar"
)

// ErrUnavailable is an engine that cannot run on this Mac, with the reason Apple gives.
var ErrUnavailable = errors.New("unavailable")

// Launcher runs the Apple helper for the Apple engine and the voice, and starts a new one
// when the last one ended: the bridge kills a helper that misses a deadline.
type Launcher struct {
	Path string

	mu    sync.Mutex
	h     *sidecar.Helper
	start func(ctx context.Context, path string) (*sidecar.Helper, error)
}

// Get returns a running helper.
func (l *Launcher) Get(ctx context.Context) (*sidecar.Helper, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.h != nil && l.h.Err() == nil {
		return l.h, nil
	}
	if l.h != nil {
		l.h.Close()
		l.h = nil
	}
	if l.Path == "" {
		return nil, errors.New("no Apple helper: set helper in the engines file or pass --helper")
	}
	start := l.start
	if start == nil {
		start = func(ctx context.Context, path string) (*sidecar.Helper, error) { return sidecar.Start(ctx, path) }
	}
	h, err := start(ctx, l.Path)
	if err != nil {
		return nil, err
	}
	l.h = h
	return h, nil
}

// Close ends the helper, if one is running.
func (l *Launcher) Close() {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.h != nil {
		l.h.Close()
		l.h = nil
	}
}

// Apple summarizes with Apple's on-device model through the helper (chunk 03).
type Apple struct {
	Helper *Launcher
}

func (a *Apple) Summarize(ctx context.Context, item Item) (Summary, error) {
	return a.Stream(ctx, item, nil)
}

// Stream is Summarize with each text delta handed to onText.
func (a *Apple) Stream(ctx context.Context, item Item, onText func(string)) (Summary, error) {
	h, err := a.Helper.Get(ctx)
	if err != nil {
		return Summary{}, err
	}
	if !h.Hello().Has("summarize") {
		return Summary{}, fmt.Errorf("%w: the helper does not offer summarize", ErrUnavailable)
	}
	t := newTimer(nil, onText)
	data, err := h.Call(ctx, "summarize", map[string]string{"instructions": Instructions, "prompt": Prompt(item)}, func(e sidecar.Event) {
		var d struct{ Text string }
		if json.Unmarshal(e.Data, &d) == nil && e.Name == "delta" {
			t.add(d.Text)
		}
	})
	if err != nil {
		var he *sidecar.Error
		if errors.As(err, &he) {
			switch he.Code {
			case "unavailable":
				err = fmt.Errorf("%w: %s", ErrUnavailable, he.Message)
			case "refused":
				err = fmt.Errorf("%w: %s", ErrRefused, he.Message)
			case "incomplete":
				err = fmt.Errorf("%w: %s", ErrIncomplete, he.Message)
			}
		}
		s, _ := t.finish()
		return s, err
	}
	var r struct {
		Text         string
		InputTokens  *int64 `json:"input_tokens"`
		OutputTokens *int64 `json:"output_tokens"`
	}
	if err := json.Unmarshal(data, &r); err != nil {
		return Summary{}, fmt.Errorf("helper result: %w", err)
	}
	// The result holds the whole answer; deltas served the timings and speech.
	t.text.Reset()
	t.text.WriteString(r.Text)
	if r.InputTokens != nil && r.OutputTokens != nil {
		t.usage(*r.InputTokens, *r.OutputTokens, Price{})
	}
	return t.finish()
}
