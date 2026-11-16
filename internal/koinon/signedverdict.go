package koinon

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/rwcii/koinon-voice/internal/signed"
)

// KindVerdicts are the verdicts that each kind of item takes (Koinon kindVerdicts, R3).
var KindVerdicts = map[string][]string{
	"decision": {"approved", "declined", "feedback"},
	"task":     {"completed", "declined", "feedback"},
	"fyi":      {"acknowledged"},
}

// SnapshotDigest is the digest of an item as it is shown (Koinon contract D2.2). The
// dashboard puts it in the form; a verdict is signed only when the item held now has it.
func SnapshotDigest(it Item) (string, error) {
	return signed.Snapshot{
		Reference: it.Reference, Revision: it.Revision, Kind: it.Kind, Title: it.Title,
		Where: it.Where, Context: it.Context, Recommendation: it.Recommendation,
		AgentText: it.AgentText, Commands: it.Commands,
	}.Digest()
}

// VerdictRequest is a verdict on one item of one computer, as the maintainer was shown it.
type VerdictRequest struct {
	Pin       string
	Reference string
	Revision  int64
	Digest    string
	Verdict   string
	Note      string
	// Intent identifies the form that the page rendered for this item; the same form sent
	// again is answered with the first sending's result and makes no new write.
	Intent string
}

// VerdictResult is what became of a verdict.
type VerdictResult struct {
	// State is recorded, refused, not_run or unknown.
	State WriteState
	// Code and Details are the refusal's; Local is true when Voice refused it itself and
	// sent nothing.
	Code    string
	Details json.RawMessage
	Local   bool
	// Final is true for an unknown outcome that can no longer be learned.
	Final  bool
	Reason string
	// Pending: an earlier verdict on this item waits for its outcome; this one sent nothing.
	Pending bool
	// Repeat: this form was sent before; this is that sending's result and nothing was sent.
	Repeat bool
}

// DetailText is a refusal's details as short text for the page, at most 200 bytes.
func DetailText(d json.RawMessage) string {
	var v any
	if len(d) == 0 || json.Unmarshal(d, &v) != nil || v == nil {
		return ""
	}
	b, _ := json.Marshal(v)
	if len(b) > 200 {
		return string(b[:200]) + "…"
	}
	return string(b)
}

// PendingWrite is a verdict whose outcome Voice does not know, as a State shows it. It holds
// no authorization, signature or key.
type PendingWrite struct {
	Request string
	// Target is the computer the verdict is for; it differs from the State's pin while the
	// record waits on the node before the session lists that computer.
	Target    string
	Reference string
	Verdict   string
	// Done: the outcome is known (State, Code, Detail), learned after the answer was lost;
	// it shows until the maintainer removes the notice.
	Done   bool
	State  WriteState
	Final  bool
	Code   string
	Detail string
	Reason string
}

// op is a verdict on (target, reference) that this process sent or recovered.
type op struct {
	w       Write
	result  VerdictResult
	running bool
}

func opKey(pin, ref string) string { return pin + "\x00" + ref }

