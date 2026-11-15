package koinon

import (
	"encoding/json"
	"errors"
	"fmt"
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

// shownRequest is a verdict on the item as the source holds it now.
func shownRequest(t *testing.T, src *SignedSource, verdict string) VerdictRequest {
	t.Helper()
	it := otherState(src).Items[0]
	d, err := SnapshotDigest(it)
	if err != nil {
		t.Fatal(err)
	}
	return VerdictRequest{Pin: otherPin, Reference: it.Reference, Revision: it.Revision, Digest: d, Verdict: verdict, Intent: src.NewIntent()}
}

// verdictRequests counts the distinct request IDs of the verdicts that reached the node.
func verdictRequests(n *koinontest.SignedNode) map[string]int {
	m := map[string]int{}
	n.Change(nodePin, func(*koinontest.Target) {
		for _, e := range n.Seen {
			if e.Op == "decision_verdict" {
				m[e.Request]++
			}
		}
	})
	return m
}

func recorded(n *koinontest.SignedNode) int {
	c := 0
	n.Change(otherPin, func(tg *koinontest.Target) { c = len(tg.Verdicts) })
	return c
}

func journalLen(t *testing.T, j Journal) int {
	t.Helper()
	m, err := j.Load()
	if err != nil {
		t.Fatal(err)
	}
	return len(m)
}

func TestVerdictSignsTheShownItem(t *testing.T) {
	r := newSourceRig(t, bothAuths(), nil)
	waitStates(t, r.src, "settled", settled)
	res := r.src.Verdict(ctx(t), shownRequest(t, r.src, "approved"))
	if res.State != Recorded || recorded(r.node) != 1 || journalLen(t, r.journal) != 0 {
		t.Errorf("verdict %+v, %d recorded", res, recorded(r.node))
	}
}

func TestVerdictRefusesWhatWasNotShown(t *testing.T) {
	r := newSourceRig(t, bothAuths(), nil)
	waitStates(t, r.src, "settled", settled)
	for name, tc := range map[string]struct {
		change func(*VerdictRequest)
		code   string
	}{
		"digest":     {func(v *VerdictRequest) { v.Digest = strings.Repeat("0", 64) }, "snapshot_changed"},
		"revision":   {func(v *VerdictRequest) { v.Revision++ }, "snapshot_changed"},
		"reference":  {func(v *VerdictRequest) { v.Reference = "P-9" }, "snapshot_changed"},
		"kind":       {func(v *VerdictRequest) { v.Verdict = "dismissed" }, "invalid_verdict"},
		"other pin":  {func(v *VerdictRequest) { v.Pin = thirdPin }, "unknown_target"},
		"legacy pin": {func(v *VerdictRequest) { v.Pin = "" }, "unknown_target"},
	} {
		req := shownRequest(t, r.src, "approved")
		tc.change(&req)
		res := r.src.Verdict(ctx(t), req)
		if res.State != Refused || !res.Local || res.Code != tc.code {
			t.Errorf("%s: %+v", name, res)
		}
	}
	if n := len(verdictRequests(r.node)); n != 0 || journalLen(t, r.journal) != 0 {
		t.Errorf("%d verdicts sent", n)
	}
}

func TestVerdictOfflineIsNotRunAtOnce(t *testing.T) {
	r := newSourceRig(t, bothAuths(), nil)
	waitStates(t, r.src, "settled", settled)
	req := shownRequest(t, r.src, "approved")
	r.node.Change(otherPin, func(tg *koinontest.Target) { tg.Offline = true })
	res := r.src.Verdict(ctx(t), req)
	if res.State != NotRun || res.Code != "computer_offline" || journalLen(t, r.journal) != 0 || recorded(r.node) != 0 {
		t.Errorf("offline verdict %+v", res)
	}
}

func TestVerdictSendsNothingWithoutTheJournal(t *testing.T) {
	r := newSourceRig(t, bothAuths(), nil, func(r *sourceRig) {
		blocker := filepath.Join(filepath.Dir(r.journal.Path), "file")
		os.WriteFile(blocker, nil, 0o600)
		r.src.opts.Journal = Journal{Path: filepath.Join(blocker, "pending.json")}
	})
	waitStates(t, r.src, "settled", settled)
	res := r.src.Verdict(ctx(t), shownRequest(t, r.src, "approved"))
	if res.Code != "journal_failed" || !res.Local || len(verdictRequests(r.node)) != 0 {
		t.Errorf("verdict with an unwritable journal: %+v", res)
	}
}

func TestVerdictLostAnswerIsAskedAndRecordedOnce(t *testing.T) {
	r := newSourceRig(t, bothAuths(), nil)
	waitStates(t, r.src, "settled", settled)
	req := shownRequest(t, r.src, "approved")
	r.node.Change(otherPin, func(tg *koinontest.Target) { tg.DropNext = true })
	res := r.src.Verdict(ctx(t), req)
	if res.State != Unknown {
		t.Fatalf("lost answer: %+v", res)
	}
	waitRecorded(t, r)
	if recorded(r.node) != 1 || len(verdictRequests(r.node)) != 1 || journalLen(t, r.journal) != 0 {
		t.Errorf("%d recorded, requests %v", recorded(r.node), verdictRequests(r.node))
	}
}

func TestVerdictWhilePendingSendsNothing(t *testing.T) {
	r := newSourceRig(t, bothAuths(), nil)
	waitStates(t, r.src, "settled", settled)
	req := shownRequest(t, r.src, "approved")
	// Another node's write for the same computer and item waits for its outcome.
	other := signedEntry(t, r, strings.Repeat("e", 64), "old-label", time.Now().Unix())
	if _, err := r.journal.Reserve(other); err != nil {
		t.Fatal(err)
	}
	res := r.src.Verdict(ctx(t), req)
	if !res.Pending || res.State != Unknown || len(verdictRequests(r.node)) != 0 {
		t.Errorf("verdict while another waits: %+v", res)
	}
}

func TestVerdictConcurrentClicksMakeOneWrite(t *testing.T) {
	r := newSourceRig(t, bothAuths(), nil)
	waitStates(t, r.src, "settled", settled)
	req := shownRequest(t, r.src, "approved")
	r.node.Change(otherPin, func(tg *koinontest.Target) { tg.DropNext = true })
	var wg sync.WaitGroup
	results := make([]VerdictResult, 8)
	for i := range results {
		wg.Add(1)
		go func() {
			defer wg.Done()
			q := req
			q.Intent = r.src.NewIntent() // separate forms for one item
			results[i] = r.src.Verdict(ctx(t), q)
		}()
	}
	wg.Wait()
	waitRecorded(t, r)
	if n := len(verdictRequests(r.node)); n != 1 || recorded(r.node) != 1 {
		t.Errorf("%d request IDs, %d recorded: %+v", n, recorded(r.node), results)
	}
}

// signedEntry is a journal entry for a verdict on P-1 of the other computer, signed by the
// rig's key under the target's current authorization.
func signedEntry(t *testing.T, r *sourceRig, nodePinOf, label string, created int64) Entry {
	t.Helper()
	it := otherState(r.src).Items
	var shown Item
	if len(it) > 0 {
		shown = it[0]
	} else {
		shown = Item{Reference: "P-1", Revision: 2, Kind: "decision", Title: "Ship it", Where: "here",
			Context: "Approve it", Recommendation: "Yes.", Commands: []string{"go test ./..."}}
	}
	sig := &Signed{opts: SignedOptions{NodePin: nodePin, Key: r.key, Auths: bothAuths(),
		Now: func() time.Time { return time.Unix(created, 0) }}}
	w, err := sig.NewVerdict(otherPin, shown, "approved", "")
	if err != nil {
		t.Fatal(err)
	}
	return Entry{NodePin: nodePinOf, Node: label, Envelope: w.Envelope(), FirstSent: created}
}

func TestVerdictRecoveredAfterARestartBeforeSending(t *testing.T) {
	var e Entry
	r := newSourceRig(t, bothAuths(), nil, func(r *sourceRig) {
		// A renamed row: the entry's label is the old one; its node pin routes it.
		e = signedEntry(t, r, nodePin, "old-label", time.Now().Unix())
		if _, err := r.journal.Reserve(e); err != nil {
			t.Fatal(err)
		}
	})
	waitRecorded(t, r)
	reqs := verdictRequests(r.node)
	if recorded(r.node) != 1 || len(reqs) != 1 || reqs[e.Request()] != 1 || journalLen(t, r.journal) != 0 {
		t.Errorf("recovery sent %v, %d recorded", reqs, recorded(r.node))
	}
}

func TestVerdictRecoveredAfterALostAnswer(t *testing.T) {
	var e Entry
	r := newSourceRig(t, bothAuths(), nil, func(r *sourceRig) {
		e = signedEntry(t, r, nodePin, "studio", time.Now().Unix())
		if _, err := r.journal.Reserve(e); err != nil {
			t.Fatal(err)
		}
		// The earlier process sent it; the target recorded it and the answer was lost.
		cr, cw := io.Pipe()
		nr, nw := io.Pipe()
		go func() { r.node.Serve(cr, nw); nw.Close() }()
		old, err := OpenSigned(ctx(t), SignedOptions{NodePin: nodePin, Client: ClientName, Key: r.key, Auths: bothAuths()}, nr, cw)
		if err != nil {
			t.Fatal(err)
		}
		r.node.Change(otherPin, func(tg *koinontest.Target) { tg.DropNext = true })
		if res := old.Submit(ctx(t), Write{envelope: e.Envelope}, nil); res.State != Unknown {
			t.Fatalf("earlier sending: %+v", res)
		}
		old.Close()
	})
	waitRecorded(t, r)
	if recorded(r.node) != 1 || len(verdictRequests(r.node)) != 1 || journalLen(t, r.journal) != 0 {
		t.Errorf("%d recorded, requests %v", recorded(r.node), verdictRequests(r.node))
	}
	if reads(r.node, otherPin)["outcome"] == 0 {
		t.Error("the outcome was not asked first")
	}
}

func TestVerdictFinalUnknownStaysAndIsNotAskedAgain(t *testing.T) {
	var e Entry
	r := newSourceRig(t, bothAuths(), nil, func(r *sourceRig) {
		e = signedEntry(t, r, nodePin, "studio", time.Now().Unix())
		r.journal.Reserve(e)
		// The key was enrolled again on the target since the write.
		r.node.Change(otherPin, func(tg *koinontest.Target) { tg.Auth = strings.Repeat("9", 64) })
	})
	m := waitStates(t, r.src, "final", func(m map[string]State) bool {
		p := m[otherPin].Pending
		return len(p) == 1 && p[0].Final
	})
	if p := m[otherPin].Pending[0]; p.Code != "signature_refused" || !strings.Contains(p.Reason, "read the record") {
		t.Errorf("final unknown %+v", p)
	}
	j, _ := r.journal.Load()
	if j[e.Request()].Final != "unknown" || recorded(r.node) != 0 {
		t.Errorf("journal %+v", j)
	}
	// A new process shows it and does not ask again.
	before := reads(r.node, otherPin)["outcome"]
	r2 := newSourceRig(t, bothAuths(), nil, func(r2 *sourceRig) {
		r2.journal = r.journal
		r2.src.opts.Journal = r.journal
		r2.src.opts.Access.Key = r.key
	})
	waitStates(t, r2.src, "shown", func(m map[string]State) bool {
		p := m[otherPin].Pending
		return m[otherPin].Status == Connected && len(p) == 1 && p[0].Final
	})
	if n := reads(r2.node, otherPin)["outcome"]; n != 0 {
		t.Errorf("asked again %d times (first node %d)", n, before)
	}
	// Dismissing removes only the local record and sends nothing.
	outcomes := reads(r2.node, otherPin)["outcome"]
	if err := r2.src.Dismiss(e.Request()); err != nil {
		t.Fatal(err)
	}
	if journalLen(t, r2.journal) != 0 || len(otherState(r2.src).Pending) != 0 {
		t.Errorf("dismiss: journal %d, pending %v", journalLen(t, r2.journal), otherState(r2.src).Pending)
	}
	if verdictsIn(r2.node) != 0 || reads(r2.node, otherPin)["outcome"] != outcomes {
		t.Error("dismiss sent something")
	}
	if err := r2.src.Dismiss(e.Request()); err == nil {
		t.Error("a second dismiss found the record")
	}
}

func verdictsIn(n *koinontest.SignedNode) int { return len(verdictRequests(n)) }

func TestVerdictBadEntryIsShownNotSent(t *testing.T) {
	var e Entry
	r := newSourceRig(t, bothAuths(), nil, func(r *sourceRig) {
		e = signedEntry(t, r, nodePin, "studio", time.Now().Unix())
		e.Envelope.Args["note"] = "changed after signing"
		r.journal.Reserve(e)
	})
	m := waitStates(t, r.src, "invalid", func(m map[string]State) bool {
		p := m[otherPin].Pending
		return m[otherPin].Status == Connected && len(p) == 1 && p[0].Code == "invalid_entry"
	})
	p := m[otherPin].Pending[0]
	if !p.Final || !strings.Contains(p.Reason, "signature") || strings.Contains(p.Reason, e.Envelope.Sig) || strings.Contains(p.Reason, e.Envelope.Auth) {
		t.Errorf("bad entry %+v", p)
	}
	time.Sleep(50 * time.Millisecond)
	if verdictsIn(r.node) != 0 || reads(r.node, otherPin)["outcome"] != 0 {
		t.Error("a bad entry was sent or asked about")
	}
}

func TestVerdictIgnoresAnotherNodesEntry(t *testing.T) {
	var e Entry
	r := newSourceRig(t, bothAuths(), nil, func(r *sourceRig) {
		// The label "studio" now names another node; its entry stays with its own pin.
		e = signedEntry(t, r, strings.Repeat("e", 64), "studio", time.Now().Unix())
		r.journal.Reserve(e)
	})
	waitStates(t, r.src, "settled", settled)
	time.Sleep(50 * time.Millisecond)
	if len(otherState(r.src).Pending) != 0 || verdictsIn(r.node) != 0 || journalLen(t, r.journal) != 1 {
		t.Errorf("another node's entry was adopted: %+v", otherState(r.src).Pending)
	}
}

func TestJournalKeepsOtherEntries(t *testing.T) {
	j := Journal{Path: filepath.Join(t.TempDir(), "pending.json")}
	mk := func(req, target, ref, node string) Entry {
		return Entry{NodePin: node, Envelope: signed.Envelope{Request: req, Target: target, Args: map[string]any{"reference": ref}}}
	}
	if _, err := j.Reserve(mk("req-aaaaaaaaaaaaaaaa", otherPin, "P-1", nodePin)); err != nil {
		t.Fatal(err)
	}
	if _, err := j.Reserve(mk("req-bbbbbbbbbbbbbbbb", otherPin, "P-2", nodePin)); err != nil {
		t.Fatal(err)
	}
	// The same computer and item through another node is the same operation.
	if ex, err := j.Reserve(mk("req-cccccccccccccccc", otherPin, "P-1", thirdPin)); !errors.Is(err, ErrPending) || ex.Request() != "req-aaaaaaaaaaaaaaaa" {
		t.Errorf("second node: %v %+v", err, ex)
	}
	if err := j.Remove("req-aaaaaaaaaaaaaaaa"); err != nil {
		t.Fatal(err)
	}
	m, _ := j.Load()
	if len(m) != 1 || m["req-bbbbbbbbbbbbbbbb"].Reference() != "P-2" {
		t.Errorf("journal %v", m)
	}
	if info, err := os.Stat(j.Path); err != nil || info.Mode().Perm() != 0o600 {
		t.Errorf("journal mode %v %v", info.Mode(), err)
	}
	// Concurrent reservations of one operation: exactly one wins.
	var wg sync.WaitGroup
	var mu sync.Mutex
	won := 0
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			jj := Journal{Path: j.Path} // as another process would
			req := "req-" + strings.Repeat(string(rune('d'+i%20)), 16) + string(rune('a'+i))
			if _, err := jj.Reserve(mk(req, thirdPin, "P-7", nodePin)); err == nil {
				mu.Lock()
				won++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if won != 1 {
		t.Errorf("%d reservations of one operation", won)
	}
}

func TestVerdictSameFormIsAnsweredOnce(t *testing.T) {
	r := newSourceRig(t, bothAuths(), nil)
	waitStates(t, r.src, "settled", settled)
	req := shownRequest(t, r.src, "approved")
	first := r.src.Verdict(ctx(t), req)
	again := r.src.Verdict(ctx(t), req)
	if first.State != Recorded || again.State != Recorded || !again.Repeat || len(verdictRequests(r.node)) != 1 {
		t.Errorf("first %+v again %+v", first, again)
	}
	for _, intent := range []string{"", "made-up-by-the-client"} {
		req.Intent = intent
		if res := r.src.Verdict(ctx(t), req); res.Code != "invalid_request" || !res.Local {
			t.Errorf("intent %q: %+v", intent, res)
		}
	}
	if len(verdictRequests(r.node)) != 1 {
		t.Error("an unissued form was sent")
	}
}

func TestVerdictReconciledRefusalIsShownWithDetails(t *testing.T) {
	var e Entry
	var states []State
	var smu sync.Mutex
	r := newSourceRig(t, bothAuths(), nil, func(r *sourceRig) {
		e = signedEntry(t, r, nodePin, "studio", time.Now().Unix())
		r.journal.Reserve(e)
		// The item moved on before the write arrived: the target refuses it.
		r.node.Change(otherPin, func(tg *koinontest.Target) { tg.Items["P-1"]["revision"] = int64(9) })
		r.src.opts.OnChange = func(s []State) { smu.Lock(); states = s; smu.Unlock() }
	})
	m := waitStates(t, r.src, "refused", func(m map[string]State) bool {
		p := m[otherPin].Pending
		return len(p) == 1 && p[0].Done
	})
	p := m[otherPin].Pending[0]
	for deadline := time.Now().Add(2 * time.Second); journalLen(t, r.journal) != 0 && time.Now().Before(deadline); {
		time.Sleep(5 * time.Millisecond)
	}
	if p.State != Refused || p.Code != "stale_revision" || !strings.Contains(p.Detail, "9") || journalLen(t, r.journal) != 0 {
		t.Errorf("reconciled refusal %+v", p)
	}
	if err := r.src.Dismiss(e.Request()); err != nil || len(otherState(r.src).Pending) != 0 {
		t.Errorf("removing the notice: %v", err)
	}
	_ = states
}

func TestVerdictFinalUnknownWaitsForTheJournal(t *testing.T) {
	var e Entry
	r := newSourceRig(t, bothAuths(), nil, func(r *sourceRig) {
		e = signedEntry(t, r, nodePin, "studio", time.Now().Unix())
		r.journal.Reserve(e)
		r.node.Change(otherPin, func(tg *koinontest.Target) { tg.Auth = strings.Repeat("9", 64) })
		// Every later write of the journal fails.
		os.Mkdir(r.journal.Path+".tmp"+fmt.Sprint(os.Getpid()), 0o700)
	})
	m := waitStates(t, r.src, "not final", func(m map[string]State) bool {
		p := m[otherPin].Pending
		return len(p) == 1 && strings.Contains(p[0].Reason, "could not be marked")
	})
	if m[otherPin].Pending[0].Final {
		t.Error("shown final before the journal was marked")
	}
	if err := r.src.Dismiss(e.Request()); err == nil {
		t.Error("a record not marked final was dismissed")
	}
	os.Remove(r.journal.Path + ".tmp" + fmt.Sprint(os.Getpid()))
	waitStates(t, r.src, "final", func(m map[string]State) bool {
		p := m[otherPin].Pending
		return len(p) == 1 && p[0].Final
	})
	j, _ := r.journal.Load()
	if j[e.Request()].Final != "unknown" {
		t.Errorf("journal %+v", j[e.Request()])
	}
}

func TestVerdictDismissedFormStaysAnswered(t *testing.T) {
	r := newSourceRig(t, bothAuths(), nil)
	waitStates(t, r.src, "settled", settled)
	req := shownRequest(t, r.src, "approved")
	// Its answer is lost, and the key is enrolled again before the outcome is asked.
	r.node.Change(otherPin, func(tg *koinontest.Target) { tg.DropNext = true })
	r.src.mu.Lock()
	r.src.opts.Access.Auths = bothAuths()
	r.src.mu.Unlock()
	res := r.src.Verdict(ctx(t), req)
	if res.State != Unknown {
		t.Fatalf("lost answer: %+v", res)
	}
	r.node.Change(otherPin, func(tg *koinontest.Target) { tg.Auth = strings.Repeat("9", 64); tg.Verdicts = nil })
	m := waitStates(t, r.src, "final", func(m map[string]State) bool {
		p := m[otherPin].Pending
		return len(p) == 1 && p[0].Final
	})
	if err := r.src.Dismiss(m[otherPin].Pending[0].Request); err != nil {
		t.Fatal(err)
	}
	r.node.Change(otherPin, func(tg *koinontest.Target) { tg.Auth = strings.Repeat("2", 64) })
	sent := len(verdictRequests(r.node))
	again := r.src.Verdict(ctx(t), req)
	if !again.Repeat || len(verdictRequests(r.node)) != sent {
		t.Errorf("the dismissed form was sent again: %+v", again)
	}
}

func TestJournalRefusesWhatIsNotAnObject(t *testing.T) {
	for _, body := range []string{"null", "[]", "{}{}", "{} x", "7"} {
		j := Journal{Path: filepath.Join(t.TempDir(), "pending.json")}
		os.WriteFile(j.Path, []byte(body), 0o600)
		if _, err := j.Reserve(Entry{Envelope: signed.Envelope{Request: "req-aaaaaaaaaaaaaaaa"}}); err == nil {
			t.Errorf("%s: reserved", body)
		}
		if b, _ := os.ReadFile(j.Path); string(b) != body {
			t.Errorf("%s: the file changed to %s", body, b)
		}
	}
	var _ json.RawMessage
}

// recordedNotice: the other computer shows one write whose outcome was learned: recorded.
func recordedNotice(m map[string]State) bool {
	p := m[otherPin].Pending
	return m[otherPin].Status == Connected && len(p) == 1 && p[0].Done && p[0].State == Recorded
}

// waitRecorded waits for the recorded notice and for its record to leave the journal, which
// happens just after the notice is shown.
func waitRecorded(t *testing.T, r *sourceRig) {
	t.Helper()
	waitStates(t, r.src, "recorded", recordedNotice)
	for deadline := time.Now().Add(2 * time.Second); journalLen(t, r.journal) != 0 && time.Now().Before(deadline); {
		time.Sleep(5 * time.Millisecond)
	}
}
