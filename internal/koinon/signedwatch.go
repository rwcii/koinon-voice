package koinon

import (
	"context"
	"errors"
	"sync"
	"time"
)

// Change is what a target's changes wait found.
type Change struct {
	Pin string
	// Topics changed, with their new versions (20-digit strings, Koinon contract cbd8e21,
	// Answer shapes); the consumer reads them, then calls Done.
	Topics map[string]string
	// Removed topics are gone on the target, such as a deleted room; the consumer drops
	// them, then calls Done with them.
	Removed []string
	// Offline: the target answered computer_offline; its wait pauses until Resume, which
	// the consumer calls when the node's computers topic shows it back (D6.4).
	Offline bool
	// SessionEnded: the session ended (maintainer_token_expired or the stream closed); every
	// wait stops, and the consumer signs a new session and watches again from Versions (D6.5).
	SessionEnded bool
	// Err is any other refusal or failure; the wait tries again after a delay.
	Err error
}

// Watcher keeps one signed changes wait at a time for each target that Voice shows
// (Koinon contract D6.1, D6.2). Versions advance only when the consumer calls Done after it
// read the changed topics, so a change is never lost between a read and the next wait.
type Watcher struct {
	s      *Signed
	out    chan<- Change
	wait   int
	delay  func(attempt int) time.Duration
	ctx    context.Context
	cancel context.CancelFunc

	mu       sync.Mutex
	versions map[string]map[string]string
	targets  map[string]*watchTarget
	wg       sync.WaitGroup
}

type watchTarget struct {
	cancel context.CancelFunc
	done   chan struct{}
	resume chan struct{}
}

// WatchOptions set the wait and the delay after a failure; zero values take the defaults
// (30 seconds; a delay that grows to 30 seconds).
type WatchOptions struct {
	WaitSeconds int
	Delay       func(attempt int) time.Duration
}

// Watch starts watching from the stored versions (pin to topic to version). Changes go to
// out. Stop ends every wait.
func (s *Signed) Watch(ctx context.Context, versions map[string]map[string]string, out chan<- Change, opts WatchOptions) *Watcher {
	if opts.WaitSeconds <= 0 || opts.WaitSeconds > 30 {
		opts.WaitSeconds = 30
	}
	if opts.Delay == nil {
		opts.Delay = func(attempt int) time.Duration {
			d := time.Second << min(attempt, 5)
			return min(d, 30*time.Second)
		}
	}
	w := &Watcher{s: s, out: out, wait: opts.WaitSeconds, delay: opts.Delay,
		versions: map[string]map[string]string{}, targets: map[string]*watchTarget{}}
	w.ctx, w.cancel = context.WithCancel(ctx)
	for pin, v := range versions {
		w.versions[pin] = copyVersions(v)
	}
	return w
}

// Add starts the wait of one target (the node's pin includes the topic computers).
func (w *Watcher) Add(pin string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if _, ok := w.targets[pin]; ok || w.ctx.Err() != nil {
		return
	}
	ctx, cancel := context.WithCancel(w.ctx)
	t := &watchTarget{cancel: cancel, done: make(chan struct{}, 1), resume: make(chan struct{}, 1)}
	w.targets[pin] = t
	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		w.run(ctx, pin, t)
	}()
}

// Remove stops one target's wait; its versions stay for a later Add.
func (w *Watcher) Remove(pin string) {
	w.mu.Lock()
	t, ok := w.targets[pin]
	delete(w.targets, pin)
	w.mu.Unlock()
	if ok {
		t.cancel()
	}
}

// Done stores the versions that the consumer has read for a target and drops the removed
// topics, and lets its next wait start.
func (w *Watcher) Done(pin string, read map[string]string, removed []string) {
	w.store(pin, read, removed)
	w.mu.Lock()
	t := w.targets[pin]
	w.mu.Unlock()
	if t != nil {
		select {
		case t.done <- struct{}{}:
		default:
		}
	}
}

