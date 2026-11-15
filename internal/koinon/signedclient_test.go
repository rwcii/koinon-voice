package koinon

import (
	"bufio"
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rwcii/koinon-voice/internal/koinon/koinontest"
	"github.com/rwcii/koinon-voice/internal/signed"
)

const (
	nodePin  = "a0a1a2a3a4a5a6a7a8a9aaabacadaeafb0b1b2b3b4b5b6b7b8b9babbbcbdbebf"
	otherPin = "b0b1b2b3b4b5b6b7b8b9babbbcbdbebfc0c1c2c3c4c5c6c7c8c9cacbcccdcecf"
)

type fakeClock struct {
	mu  sync.Mutex
	now int64
}

func (c *fakeClock) Now() time.Time { c.mu.Lock(); defer c.mu.Unlock(); return time.Unix(c.now, 0) }
func (c *fakeClock) Add(s int64)    { c.mu.Lock(); c.now += s; c.mu.Unlock() }

// sentLog keeps every byte the client writes to the node.
type sentLog struct {
	mu sync.Mutex
	b  bytes.Buffer
	w  io.Writer
}

func (r *sentLog) Write(p []byte) (int, error) {
	r.mu.Lock()
	r.b.Write(p)
	r.mu.Unlock()
	return r.w.Write(p)
}

type rig struct {
	s     *Signed
	node  *koinontest.SignedNode
	clock *fakeClock
	key   signed.Key
	sent  *sentLog
}

func newRig(t *testing.T) *rig {
	t.Helper()
	seed := bytes.Repeat([]byte{7}, 32)
	key := signed.Key{User: "robert", Private: ed25519.NewKeyFromSeed(seed)}
	clock := &fakeClock{now: 1794699000}
	node := koinontest.NewSignedNode(nodePin, key.Public(), clock.Now)
	node.AddTarget(otherPin)
	auths := map[string]string{nodePin: strings.Repeat("1", 64), otherPin: strings.Repeat("2", 64)}
	node.Change(nodePin, func(tg *koinontest.Target) { tg.Auth = auths[nodePin] })
	node.Change(otherPin, func(tg *koinontest.Target) { tg.Items["P-1"] = item("Approve it"); tg.Auth = auths[otherPin] })
	cr, cw := io.Pipe()
	nr, nw := io.Pipe()
	go func() {
		node.Serve(cr, nw)
		nw.Close()
	}()
	sent := &sentLog{w: cw}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	s, err := OpenSigned(ctx, SignedOptions{NodePin: nodePin, Client: "koinon-voice", Key: key, Auths: auths, Now: clock.Now, Version: "test"}, nr, sent)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cw.Close() })
	return &rig{s: s, node: node, clock: clock, key: key, sent: sent}
}

func item(context string) map[string]any {
	return map[string]any{"reference": "P-1", "revision": int64(2), "kind": "decision", "title": "Ship it",
		"where": "here", "context": context, "recommendation": "Yes.", "agent_text": "", "commands": []string{"go test ./..."},
		"state": "active"}
}

func ctx(t *testing.T) context.Context {
	c, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)
	return c
}

func TestSignedSessionAndVerdict(t *testing.T) {
	r := newRig(t)
	shown, err := r.s.DecisionGet(ctx(t), otherPin, "P-1")
	if err != nil || shown.Context != "Approve it" || len(shown.Commands) != 1 {
		t.Fatalf("decision_get %+v %v", shown, err)
	}
	w, err := r.s.NewVerdict(otherPin, shown, "approved", "ok")
	if err != nil {
		t.Fatal(err)
	}
	var v map[string]any
	if res := r.s.Submit(ctx(t), w, &v); res.State != Recorded || v["verdict"] != "approved" {
		t.Fatalf("submit %+v %v", res, v)
	}
	// The private key goes nowhere: only the session line and public envelopes were sent.
	sent := r.sent.b.String()
	seed := base64.StdEncoding.EncodeToString(r.key.Private.Seed())
	if strings.Contains(sent, seed) || strings.Contains(sent, string(r.key.Private.Seed())) {
		t.Error("the private key was sent")
	}
	if !strings.HasPrefix(sent, `{"v":"koinon-maintainer-1"`) || !strings.Contains(strings.SplitN(sent, "\n", 2)[0], `"op":"session"`) {
		t.Errorf("first line is not the session envelope: %.120s", sent)
	}
}

