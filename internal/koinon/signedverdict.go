package koinon

import (
	"context"
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
}

// PendingWrite is a verdict whose outcome Voice does not know, as a State shows it. It holds
// no authorization, signature or key.
type PendingWrite struct {
	Request   string
	Reference string
	Verdict   string
	Final     bool
	Code      string
	Reason    string
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
	key := opKey(req.Pin, req.Reference)
	s.mu.Lock()
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
	s.ops[key] = o
	s.mu.Unlock()

	entry := Entry{NodePin: s.opts.Host.NodePin, Node: s.opts.Host.Label, Envelope: w.Envelope(), FirstSent: time.Now().Unix()}
	if existing, err := s.opts.Journal.Reserve(entry); err != nil {
		s.mu.Lock()
		delete(s.ops, key)
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
		// A failed removal leaves an entry that the next start asks about: harmless.
		s.opts.Journal.Remove(w.envelope.Request)
		s.mu.Lock()
		delete(s.ops, key)
		s.mu.Unlock()
		return resultOf(res.State, res.Refusal, res.Err)
	}
	s.mu.Lock()
	o.result = resultOf(Unknown, res.Refusal, res.Err)
	s.trackLocked(req.Pin, entry, o.result)
	s.mu.Unlock()
	s.publish()
	go s.reconcile(sctx, sig, key)
	return o.result
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

// trackLocked shows a pending write on its target; the caller holds mu.
func (s *SignedSource) trackLocked(pin string, e Entry, res VerdictResult) {
	t := s.target(pin)
	if t.pending == nil {
		t.pending = map[string]PendingWrite{}
	}
	verdict, _ := e.Envelope.Args["verdict"].(string)
	t.pending[e.Request()] = PendingWrite{Request: e.Request(), Reference: e.Reference(), Verdict: verdict,
		Final: res.Final, Code: res.Code, Reason: res.Reason}
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
		if r.Final {
			if r.State == Unknown {
				reason := "the outcome can no longer be asked; read the record on that computer"
				if r.Refusal != nil {
					reason += " (" + r.Refusal.Code + ")"
				}
				s.opts.Journal.Update(request, func(e *Entry) { e.Final, e.Reason = "unknown", reason })
				s.update(func() {
					o.result = VerdictResult{State: Unknown, Final: true, Reason: reason}
					if r.Refusal != nil {
						o.result.Code = r.Refusal.Code
					}
					if t, ok := s.targets[pin]; ok && t.pending != nil {
						p := t.pending[request]
						p.Final, p.Code, p.Reason = true, o.result.Code, reason
						t.pending[request] = p
						publishPending(t)
					}
				})
				return
			}
			if err := s.opts.Journal.Remove(request); err != nil {
				// Kept in the journal; the next start asks about it again.
				return
			}
			s.update(func() {
				delete(s.ops, key)
				if t, ok := s.targets[pin]; ok && t.pending != nil {
					delete(t.pending, request)
					publishPending(t)
				}
			})
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
		for _, e := range entries {
			if e.NodePin != s.opts.Host.NodePin {
				continue // another node's entry; board and dashboard show it
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

// Dismiss removes a pending write whose outcome can no longer be learned, or that cannot be
// used, from the journal and the view. It sends nothing.
func (s *SignedSource) Dismiss(request string) error {
	s.mu.Lock()
	var found *target
	for _, t := range s.targets {
		if p, ok := t.pending[request]; ok && p.Final {
			found = t
		}
	}
	s.mu.Unlock()
	if found == nil {
		return errors.New("no such record whose outcome can no longer be learned")
	}
	if err := s.opts.Journal.Remove(request); err != nil {
		return err
	}
	s.update(func() {
		p := found.pending[request]
		delete(found.pending, request)
		delete(s.ops, opKey(found.state.Pin, p.Reference))
		publishPending(found)
	})
	return nil
}