// Verdict signs a verdict on the item that this source holds for the pin, only when it is
// at the revision and digest that the maintainer was shown, and sends it once. The write is
// in the journal before it is sent; an unknown outcome is asked about until it is known.
func (s *SignedSource) Verdict(ctx context.Context, req VerdictRequest) VerdictResult {
	local := func(code, reason string) VerdictResult {
		return VerdictResult{State: Refused, Code: code, Reason: reason, Local: true}
	}
	if s.opts.Journal.Path == "" {
		return local("no_journal", "no pending writes file is set")
	}
	if req.Intent == "" {
		return local("invalid_request", "the form has no intent token; read the page again")
	}
	key := opKey(req.Pin, req.Reference)
	s.mu.Lock()
	if request, ok := s.intents[req.Intent]; ok {
		res := s.outcomes[request]
		s.mu.Unlock()
		res.Repeat = true
		return res
	}
	if _, ok := s.issued[req.Intent]; !ok {
		s.mu.Unlock()
		return local("invalid_request", "the form was not issued by this Koinon Voice, or has expired; read the page again")
	}
	t, ok := s.targets[req.Pin]
	if !ok || !t.paired {
		s.mu.Unlock()
		return local("unknown_target", "this node does not list that computer")
	}
	if r := s.opts.Access.NodeReason(req.Pin); r != "" {
		s.mu.Unlock()
		return local("not_enrolled", r)
	}
	if o, ok := s.ops[key]; ok {
		res := o.result
		s.mu.Unlock()
		res.Pending = true
		return res
	}
	item, ok := t.items[req.Reference]
	digest, _ := SnapshotDigest(item)
	if !ok || item.Revision != req.Revision || digest != req.Digest {
		s.mu.Unlock()
		return local("snapshot_changed", "the item changed since the page showed it; read it again")
	}
	if !allowed(item.Kind, req.Verdict) {
		s.mu.Unlock()
		return local("invalid_verdict", fmt.Sprintf("a %s item does not take %q", item.Kind, req.Verdict))
	}
	sig, sctx := s.sig, s.sctx
	if sig == nil || t.offline {
		s.mu.Unlock()
		return VerdictResult{State: NotRun, Code: "computer_offline", Reason: "the computer is not connected now; nothing was sent", Local: true}
	}
	shown := item
	shown.Commands = append([]string{}, item.Commands...)
	w, err := sig.NewVerdict(req.Pin, shown, req.Verdict, req.Note)
	if err != nil {
		s.mu.Unlock()
		return local("sign_failed", err.Error())
	}
	o := &op{w: w, result: VerdictResult{State: Unknown, Reason: "waiting for the answer"}}
	request := w.envelope.Request
	s.ops[key] = o
	delete(s.issued, req.Intent)
	s.intents[req.Intent] = request
	s.outcomes[request] = o.result
	s.mu.Unlock()

	entry := Entry{NodePin: s.opts.Host.NodePin, Node: s.opts.Host.Label, Envelope: w.Envelope(), FirstSent: time.Now().Unix()}
	if existing, err := s.opts.Journal.Reserve(entry); err != nil {
		s.mu.Lock()
		delete(s.ops, key)
		delete(s.intents, req.Intent)
		delete(s.outcomes, request)
		s.issued[req.Intent] = time.Now() // nothing was sent: the form can be sent again
		s.mu.Unlock()
		if errors.Is(err, ErrPending) {
			return VerdictResult{State: Unknown, Pending: true, Final: existing.Final != "",
				Reason: "a verdict on this item through " + existing.Node + " is waiting for its outcome; nothing was sent"}
		}
		return local("journal_failed", "the pending writes file could not be written, so nothing was sent: "+err.Error())
	}
	res := sig.Submit(ctx, w, nil)
	switch res.State {
	case Recorded, Refused, NotRun:
		return s.finish(sctx, key, req.Pin, entry, resultOf(res.State, res.Refusal, res.Err), false)
	}
	s.mu.Lock()
	o.result = resultOf(Unknown, res.Refusal, res.Err)
	s.outcomes[request] = o.result
	s.trackLocked(req.Pin, entry, o.result)
	out := o.result
	s.mu.Unlock()
	s.publish()
	go s.reconcile(sctx, sig, key)
	return out
}

// finish records a write's known result: its journal entry is removed and the result kept for
// a repeated form. When removal fails, the result stays shown with that reason, the item
// stays reserved, and removal is tried again. inView shows the result on the target, for an
// outcome learned after the answer was lost.
func (s *SignedSource) finish(ctx context.Context, key, pin string, e Entry, res VerdictResult, inView bool) VerdictResult {
	request := e.Request()
	// The result is shown before its record leaves the journal.
	s.update(func() {
		s.outcomes[request] = res
		if o, ok := s.ops[key]; ok {
			o.result = res
		}
		if inView {
			s.doneLocked(pin, e, res)
		}
	})
	err := s.opts.Journal.Remove(request)
	s.update(func() {
		if err != nil {
			res.Reason = joinReason(res.Reason, "the local pending record could not be removed yet ("+err.Error()+"); trying again")
			s.outcomes[request] = res
			if o, ok := s.ops[key]; ok {
				o.result = res
			}
			s.doneLocked(pin, e, res)
			return
		}
		delete(s.ops, key)
	})
	if err != nil && ctx != nil {
		go s.retryRemove(ctx, key, pin, request)
	}
	return res
}

func joinReason(a, b string) string {
	if a == "" {
		return b
	}
	return a + "; " + b
}

// retryRemove removes a finished write's journal entry until it succeeds or the session
// ends; the next start asks about a remaining entry and finds the same outcome.
func (s *SignedSource) retryRemove(ctx context.Context, key, pin, request string) {
	delay := s.opts.MinDelay
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(delay):
		}
		if s.opts.Journal.Remove(request) == nil {
			s.update(func() {
				delete(s.ops, key)
				if t, ok := s.targets[pin]; ok && t.pending != nil {
					if p, ok := t.pending[request]; ok {
						p.Reason = s.outcomes[request].Reason
						t.pending[request] = p
						publishPending(t)
					}
				}
			})
			return
		}
		delay = min(2*delay, s.opts.MaxDelay)
	}
}