func TestSignedVerdictOnChangedText(t *testing.T) {
	r := newRig(t)
	shown, _ := r.s.DecisionGet(ctx(t), otherPin, "P-1")
	// The target's text changes after it was shown, at the same revision.
	r.node.Change(otherPin, func(tg *koinontest.Target) { tg.Items["P-1"] = item("Other text") })
	w, _ := r.s.NewVerdict(otherPin, shown, "approved", "")
	res := r.s.Submit(ctx(t), w, nil)
	if res.State != Refused || res.Refusal.Code != "snapshot_changed" {
		t.Errorf("verdict on other text: %+v", res)
	}
	shown.Revision = 1
	w, _ = r.s.NewVerdict(otherPin, shown, "approved", "")
	res = r.s.Submit(ctx(t), w, nil)
	if cur, ok := res.Refusal.CurrentRevision(); res.State != Refused || !ok || cur != 2 {
		t.Errorf("stale revision: %+v", res)
	}
}

func TestSignedLostAnswerIsReconciledOnce(t *testing.T) {
	r := newRig(t)
	shown, _ := r.s.DecisionGet(ctx(t), otherPin, "P-1")
	r.node.Change(otherPin, func(tg *koinontest.Target) { tg.DropNext = true })
	w, _ := r.s.NewVerdict(otherPin, shown, "approved", "")
	if res := r.s.Submit(ctx(t), w, nil); res.State != Unknown {
		t.Fatalf("a dropped answer: %+v", res)
	}
	var v map[string]any
	rec := r.s.Reconcile(ctx(t), w, &v)
	if rec.State != Recorded || !rec.Final || v["verdict"] != "approved" {
		t.Errorf("reconcile %+v %v", rec, v)
	}
	// Sending the same envelope again answers from the receipt: still one verdict.
	if res := r.s.Submit(ctx(t), w, nil); res.State != Recorded {
		t.Errorf("resend %+v", res)
	}
	r.node.Change(otherPin, func(tg *koinontest.Target) {
		if len(tg.Verdicts) != 1 {
			t.Errorf("%d verdicts", len(tg.Verdicts))
		}
	})
}

func TestSignedOfflineAndUnknownRequest(t *testing.T) {
	r := newRig(t)
	shown, _ := r.s.DecisionGet(ctx(t), otherPin, "P-1")
	r.node.Change(otherPin, func(tg *koinontest.Target) { tg.Offline = true })
	w, _ := r.s.NewVerdict(otherPin, shown, "approved", "")
	if res := r.s.Submit(ctx(t), w, nil); res.State != NotRun || res.Refusal.Code != "computer_offline" {
		t.Errorf("offline: %+v", res)
	}
	r.node.Change(otherPin, func(tg *koinontest.Target) { tg.Offline = false })
	// Within the window an unknown request is sent again, once, with the same envelope.
	rec := r.s.Reconcile(ctx(t), w, nil)
	if rec.State != Recorded || !rec.Final {
		t.Errorf("within the window: %+v", rec)
	}
	// Past the window with no receipt: final, did not run; no new request is signed.
	w2, _ := r.s.NewVerdict(otherPin, shown, "declined", "")
	r.clock.Add(WindowSeconds + 1)
	before := len(r.node.Seen)
	rec = r.s.Reconcile(ctx(t), w2, nil)
	if rec.State != NotRun || !rec.Final {
		t.Errorf("past the window: %+v", rec)
	}
	for _, e := range r.node.Seen[before:] {
		if e.Op == "decision_verdict" {
			t.Errorf("a write was sent after the window: %+v", e)
		}
	}
}

func TestSignedAuthorizationChanged(t *testing.T) {
	r := newRig(t)
	shown, _ := r.s.DecisionGet(ctx(t), otherPin, "P-1")
	r.node.Change(otherPin, func(tg *koinontest.Target) { tg.DropNext = true })
	w, _ := r.s.NewVerdict(otherPin, shown, "approved", "")
	r.s.Submit(ctx(t), w, nil)
	// The key is removed and enrolled again with a new statement: a new authorization.
	newAuth := strings.Repeat("3", 64)
	r.node.Change(otherPin, func(tg *koinontest.Target) { tg.Auth = newAuth })
	r.s.opts.Auths[otherPin] = newAuth
	rec := r.s.Reconcile(ctx(t), w, nil)
	if rec.State != Unknown || !rec.Final {
		t.Errorf("authorization changed: %+v", rec)
	}
	// The next envelope carries the new authorization and runs.
	if _, err := r.s.DecisionGet(ctx(t), otherPin, "P-1"); err != nil {
		t.Errorf("after re-enrollment: %v", err)
	}
	// An envelope of the earlier authorization, replayed, is refused.
	var ref *Refusal
	if err := r.s.Send(ctx(t), w.Envelope(), nil); !errors.As(err, &ref) || ref.Code != "signature_refused" || ref.Reason() != "authorization_changed" {
		t.Errorf("an old envelope: %v", err)
	}
}

