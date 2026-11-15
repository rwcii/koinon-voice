package koinon

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/json"
	"io"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rwcii/koinon-voice/internal/koinon/koinontest"
	"github.com/rwcii/koinon-voice/internal/signed"
)

type sourceRig struct {
	node   *koinontest.SignedNode
	src    *SignedSource
	cancel context.CancelFunc
	done   chan struct{}

	mu    sync.Mutex
	opens int
	ends  []io.Closer // the node's side of each session, to end it
}

func newSourceRig(t *testing.T, auths map[string]string, accessErr error) *sourceRig {
	t.Helper()
	key := signed.Key{User: "robert", Private: ed25519.NewKeyFromSeed(bytes.Repeat([]byte{5}, 32))}
	node := koinontest.NewSignedNode(nodePin, key.Public(), time.Now)
	node.Wait = 200 * time.Millisecond
	node.AddTarget(otherPin)
	node.Change(nodePin, func(tg *koinontest.Target) {
		tg.Auth, tg.Name = strings.Repeat("1", 64), "desk"
		tg.Versions["computers"], tg.Versions["decisions"], tg.Versions["inbox"] = 1, 1, 1
	})
	node.Change(otherPin, func(tg *koinontest.Target) {
		tg.Auth, tg.Name = strings.Repeat("2", 64), "lab-1"
		tg.Items["P-1"] = item("Approve it")
		tg.Inbox = []map[string]any{{"seq": int64(1), "body": "hello"}, {"seq": int64(2), "body": "again"}}
		tg.AckedThrough = 1
		tg.Rooms = []map[string]any{{"room_id": "R1", "name": "ops"}}
		tg.Posts["R1"] = []map[string]any{{"seq": int64(1), "body": "first"}}
		tg.Versions["decisions"], tg.Versions["inbox"], tg.Versions["R1"] = 1, 1, 1
	})
	r := &sourceRig{node: node, done: make(chan struct{})}
	open := func(ctx context.Context, opts SignedOptions) (*Signed, error) {
		cr, cw := io.Pipe()
		nr, nw := io.Pipe()
		go func() {
			node.Serve(cr, nw)
			nw.Close()
		}()
		r.mu.Lock()
		r.opens++
		r.ends = append(r.ends, nw)
		r.mu.Unlock()
		return OpenSigned(ctx, opts, nr, cw)
	}
	r.src = NewSignedSource(SourceOptions{
		Host:      Host{Label: "studio", NodePin: nodePin},
		Access:    Access{Key: key, Auths: auths},
		AccessErr: accessErr,
		Open:      open,
		MinDelay:  10 * time.Millisecond,
		MaxDelay:  50 * time.Millisecond,
		Watch:     WatchOptions{Delay: func(int) time.Duration { return 10 * time.Millisecond }},
	})
	ctx, cancel := context.WithCancel(context.Background())
	r.cancel = cancel
	go func() { r.src.Run(ctx); close(r.done) }()
	t.Cleanup(func() {
		cancel()
		select {
		case <-r.done:
		case <-time.After(5 * time.Second):
			t.Error("Run did not end after cancel")
		}
	})
	return r
}

func bothAuths() map[string]string {
	return map[string]string{nodePin: strings.Repeat("1", 64), otherPin: strings.Repeat("2", 64)}
}

