package koinontest

import (
	"fmt"
	"sort"

	"github.com/rwcii/koinon-voice/internal/signed"
)

// RemoveTarget unpairs a computer: computers no longer lists it and its envelopes are refused.
func (n *SignedNode) RemoveTarget(pin string) {
	n.mu.Lock()
	delete(n.targets, pin)
	n.mu.Unlock()
}

// read answers the reads that keep today's results (Koinon contract, Answer shapes), and
// computers. Pages take after and limit.
func (n *SignedNode) read(pin string, t *Target, e signed.Envelope) (string, bool) {
	n.mu.Lock()
	defer n.mu.Unlock()
	t.Reads[e.Op]++
	if t.Fail[e.Op] > 0 {
		t.Fail[e.Op]--
		return refuse("internal", nil)
	}
	after, _ := e.Args["after"].(int64)
	limit, _ := e.Args["limit"].(int64)
	if limit <= 0 {
		limit = 100
	}
	switch e.Op {
	case "computers":
		if pin != n.Pin {
			return refuse("invalid_request", nil)
		}
		var list []map[string]any
		pins := make([]string, 0, len(n.targets))
		for p := range n.targets {
			if p != n.Pin {
				pins = append(pins, p)
			}
		}
		sort.Strings(pins)
		for _, p := range pins {
			o := n.targets[p]
			state := "connected"
			if o.Offline {
				state = "offline"
			}
			list = append(list, map[string]any{"pin": p, "name": o.Name, "state": state, "reason": o.Reason, "since": n.Now().Unix()})
		}
		return answer(map[string]any{"node": map[string]string{"pin": n.Pin, "name": t.Name}, "computers": list})
	case "board_list":
		refs := make([]string, 0, len(t.Items))
		for r, it := range t.Items {
			if it["state"] == "active" {
				refs = append(refs, r)
			}
		}
		sort.Strings(refs)
		var items []map[string]any
		var next any
		more := false
		for i := int(after); i < len(refs); i++ {
			if int64(len(items)) == limit {
				more, next = true, int64(i)
				break
			}
			it := t.Items[refs[i]]
			items = append(items, map[string]any{"reference": it["reference"], "title": it["title"], "kind": it["kind"],
				"state": it["state"], "revision": it["revision"], "order": i})
		}
		return answer(map[string]any{"counts": map[string]int{"waiting": len(refs)}, "items": items, "more": more, "next_after": next})
	case "inbox":
		var msgs []map[string]any
		more := false
		for _, m := range t.Inbox {
			if m["seq"].(int64) <= after {
				continue
			}
			if int64(len(msgs)) == limit {
				more = true
				break
			}
			msgs = append(msgs, m)
		}
		return answer(map[string]any{"messages": msgs, "acked_through": t.AckedThrough, "more": more})
	case "room_list":
		return answer(map[string]any{"rooms": t.Rooms})
	case "room_read":
		room := fmt.Sprint(e.Args["room"])
		posts, ok := t.Posts[room]
		if !ok {
			return refuse("room_not_found", nil)
		}
		var out []map[string]any
		more := false
		for _, p := range posts {
			if p["seq"].(int64) <= after {
				continue
			}
			if int64(len(out)) == limit {
				more = true
				break
			}
			out = append(out, p)
		}
		return answer(map[string]any{"room_id": room, "posts": out, "more": more})
	}
	return refuse("unknown_operation", nil)
}