func TestSignedWatch(t *testing.T) {
	r := newRig(t)
	out := make(chan Change, 4)
	w := r.s.Watch(context.Background(), map[string]map[string]string{otherPin: {"decisions": "00000000000000000000"}}, out, WatchOptions{WaitSeconds: 1})
	defer w.Stop()
	w.Add(otherPin)
	r.node.Change(otherPin, func(tg *koinontest.Target) { tg.Versions["decisions"] = 1 })
	c := recv(t, out)
	if c.Pin != otherPin || c.Topics["decisions"] != "00000000000000000001" {
		t.Fatalf("change %+v", c)
	}
	// A change while the consumer reads is not lost: the next wait answers at once.
	r.node.Change(otherPin, func(tg *koinontest.Target) { tg.Versions["decisions"] = 2 })
	w.Done(otherPin, c.Topics, c.Removed)
	if c := recv(t, out); c.Topics["decisions"] != "00000000000000000002" {
		t.Fatalf("second change %+v", c)
	}
	w.Done(otherPin, map[string]string{"decisions": "00000000000000000002"}, nil)
	if v := w.Versions()[otherPin]["decisions"]; v != "00000000000000000002" {
		t.Errorf("stored %s", v)
	}
	// Offline pauses the wait until Resume.
	r.node.Change(otherPin, func(tg *koinontest.Target) { tg.Offline = true })
	if c := recv(t, out); !c.Offline {
		t.Fatalf("offline %+v", c)
	}
	select {
	case c := <-out:
		t.Fatalf("a paused wait sent %+v", c)
	case <-time.After(200 * time.Millisecond):
	}
	r.node.Change(otherPin, func(tg *koinontest.Target) { tg.Offline = false; tg.Versions["decisions"] = 3 })
	w.Resume(otherPin)
	if c := recv(t, out); c.Topics["decisions"] != "00000000000000000003" {
		t.Errorf("after resume %+v", c)
	}
	// A deleted topic is reported as removed and dropped once the consumer is done.
	w.Done(otherPin, map[string]string{"decisions": "00000000000000000003"}, nil)
	r.node.Change(otherPin, func(tg *koinontest.Target) { tg.Versions["room-1"] = 1 })
	if c := recv(t, out); c.Topics["room-1"] != "00000000000000000001" {
		t.Fatalf("new room %+v", c)
	}
	w.Done(otherPin, map[string]string{"room-1": "00000000000000000001"}, nil)
	r.node.Change(otherPin, func(tg *koinontest.Target) { delete(tg.Versions, "room-1") })
	c = recv(t, out)
	if len(c.Removed) != 1 || c.Removed[0] != "room-1" || len(c.Topics) != 0 {
		t.Fatalf("deleted room %+v", c)
	}
	w.Done(otherPin, nil, c.Removed)
	if _, ok := w.Versions()[otherPin]["room-1"]; ok {
		t.Error("a removed topic kept its version")
	}
	select {
	case c := <-out:
		t.Fatalf("a handled removal came again: %+v", c)
	case <-time.After(1500 * time.Millisecond):
	}
	// Remove stops the target's wait and keeps its versions.
	w.Remove(otherPin)
	if v := w.Versions()[otherPin]["decisions"]; v != "00000000000000000003" {
		t.Errorf("versions after remove %s", v)
	}
}

func recv(t *testing.T, out chan Change) Change {
	t.Helper()
	select {
	case c := <-out:
		return c
	case <-time.After(5 * time.Second):
		t.Fatal("no change")
	}
	return Change{}
}

func TestNewer(t *testing.T) {
	if !newer("00000000000000000002", "00000000000000000001") || newer("1", "00000000000000000001") || !newer("00000000000000000001", "") {
		t.Error("version order")
	}
}