// waitStates polls the states until f holds.
func waitStates(t *testing.T, src *SignedSource, what string, f func(map[string]State) bool) map[string]State {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		m := map[string]State{}
		for _, s := range src.States() {
			m[s.Pin] = s
		}
		if f(m) {
			return m
		}
		if time.Now().After(deadline) {
			t.Fatalf("%s: states %+v", what, m)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func settled(m map[string]State) bool {
	o := m[otherPin]
	return m[nodePin].Status == Connected && o.Status == Connected && len(o.Items) == 1 &&
		len(o.Inbox) == 1 && len(o.Rooms) == 1 && len(o.Rooms[0].Posts) == 1
}

func reads(n *koinontest.SignedNode, pin string) map[string]int {
	c := map[string]int{}
	n.Change(pin, func(tg *koinontest.Target) {
		for k, v := range tg.Reads {
			c[k] = v
		}
	})
	return c
}

func TestSourceReadsTwoTargets(t *testing.T) {
	r := newSourceRig(t, bothAuths(), nil)
	m := waitStates(t, r.src, "settled", settled)
	o := m[otherPin]
	if m[nodePin].Host != "desk" || o.Host != "lab-1" || o.Node != "studio" || m[nodePin].Node != "studio" {
		t.Errorf("names %q %q, node %q", m[nodePin].Host, o.Host, o.Node)
	}
	if o.Items[0].Context != "Approve it" || o.Inbox[0].Body != "again" || o.Rooms[0].Name != "ops" || o.Rooms[0].Posts[0].Body != "first" {
		t.Errorf("other %+v", o)
	}
	if states := r.src.States(); states[0].Pin != nodePin {
		t.Errorf("the node is not first: %+v", states)
	}
}

func TestSourceReadsOnlyChangedTopics(t *testing.T) {
	r := newSourceRig(t, bothAuths(), nil)
	before := waitStates(t, r.src, "settled", settled)
	kept := before[otherPin]
	nodeBefore, otherBefore := reads(r.node, nodePin), reads(r.node, otherPin)
	r.node.Change(otherPin, func(tg *koinontest.Target) {
		it := item("Changed")
		it["revision"] = int64(3)
		tg.Items["P-1"] = it
		tg.Versions["decisions"]++
	})
	waitStates(t, r.src, "changed", func(m map[string]State) bool {
		return len(m[otherPin].Items) == 1 && m[otherPin].Items[0].Context == "Changed"
	})
	nodeAfter, otherAfter := reads(r.node, nodePin), reads(r.node, otherPin)
	for _, op := range []string{"inbox", "room_list", "room_read"} {
		if otherAfter[op] != otherBefore[op] {
			t.Errorf("%s read again: %d -> %d", op, otherBefore[op], otherAfter[op])
		}
	}
	for _, op := range []string{"computers", "board_list", "inbox"} {
		if nodeAfter[op] != nodeBefore[op] {
			t.Errorf("node %s read again", op)
		}
	}
	if otherAfter["board_list"] != otherBefore["board_list"]+1 || otherAfter["decision_get"] != otherBefore["decision_get"]+1 {
		t.Errorf("board reads %v -> %v", otherBefore, otherAfter)
	}
	// The earlier State is a snapshot: the change did not alter it.
	if kept.Items[0].Context != "Approve it" {
		t.Error("a published State changed")
	}
}

func TestSourceTriesAFailedReadAgain(t *testing.T) {
	r := newSourceRig(t, bothAuths(), nil)
	waitStates(t, r.src, "settled", settled)
	r.node.Change(otherPin, func(tg *koinontest.Target) {
		tg.Fail["board_list"] = 2
		tg.Items["P-2"] = map[string]any{"reference": "P-2", "revision": int64(1), "kind": "fyi", "title": "Note",
			"state": "active", "commands": []string{}}
		tg.Versions["decisions"]++
	})
	waitStates(t, r.src, "second item", func(m map[string]State) bool {
		return len(m[otherPin].Items) == 2 && m[otherPin].Status == Connected && m[otherPin].Code == ""
	})
	if n := reads(r.node, otherPin)["board_list"]; n < 4 {
		t.Errorf("board_list read %d times", n)
	}
}

func TestSourceLeavesUnenrolledComputers(t *testing.T) {
	r := newSourceRig(t, map[string]string{nodePin: strings.Repeat("1", 64)}, nil)
	m := waitStates(t, r.src, "not enrolled", func(m map[string]State) bool {
		return m[nodePin].Status == Connected && m[otherPin].Code == "not_enrolled"
	})
	if !strings.Contains(m[otherPin].Reason, "koinon-voice key enroll --target "+otherPin) || m[otherPin].Host != "lab-1" {
		t.Errorf("other %+v", m[otherPin])
	}
	time.Sleep(100 * time.Millisecond)
	if n := reads(r.node, otherPin); len(n) != 0 {
		t.Errorf("reads of an unenrolled computer: %v", n)
	}
}

func TestSourceOpensNothingWithoutEnrollment(t *testing.T) {
	for name, tc := range map[string]struct {
		auths map[string]string
		err   error
		code  string
	}{
		"no node auth": {map[string]string{otherPin: strings.Repeat("2", 64)}, nil, "not_enrolled"},
		"no key":       {nil, signed.ErrNoKey, "key_unavailable"},
	} {
		t.Run(name, func(t *testing.T) {
			r := newSourceRig(t, tc.auths, tc.err)
			waitStates(t, r.src, "refused", func(m map[string]State) bool {
				return m[nodePin].Status == Disconnected && m[nodePin].Code == tc.code
			})
			time.Sleep(50 * time.Millisecond)
			r.mu.Lock()
			defer r.mu.Unlock()
			if r.opens != 0 {
				t.Errorf("%d sessions opened", r.opens)
			}
		})
	}
}

func TestSourceKeysComputersByPin(t *testing.T) {
	r := newSourceRig(t, map[string]string{nodePin: strings.Repeat("1", 64), otherPin: strings.Repeat("2", 64),
		thirdPin: strings.Repeat("3", 64)}, nil)
	waitStates(t, r.src, "settled", settled)

	// A rename keeps the computer and its data.
	r.node.Change(otherPin, func(tg *koinontest.Target) { tg.Name = "lab-renamed" })
	r.node.Change(nodePin, func(tg *koinontest.Target) { tg.Versions["computers"]++ })
	waitStates(t, r.src, "renamed", func(m map[string]State) bool {
		return m[otherPin].Host == "lab-renamed" && len(m[otherPin].Items) == 1
	})

	// A new pin with the old name is another computer, with its own data.
	third := r.node.AddTarget(thirdPin)
	r.node.Change(thirdPin, func(tg *koinontest.Target) {
		tg.Auth, tg.Name = strings.Repeat("3", 64), "lab-1"
		tg.Versions["decisions"] = 1
	})
	_ = third
	r.node.Change(nodePin, func(tg *koinontest.Target) { tg.Versions["computers"]++ })
	m := waitStates(t, r.src, "third", func(m map[string]State) bool { return m[thirdPin].Status == Connected })
	if m[thirdPin].Host != "lab-1" || len(m[thirdPin].Items) != 0 || len(m[otherPin].Items) != 1 {
		t.Errorf("third %+v other %+v", m[thirdPin], m[otherPin])
	}

	// An unpaired computer stays on screen, disconnected, with its data.
	r.node.RemoveTarget(otherPin)
	r.node.Change(nodePin, func(tg *koinontest.Target) { tg.Versions["computers"]++ })
	m = waitStates(t, r.src, "unpaired", func(m map[string]State) bool { return m[otherPin].Code == "not_paired" })
	if m[otherPin].Status != Disconnected || len(m[otherPin].Items) != 1 || m[thirdPin].Status != Connected {
		t.Errorf("after unpairing: other %+v third %+v", m[otherPin], m[thirdPin])
	}
}

const thirdPin = "c0c1c2c3c4c5c6c7c8c9cacbcccdcecfd0d1d2d3d4d5d6d7d8d9dadbdcdddedf"

func TestSourceDropsADeletedRoom(t *testing.T) {
	r := newSourceRig(t, bothAuths(), nil)
	waitStates(t, r.src, "settled", settled)
	r.node.Change(otherPin, func(tg *koinontest.Target) {
		tg.Rooms = nil
		delete(tg.Posts, "R1")
		delete(tg.Versions, "R1")
	})
	waitStates(t, r.src, "room gone", func(m map[string]State) bool {
		return len(m[otherPin].Rooms) == 0 && len(m[otherPin].Items) == 1
	})
}

func TestSourceKeepsAnOfflineComputer(t *testing.T) {
	r := newSourceRig(t, bothAuths(), nil)
	waitStates(t, r.src, "settled", settled)
	r.node.Change(otherPin, func(tg *koinontest.Target) { tg.Offline, tg.Reason = true, "link_unreachable" })
	r.node.Change(nodePin, func(tg *koinontest.Target) { tg.Versions["computers"]++ })
	m := waitStates(t, r.src, "offline", func(m map[string]State) bool { return m[otherPin].Code == "computer_offline" })
	if m[otherPin].Status != Disconnected || len(m[otherPin].Items) != 1 || m[nodePin].Status != Connected {
		t.Errorf("offline: %+v", m)
	}
	r.node.Change(otherPin, func(tg *koinontest.Target) {
		tg.Offline = false
		tg.Items["P-1"]["revision"] = int64(5)
		tg.Versions["decisions"]++
	})
	r.node.Change(nodePin, func(tg *koinontest.Target) { tg.Versions["computers"]++ })
	waitStates(t, r.src, "back", func(m map[string]State) bool {
		o := m[otherPin]
		return o.Status == Connected && len(o.Items) == 1 && o.Items[0].Revision == 5
	})
}

func TestSourceGoesOnFromItsVersions(t *testing.T) {
	r := newSourceRig(t, bothAuths(), nil)
	waitStates(t, r.src, "settled", settled)
	before := reads(r.node, otherPin)
	r.mu.Lock()
	r.ends[0].Close()
	r.mu.Unlock()
	waitStates(t, r.src, "ended", func(m map[string]State) bool {
		return m[otherPin].Status == Disconnected && len(m[otherPin].Items) == 1
	})
	waitStates(t, r.src, "again", func(m map[string]State) bool {
		r.mu.Lock()
		defer r.mu.Unlock()
		return r.opens >= 2 && m[nodePin].Status == Connected
	})
	// Unchanged topics are not read again in the new session.
	time.Sleep(300 * time.Millisecond)
	after := reads(r.node, otherPin)
	for _, op := range []string{"board_list", "decision_get", "inbox", "room_list", "room_read"} {
		if after[op] != before[op] {
			t.Errorf("%s read again after the session ended: %d -> %d", op, before[op], after[op])
		}
	}
	r.node.Change(otherPin, func(tg *koinontest.Target) {
		tg.Inbox = append(tg.Inbox, map[string]any{"seq": int64(3), "body": "third"})
		tg.Versions["inbox"]++
	})
	waitStates(t, r.src, "new message", func(m map[string]State) bool {
		o := m[otherPin]
		return o.Status == Connected && len(o.Inbox) == 2 && o.Inbox[1].Body == "third"
	})
}

// fixedTransport answers each envelope with answer.
type fixedTransport struct {
	answer func(signed.Envelope, any) error
	done   chan struct{}
}

func (f *fixedTransport) Call(_ context.Context, e signed.Envelope, out any) error {
	return f.answer(e, out)
}
func (f *fixedTransport) Done() <-chan struct{} { return f.done }
func (f *fixedTransport) Close()                {}

// unitSource is a source with one confirmed computer and a session on answer.
func unitSource(t *testing.T, answer func(signed.Envelope, any) error) (*SignedSource, *Signed, *Watcher) {
	t.Helper()
	key := signed.Key{User: "robert", Private: ed25519.NewKeyFromSeed(bytes.Repeat([]byte{9}, 32))}
	src := NewSignedSource(SourceOptions{Host: Host{Label: "studio", NodePin: nodePin}, Access: Access{Key: key, Auths: bothAuths()},
		MinDelay: 2 * time.Millisecond, MaxDelay: 5 * time.Millisecond})
	src.mu.Lock()
	tg := src.target(otherPin)
	tg.paired, tg.confirmed = true, true
	setStatus(tg, Connected, "", "")
	src.mu.Unlock()
	sig := &Signed{opts: SignedOptions{NodePin: nodePin, Key: key, Auths: bothAuths()}, t: &fixedTransport{answer: answer, done: make(chan struct{})}}
	ctx, cancel := context.WithCancel(context.Background())
	w := sig.Watch(ctx, nil, make(chan Change), WatchOptions{})
	t.Cleanup(func() { cancel(); w.Stop() })
	return src, sig, w
}

func otherState(src *SignedSource) State {
	for _, s := range src.States() {
		if s.Pin == otherPin {
			return s
		}
	}
	return State{}
}

func computersAnswer(list string) string {
	return `{"node":{"pin":"` + nodePin + `","name":"desk"},"computers":[` + list + `]}`
}

func TestSourcePausesARefreshWhileOffline(t *testing.T) {
	var mu sync.Mutex
	offline, boardReads := true, 0
	src, sig, w := unitSource(t, func(e signed.Envelope, out any) error {
		mu.Lock()
		defer mu.Unlock()
		switch e.Op {
		case "computers":
			return json.Unmarshal([]byte(computersAnswer(`{"pin":"`+otherPin+`","name":"lab-1","state":"connected"}`)), out)
		case "board_list":
			boardReads++
			if offline {
				return &Refusal{Code: "computer_offline"}
			}
			return json.Unmarshal([]byte(`{"counts":{"waiting":0},"items":[],"more":false}`), out)
		}
		return &Refusal{Code: "unknown_operation"}
	})
	done := make(chan bool, 1)
	go func() {
		done <- src.apply(w.ctx, sig, w, Change{Pin: otherPin, Topics: map[string]string{"decisions": "00000000000000000002"}})
	}()
	time.Sleep(50 * time.Millisecond)
	mu.Lock()
	n := boardReads
	offline = false
	mu.Unlock()
	if s := otherState(src); s.Status != Disconnected || s.Code != "computer_offline" || n != 1 {
		t.Fatalf("offline refresh: %s %s after %d reads", s.Status, s.Code, n)
	}
	if err := src.readComputers(w.ctx, sig, w); err != nil {
		t.Fatal(err)
	}
	select {
	case ok := <-done:
		if !ok || otherState(src).Status != Connected {
			t.Errorf("after return: %v %+v", ok, otherState(src))
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the paused change did not go on after computers showed the computer back")
	}
}

func TestSourceUnpairingEndsARefresh(t *testing.T) {
	var reads atomic.Int64
	src, sig, w := unitSource(t, func(e signed.Envelope, out any) error {
		if e.Op == "computers" {
			return json.Unmarshal([]byte(computersAnswer("")), out)
		}
		reads.Add(1)
		return &Refusal{Code: "internal"}
	})
	done := make(chan bool, 1)
	go func() {
		done <- src.apply(w.ctx, sig, w, Change{Pin: otherPin, Topics: map[string]string{"decisions": "00000000000000000002"}})
	}()
	for reads.Load() < 2 {
		time.Sleep(time.Millisecond)
	}
	if err := src.readComputers(w.ctx, sig, w); err != nil {
		t.Fatal(err)
	}
	select {
	case ok := <-done:
		if ok {
			t.Error("an unpaired computer's change was done")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the refresh went on after the computer was unpaired")
	}
	n := reads.Load()
	time.Sleep(30 * time.Millisecond)
	if reads.Load() != n || otherState(src).Code != "not_paired" {
		t.Errorf("reads %d -> %d, code %q", n, reads.Load(), otherState(src).Code)
	}
}

func TestSourceShowsARoomOnlyWhenReadInFull(t *testing.T) {
	r := newSourceRig(t, bothAuths(), nil)
	waitStates(t, r.src, "settled", settled)
	r.node.Change(otherPin, func(tg *koinontest.Target) {
		tg.Fail["room_read"] = 1 << 20
		tg.Rooms = []map[string]any{{"room_id": "R1", "name": "renamed", "revision": int64(2)}}
		tg.Posts["R1"] = append(tg.Posts["R1"], map[string]any{"seq": int64(2), "body": "second"})
		tg.Versions["R1"]++
	})
	deadline := time.Now().Add(150 * time.Millisecond)
	for time.Now().Before(deadline) {
		if o := otherState(r.src); o.Rooms[0].Name != "ops" || len(o.Rooms[0].Posts) != 1 {
			t.Fatalf("a partly read room was shown: %+v", o.Rooms[0])
		}
		time.Sleep(5 * time.Millisecond)
	}
	r.node.Change(otherPin, func(tg *koinontest.Target) { tg.Fail["room_read"] = 0 })
	waitStates(t, r.src, "room read", func(m map[string]State) bool {
		rs := m[otherPin].Rooms
		return len(rs) == 1 && rs[0].Name == "renamed" && len(rs[0].Posts) == 2
	})
}