// doneLocked shows a write's known outcome on its target; the caller holds mu.
func (s *SignedSource) doneLocked(pin string, e Entry, res VerdictResult) {
	t := s.target(pin)
	if t.pending == nil {
		t.pending = map[string]PendingWrite{}
	}
	verdict, _ := e.Envelope.Args["verdict"].(string)
	t.pending[e.Request()] = PendingWrite{Request: e.Request(), Target: e.Envelope.Target, Reference: e.Reference(), Verdict: verdict, Done: true,
		State: res.State, Code: res.Code, Detail: DetailText(res.Details), Reason: res.Reason}
	publishPending(t)
}

// IntentLifetime is how long an issued form stays usable.
const IntentLifetime = 24 * time.Hour

// NewIntent issues a token for one rendered verdict form. A verdict is signed only for a token
// that this source issued and that no earlier verdict used; a used token is answered with its
// first result.
func (s *SignedSource) NewIntent() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	token := hex.EncodeToString(b)
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	for t, at := range s.issued {
		if now.Sub(at) > IntentLifetime {
			delete(s.issued, t)
		}
	}
	s.issued[token] = now
	return token
}

func allowed(kind, verdict string) bool {
	for _, v := range KindVerdicts[kind] {
		if v == verdict {
			return true
		}
	}
	return false
}

func resultOf(state WriteState, r *Refusal, err error) VerdictResult {
	res := VerdictResult{State: state}
	if r != nil {
		res.Code, res.Details, res.Reason = r.Code, r.Details, r.Message
	} else if err != nil {
		res.Reason = err.Error()
	}
	return res
}

func entryOf(w Write) Entry { return Entry{Envelope: w.envelope} }

// trackLocked shows a pending write on its target; the caller holds mu.
func (s *SignedSource) trackLocked(pin string, e Entry, res VerdictResult) {
	t := s.target(pin)
	if t.pending == nil {
		t.pending = map[string]PendingWrite{}
	}
	verdict, _ := e.Envelope.Args["verdict"].(string)
	t.pending[e.Request()] = PendingWrite{Request: e.Request(), Target: e.Envelope.Target, Reference: e.Reference(), Verdict: verdict,
		State: Unknown, Final: res.Final, Code: res.Code, Detail: DetailText(res.Details), Reason: res.Reason}
	publishPending(t)
}

func publishPending(t *target) {
	list := make([]PendingWrite, 0, len(t.pending))
	for _, p := range t.pending {
		list = append(list, p)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Request < list[j].Request })
	t.state.Pending = list
}

// publish sends the current states to OnChange.
func (s *SignedSource) publish() { s.update(func() {}) }

// reconcile asks the target for an unknown write's outcome under its original envelope until
// it is known, the outcome can no longer be learned, or the session ends.
func (s *SignedSource) reconcile(ctx context.Context, sig *Signed, key string) {
	s.mu.Lock()
	o, ok := s.ops[key]
	if !ok || o.running {
		s.mu.Unlock()
		return
	}
	o.running = true
	w := o.w
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		o.running = false
		s.mu.Unlock()
	}()
	pin, request := w.envelope.Target, w.envelope.Request
	delay := s.opts.MinDelay
	for ctx.Err() == nil {
		r := sig.Reconcile(ctx, w, nil)
		if ctx.Err() != nil {
			return
		}
		if r.Final && r.State == Unknown {
			reason := "the outcome can no longer be asked; read the record on that computer"
			code := ""
			if r.Refusal != nil {
				code = r.Refusal.Code
				reason += " (" + code + ")"
			}
			if err := s.opts.Journal.Update(request, func(e *Entry) { e.Final, e.Reason = "unknown", reason }); err != nil {
				// Not final until the journal says so: a restart would ask again.
				s.update(func() {
					o.result = VerdictResult{State: Unknown, Code: code,
						Reason: reason + "; the local pending record could not be marked (" + err.Error() + "); trying again"}
					s.outcomes[request] = o.result
					s.trackLocked(pin, entryOf(w), o.result)
				})
			} else {
				s.update(func() {
					o.result = VerdictResult{State: Unknown, Final: true, Code: code, Reason: reason}
					s.outcomes[request] = o.result
					s.trackLocked(pin, entryOf(w), o.result)
				})
				return
			}
		} else if r.Final {
			s.finish(ctx, key, pin, entryOf(w), resultOf(r.State, r.Refusal, r.Err), true)
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(delay):
		}
		delay = min(2*delay, s.opts.MaxDelay)
	}
}