// Resume restarts a target's wait after it was offline.
func (w *Watcher) Resume(pin string) {
	w.mu.Lock()
	t := w.targets[pin]
	w.mu.Unlock()
	if t != nil {
		select {
		case t.resume <- struct{}{}:
		default:
		}
	}
}

// Versions returns a copy of the stored versions, to keep across sessions and restarts.
func (w *Watcher) Versions() map[string]map[string]string {
	w.mu.Lock()
	defer w.mu.Unlock()
	c := map[string]map[string]string{}
	for pin, v := range w.versions {
		c[pin] = copyVersions(v)
	}
	return c
}

// Stop ends every wait and returns when they have ended.
func (w *Watcher) Stop() {
	w.cancel()
	w.wg.Wait()
}

func (w *Watcher) store(pin string, read map[string]string, removed []string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	v := w.versions[pin]
	if v == nil {
		v = map[string]string{}
		w.versions[pin] = v
	}
	for topic, n := range read {
		if newer(n, v[topic]) {
			v[topic] = n
		}
	}
	for _, topic := range removed {
		delete(v, topic)
	}
}

func (w *Watcher) run(ctx context.Context, pin string, t *watchTarget) {
	attempt := 0
	for ctx.Err() == nil {
		w.mu.Lock()
		have := copyVersions(w.versions[pin])
		w.mu.Unlock()
		args := map[string]any{"versions": toArgs(have), "wait_seconds": w.wait}
		var answer struct {
			Versions map[string]string `json:"versions"`
			Changed  []string          `json:"changed"`
		}
		err := w.s.Read(ctx, pin, "changes", args, &answer)
		if ctx.Err() != nil {
			return
		}
		var r *Refusal
		switch {
		case err == nil:
			attempt = 0
			// The target's changed set names every topic that differs, including one that
			// is gone (present only in the versions sent); a newer version is a change too.
			changed, removed := map[string]string{}, []string{}
			for _, topic := range answer.Changed {
				if n, ok := answer.Versions[topic]; ok {
					changed[topic] = n
				} else if _, had := have[topic]; had {
					removed = append(removed, topic)
				}
			}
			for topic, n := range answer.Versions {
				if newer(n, have[topic]) {
					changed[topic] = n
				}
			}
			if len(changed) == 0 && len(removed) == 0 {
				continue
			}
			if !w.emit(ctx, Change{Pin: pin, Topics: changed, Removed: removed}) {
				return
			}
			select {
			case <-t.done:
			case <-ctx.Done():
				return
			}
		case errors.As(err, &r) && r.Code == "computer_offline":
			if !w.emit(ctx, Change{Pin: pin, Offline: true}) {
				return
			}
			select {
			case <-t.resume:
			case <-ctx.Done():
				return
			}
		case errors.As(err, &r) && r.Code == "maintainer_token_expired",
			errors.Is(err, ErrClosed), errors.Is(err, ErrStalled):
			w.emit(ctx, Change{Pin: pin, SessionEnded: true})
			w.cancel()
			return
		default:
			if !w.emit(ctx, Change{Pin: pin, Err: err}) {
				return
			}
			select {
			case <-time.After(w.delay(attempt)):
				attempt++
			case <-ctx.Done():
				return
			}
		}
	}
}

func (w *Watcher) emit(ctx context.Context, c Change) bool {
	select {
	case w.out <- c:
		return true
	case <-ctx.Done():
		return false
	}
}

func copyVersions(v map[string]string) map[string]string {
	c := make(map[string]string, len(v))
	for k, n := range v {
		c[k] = n
	}
	return c
}

// newer compares two versions, decimal strings of digits: a longer one is larger, and equal
// lengths compare as text.
func newer(a, b string) bool {
	if len(a) != len(b) {
		return len(a) > len(b)
	}
	return a > b
}

// toArgs turns versions into a v1 object of strings.
func toArgs(v map[string]string) map[string]any {
	m := make(map[string]any, len(v))
	for k, n := range v {
		m[k] = n
	}
	return m
}