func TestStartSignedProcess(t *testing.T) {
	key := signed.Key{User: "robert", Private: ed25519.NewKeyFromSeed(bytes.Repeat([]byte{9}, 32))}
	dir := t.TempDir()
	auths := map[string]string{nodePin: strings.Repeat("1", 64)}
	path := koinontest.WriteSigned(filepath.Join(dir, "koinon"), dir, koinontest.SignedConfig{Pin: nodePin, PublicKey: key.Public(), Auths: auths})
	opts := SignedOptions{Koinon: path, Args: []string{"--state-dir", "/tmp/x y"}, NodePin: nodePin, Client: "koinon-voice", Key: key, Auths: auths, Version: "test", Grace: time.Second}
	s, err := StartSigned(ctx(t), opts)
	if err != nil {
		t.Fatal(err)
	}
	var raw json.RawMessage
	if err := s.Read(ctx(t), nodePin, "computers", nil, &raw); err != nil || !strings.Contains(string(raw), nodePin) {
		t.Errorf("computers %s %v", raw, err)
	}
	s.Close()
	args, _ := os.ReadFile(filepath.Join(dir, "args"))
	if string(args) != "mcp\n--maintainer\n--signed\n--state-dir\n/tmp/x y" {
		t.Errorf("args %q", args)
	}
	seed := base64.StdEncoding.EncodeToString(key.Private.Seed())
	if strings.Contains(string(args), seed) {
		t.Error("the key was an argument")
	}
	// A session that the node refuses gives Koinon's code.
	opts.Auths = map[string]string{nodePin: strings.Repeat("9", 64)}
	_, err = StartSigned(ctx(t), opts)
	var r *Refusal
	if !errors.As(err, &r) || r.Code != "signature_refused" || strings.Contains(err.Error(), seed) {
		t.Errorf("refused session: %v", err)
	}
}

func TestSignedWriteIsImmutable(t *testing.T) {
	r := newRig(t)
	args := map[string]any{"reference": "P-1", "nested": map[string]any{"a": "b"}}
	w, err := r.s.NewWrite(otherPin, "ack", args)
	if err != nil {
		t.Fatal(err)
	}
	args["reference"] = "changed"
	args["nested"].(map[string]any)["a"] = "changed"
	view := w.Envelope()
	view.Args["reference"] = "changed too"
	if err := signed.Verify(r.key.Public(), w.Envelope()); err != nil || w.Envelope().Args["reference"] != "P-1" {
		t.Errorf("the write changed: %v %+v", err, w.Envelope().Args)
	}
}

// fixedNode answers each signed call with the next fixed text, to act out answers that the
// fake node does not make by itself.
func fixedNode(t *testing.T, answers ...string) *Signed {
	t.Helper()
	cr, cw := io.Pipe()
	nr, nw := io.Pipe()
	go func() {
		defer nw.Close()
		br := bufio.NewReader(cr)
		br.ReadBytes('\n') // the session line
		next := 0
		for {
			line, err := br.ReadBytes('\n')
			if err != nil {
				return
			}
			var m struct {
				ID     json.RawMessage `json:"id"`
				Method string          `json:"method"`
			}
			json.Unmarshal(line, &m)
			var result any = map[string]any{"protocolVersion": "2025-06-18"}
			if m.Method == "tools/call" {
				text := answers[next]
				next++
				result = map[string]any{"content": []map[string]any{{"type": "text", "text": text}}, "isError": strings.Contains(text, `"ok":false`)}
			} else if m.ID == nil {
				continue
			}
			b, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": m.ID, "result": result})
			nw.Write(append(b, '\n'))
		}
	}()
	t.Cleanup(func() { cw.Close() })
	key := signed.Key{User: "robert", Private: ed25519.NewKeyFromSeed(bytes.Repeat([]byte{5}, 32))}
	s, err := OpenSigned(ctx(t), SignedOptions{NodePin: nodePin, Client: "koinon-voice", Key: key,
		Auths: map[string]string{nodePin: strings.Repeat("1", 64), otherPin: strings.Repeat("2", 64)}, Version: "test"}, nr, cw)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestSignedOutcomeKeepsRefusalDetails(t *testing.T) {
	s := fixedNode(t, `{"ok":true,"result":{"state":"refused","code":"stale_revision","error":"stale","details":{"current_revision":4}}}`)
	w, _ := s.NewWrite(otherPin, "decision_verdict", map[string]any{"reference": "P-1", "revision": 3})
	rec := s.Reconcile(ctx(t), w, nil)
	if cur, ok := rec.Refusal.CurrentRevision(); rec.State != Refused || !rec.Final || !ok || cur != 4 || rec.Refusal.Message != "stale" {
		t.Errorf("refused receipt %+v %+v", rec, rec.Refusal)
	}
}

func TestSignedReenrolledDuringReconcileIsUnknown(t *testing.T) {
	s := fixedNode(t,
		`{"ok":true,"result":{"state":"unknown_request","now":1}}`,
		`{"ok":false,"code":"signature_refused","error":"refused","details":{"reason":"authorization_changed"}}`)
	w, _ := s.NewWrite(otherPin, "decision_verdict", map[string]any{"reference": "P-1"})
	w.envelope.Created = 1
	rec := s.Reconcile(ctx(t), w, nil)
	if rec.State != Unknown || !rec.Final {
		t.Errorf("re-enrolled between outcome and resend: %+v", rec)
	}
}