// showJournal shows this node's journal entries on the node, with reason, before they can be
// asked about: the key, the enrollment or the session may be missing. It sends nothing and
// checks nothing; recover moves each entry to its computer when the session is up.
func (s *SignedSource) showJournal(reason string) {
	if s.opts.Journal.Path == "" {
		return
	}
	entries, err := s.opts.Journal.Load()
	node := s.opts.Host.NodePin
	s.update(func() {
		n := s.targets[node]
		if err != nil {
			n.state.Code, n.state.Reason = "journal_unreadable", err.Error()
			return
		}
		for _, e := range entries {
			if e.NodePin != node {
				continue
			}
			res := VerdictResult{State: Unknown, Reason: reason}
			if e.Final != "" {
				res = VerdictResult{State: Unknown, Final: true, Reason: e.Reason}
			}
			s.trackLocked(node, e, res)
		}
	})
}

// recover reads this node's entries in the journal at the start of a session: a usable
// entry is asked about under its original envelope; one that cannot be used, or whose
// outcome can no longer be learned, is shown with the reason and not sent.
func (s *SignedSource) recover(ctx context.Context, sig *Signed) {
	if s.opts.Journal.Path == "" {
		return
	}
	entries, err := s.opts.Journal.Load()
	if err != nil {
		s.update(func() { s.failed(s.opts.Host.NodePin, err) })
		return
	}
	var keys []string
	s.update(func() {
		node := s.targets[s.opts.Host.NodePin]
		for _, e := range entries {
			if e.NodePin != s.opts.Host.NodePin {
				continue // another node's entry; board and dashboard show it
			}
			if e.Envelope.Target != s.opts.Host.NodePin && node.pending != nil {
				delete(node.pending, e.Request()) // shown on its computer from now on
				publishPending(node)
			}
			pin := e.Envelope.Target
			if signed.CheckPin(pin) != nil {
				pin = s.opts.Host.NodePin
			}
			res := VerdictResult{State: Unknown, Reason: "asking the computer for the outcome"}
			usable := false
			switch reason := checkEntry(e, s.opts.Access.Key); {
			case reason != "":
				res = VerdictResult{State: Unknown, Final: true, Code: "invalid_entry",
					Reason: reason + "; nothing was sent. Its record is in " + s.opts.Journal.Path}
			case e.Final != "":
				res = VerdictResult{State: Unknown, Final: true, Reason: e.Reason}
			case s.targetPaired(pin) && s.opts.Access.NodeReason(pin) == "":
				usable = true
			default:
				res.Reason = "this computer is not paired with the node or not enrolled now; nothing was sent"
			}
			s.trackLocked(pin, e, res)
			s.outcomes[e.Request()] = res
			if !usable {
				continue
			}
			key := opKey(pin, e.Reference())
			if _, ok := s.ops[key]; !ok {
				s.ops[key] = &op{w: Write{envelope: e.Envelope}, result: res}
			}
			keys = append(keys, key)
		}
	})
	for _, key := range keys {
		go s.reconcile(ctx, sig, key)
	}
}

func (s *SignedSource) targetPaired(pin string) bool {
	t, ok := s.targets[pin]
	return ok && t.paired
}

// Dismiss removes a notice: a write's known outcome, or a write whose outcome can no longer
// be learned or whose record cannot be used, which also leaves the journal. It sends
// nothing, and a form already sent stays answered by its result.
func (s *SignedSource) Dismiss(request string) error {
	s.mu.Lock()
	var found *target
	var p PendingWrite
	for _, t := range s.targets {
		if q, ok := t.pending[request]; ok && (q.Final || q.Done) {
			found, p = t, q
		}
	}
	s.mu.Unlock()
	if found == nil {
		return errors.New("no such notice to remove")
	}
	if p.Final {
		if err := s.opts.Journal.Remove(request); err != nil {
			return err
		}
	}
	s.update(func() {
		delete(found.pending, request)
		if p.Final {
			delete(s.ops, opKey(found.state.Pin, p.Reference))
			res := s.outcomes[request]
			res.Reason = joinReason(res.Reason, "its local record was removed")
			s.outcomes[request] = res
		}
		publishPending(found)
	})
	return nil
}
