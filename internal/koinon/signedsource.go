package koinon

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// SourceOptions configure a signed source. Zero values take the defaults below.
type SourceOptions struct {
	// Host is a signed row of hosts.toml (Host.Signed).
	Host Host
	// Access is this install's key and authorizations; AccessErr is why it could not be
	// loaded. With AccessErr, or without an authorization for the node's pin, the source
	// opens nothing and shows the node disconnected with the reason.
	Access    Access
	AccessErr error
	// Open signs a new session with the node; nil uses StartSigned for the local form and
	// OpenSignedHTTP for a Serve row.
	Open func(ctx context.Context, opts SignedOptions) (*Signed, error)
	// Version is Koinon Voice's version for MCP initialize.
	Version string
	// RequestTimeout bounds each read (10 s).
	RequestTimeout time.Duration
	// MinDelay and MaxDelay bound the wait before a new session, and between tries of a
	// failed read (1 s to 60 s). The wait doubles after each failure.
	MinDelay, MaxDelay time.Duration
	// Watch sets the changes waits.
	Watch WatchOptions
	// OnChange receives each new list of states, in order.
	OnChange func([]State)
}

// SignedSource holds one signed node's session and shows the node and each computer paired
// with it, each keyed by its pin (R12). It reads a target's topics once, then only the topics
// that its changes wait names (R17, R21). Versions and the data read under them are kept
// together in memory across sessions; a new process reads everything once.
type SignedSource struct {
	opts SourceOptions

	mu       sync.Mutex
	targets  map[string]*target
	versions map[string]map[string]string
	attempts int
}

// target is what the source knows of one computer. Its slices are replaced, never changed
// in place, so a published State stays as it was.
type target struct {
	state  State
	paired bool
	// confirmed: the target answered a signed read in this process.
	confirmed  bool
	offline    bool
	items      map[string]Item
	inboxAfter int64
	inbox      []Message
	rooms      map[string]*roomCursor
	order      []string
}

// NewSignedSource returns a source for a signed row. Run starts it.
func NewSignedSource(opts SourceOptions) *SignedSource {
	if opts.RequestTimeout == 0 {
		opts.RequestTimeout = 10 * time.Second
	}
	if opts.MinDelay == 0 {
		opts.MinDelay = time.Second
	}
	if opts.MaxDelay == 0 {
		opts.MaxDelay = 60 * time.Second
	}
	s := &SignedSource{opts: opts, targets: map[string]*target{}, versions: map[string]map[string]string{}}
	n := s.target(opts.Host.NodePin)
	n.paired = true
	n.state.Host = opts.Host.Label
	n.state.Reason = "enrolled here, not yet confirmed by the node"
	return s
}

// Host returns the source's row.
func (s *SignedSource) Host() Host { return s.opts.Host }

// States returns the node first, then each computer by name and pin.
func (s *SignedSource) States() []State {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.statesLocked()
}

func (s *SignedSource) statesLocked() []State {
	node := s.opts.Host.NodePin
	pins := make([]string, 0, len(s.targets))
	for pin := range s.targets {
		if pin != node {
			pins = append(pins, pin)
		}
	}
	sort.Slice(pins, func(i, j int) bool {
		a, b := s.targets[pins[i]].state, s.targets[pins[j]].state
		if a.Host != b.Host {
			return a.Host < b.Host
		}
		return a.Pin < b.Pin
	})
	out := []State{s.targets[node].state}
	for _, pin := range pins {
		out = append(out, s.targets[pin].state)
	}
	return out
}

// target returns the pin's entry, making it; the caller holds mu or owns s.
func (s *SignedSource) target(pin string) *target {
	t, ok := s.targets[pin]
	if !ok {
		t = &target{state: State{Host: pin, Pin: pin, Node: s.opts.Host.Label, Status: Connecting, Since: time.Now()},
			items: map[string]Item{}, rooms: map[string]*roomCursor{}}
		s.targets[pin] = t
	}
	return t
}

// update changes the source under its lock and publishes the states.
func (s *SignedSource) update(f func()) {
	s.mu.Lock()
	f()
	states := s.statesLocked()
	s.mu.Unlock()
	if s.opts.OnChange != nil {
		s.opts.OnChange(states)
	}
}

// setStatus sets a target's status, code and reason, and its time when the status changes.
func setStatus(t *target, status Status, code, reason string) {
	if t.state.Status != status {
		t.state.Since = time.Now()
	}
	t.state.Status, t.state.Code, t.state.Reason = status, code, reason
}

// Run holds the node's session until ctx ends, signing a new one after each end.
func (s *SignedSource) Run(ctx context.Context) {
	node := s.opts.Host.NodePin
	refuse := func(code, reason string) {
		s.update(func() { setStatus(s.targets[node], Disconnected, code, reason) })
		<-ctx.Done()
	}
	if s.opts.AccessErr != nil {
		refuse("key_unavailable", s.opts.AccessErr.Error())
		return
	}
	if r := s.opts.Access.NodeReason(node); r != "" {
		refuse("not_enrolled", r)
		return
	}
	delay := s.opts.MinDelay
	for ctx.Err() == nil {
		if s.session(ctx) {
			delay = s.opts.MinDelay
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(delay):
		}
		delay = min(2*delay, s.opts.MaxDelay)
	}
}

func (s *SignedSource) open(ctx context.Context) (*Signed, error) {
	opts := SignedOptions{NodePin: s.opts.Host.NodePin, Client: ClientName, Key: s.opts.Access.Key,
		Auths: s.opts.Access.Auths, Version: s.opts.Version}
	if s.opts.Open != nil {
		return s.opts.Open(ctx, opts)
	}
	if s.opts.Host.Serve != "" {
		return OpenSignedHTTP(ctx, opts, s.opts.Host.Serve, nil)
	}
	opts.Koinon, opts.Args = s.opts.Host.SignedLocal()
	return StartSigned(ctx, opts)
}

// session runs one signed session; it reports whether the node answered a read.
func (s *SignedSource) session(ctx context.Context) bool {
	s.update(func() { s.attempts++; s.targets[s.opts.Host.NodePin].state.Attempts = s.attempts })
	sctx, cancel := context.WithCancel(ctx)
	defer cancel()
	sig, err := s.open(sctx)
	if err != nil {
		s.disconnectAll(refusalCode(err, "session_failed"), err.Error())
		return false
	}
	defer sig.Close()
	changes := make(chan Change)
	s.mu.Lock()
	versions := copyAll(s.versions)
	s.mu.Unlock()
	w := sig.Watch(sctx, versions, changes, s.opts.Watch)
	var wg sync.WaitGroup
	defer func() {
		cancel()
		w.Stop()
		wg.Wait()
		s.mu.Lock()
		s.versions = w.Versions()
		s.mu.Unlock()
	}()
	// Each session reads computers first: it confirms the node, and the computers that
	// answered before go on from their versions without a read of their own.
	if err := s.readComputers(sctx, sig, w); err != nil {
		s.disconnectAll(refusalCode(err, "read_failed"), err.Error())
		return false
	}
	w.Add(s.opts.Host.NodePin)
	var answered atomic.Bool
	answered.Store(true)
	for {
		select {
		case <-sctx.Done():
			return answered.Load()
		case <-sig.Done():
			s.disconnectAll("session_ended", "the session with the node ended")
			return answered.Load()
		case c := <-changes:
			switch {
			case c.SessionEnded:
				s.disconnectAll("session_ended", "the session with the node ended")
				return answered.Load()
			case c.Offline:
				s.update(func() {
					t := s.target(c.Pin)
					t.offline = true
					setStatus(t, Disconnected, "computer_offline", "the node has no link to this computer")
				})
			case c.Err != nil:
				s.update(func() { s.failed(c.Pin, c.Err) })
			default:
				// The watcher sends a target's next change only after Done, so one apply
				// runs per target at a time.
				wg.Add(1)
				go func() {
					defer wg.Done()
					if s.apply(sctx, sig, w, c) {
						answered.Store(true)
					}
				}()
			}
		}
	}
}

// failed shows a read or wait failure on a target without dropping its data.
func (s *SignedSource) failed(pin string, err error) {
	t := s.target(pin)
	t.state.Code, t.state.Reason = refusalCode(err, "read_failed"), err.Error()
}

// disconnectAll marks every target disconnected after the session ended; data stays.
func (s *SignedSource) disconnectAll(code, reason string) {
	s.update(func() {
		for _, t := range s.targets {
			if t.paired && t.state.Code != "not_enrolled" {
				setStatus(t, Disconnected, code, reason)
			}
		}
	})
}

// apply reads a change's topics, tries again after a failure, and lets the next wait start
// only when every topic was read (Watcher.Done).
func (s *SignedSource) apply(ctx context.Context, sig *Signed, w *Watcher, c Change) bool {
	delay := s.opts.MinDelay
	for {
		err := s.read(ctx, sig, w, c)
		if ctx.Err() != nil {
			return false
		}
		if err == nil {
			s.update(func() {
				t := s.target(c.Pin)
				if !t.offline && t.paired {
					t.confirmed = true
					setStatus(t, Connected, "", "")
				}
			})
			w.Done(c.Pin, c.Topics, c.Removed)
			return true
		}
		s.update(func() { s.failed(c.Pin, err) })
		select {
		case <-ctx.Done():
			return false
		case <-time.After(delay):
		}
		delay = min(2*delay, s.opts.MaxDelay)
	}
}

// read reads a change's topics for one target. Each topic's data is replaced only after it
// was read in full.
func (s *SignedSource) read(ctx context.Context, sig *Signed, w *Watcher, c Change) error {
	pin := c.Pin
	if _, ok := c.Topics["computers"]; ok && pin == s.opts.Host.NodePin {
		if err := s.readComputers(ctx, sig, w); err != nil {
			return err
		}
	}
	if len(c.Removed) > 0 {
		s.update(func() {
			t := s.target(pin)
			for _, topic := range c.Removed {
				switch topic {
				case "decisions":
					t.items, t.state.Items, t.state.Counts = map[string]Item{}, nil, Counts{}
				case "inbox":
					t.inbox, t.state.Inbox = nil, nil
				default:
					delete(t.rooms, topic)
				}
			}
			publishRooms(t)
		})
	}
	if _, ok := c.Topics["decisions"]; ok {
		if err := s.readBoard(ctx, sig, pin); err != nil {
			return err
		}
	}
	if _, ok := c.Topics["inbox"]; ok {
		if err := s.readInbox(ctx, sig, pin); err != nil {
			return err
		}
	}
	var rooms []string
	for topic := range c.Topics {
		if topic != "decisions" && topic != "inbox" && topic != "computers" {
			rooms = append(rooms, topic)
		}
	}
	if len(rooms) > 0 {
		if err := s.readRooms(ctx, sig, pin); err != nil {
			return err
		}
		sort.Strings(rooms)
		for _, id := range rooms {
			if err := s.readRoom(ctx, sig, pin, id); err != nil {
				return err
			}
		}
	}
	return nil
}

// call signs and sends one read with the request timeout.
func (s *SignedSource) call(ctx context.Context, sig *Signed, pin, op string, args map[string]any, out any) error {
	ctx, cancel := context.WithTimeout(ctx, s.opts.RequestTimeout)
	defer cancel()
	return sig.Read(ctx, pin, op, args, out)
}

// readComputers reads the node's computers and starts, pauses, resumes or stops each
// computer's wait. A computer is matched by pin only: a new pin with a known name is a new
// computer (R12).
func (s *SignedSource) readComputers(ctx context.Context, sig *Signed, w *Watcher) error {
	var a struct {
		Node      Computer   `json:"node"`
		Computers []Computer `json:"computers"`
	}
	node := s.opts.Host.NodePin
	if err := s.call(ctx, sig, node, "computers", nil, &a); err != nil {
		return err
	}
	if a.Node.Pin != node {
		return fmt.Errorf("%w: computers names the node %q, not the pinned %s", ErrProtocol, a.Node.Pin, node)
	}
	var add, resume, remove []string
	s.update(func() {
		n := s.targets[node]
		if a.Node.Name != "" {
			n.state.Host = a.Node.Name
		}
		n.confirmed = true
		setStatus(n, Connected, "", "")
		listed := map[string]bool{node: true}
		for _, cpt := range a.Computers {
			if cpt.Pin == node || listed[cpt.Pin] {
				continue
			}
			listed[cpt.Pin] = true
			t := s.target(cpt.Pin)
			if cpt.Name != "" {
				t.state.Host = cpt.Name
			}
			t.paired = true
			if r := s.opts.Access.NodeReason(cpt.Pin); r != "" {
				setStatus(t, Disconnected, "not_enrolled", r)
				continue
			}
			if cpt.State != "connected" {
				t.offline = true
				reason := "the node has no link to this computer"
				if cpt.Reason != "" {
					reason += ": " + cpt.Reason
				}
				setStatus(t, Disconnected, "computer_offline", reason)
				add = append(add, cpt.Pin)
				continue
			}
			if t.offline {
				t.offline = false
				resume = append(resume, cpt.Pin)
			}
			if t.confirmed {
				setStatus(t, Connected, "", "")
			} else {
				setStatus(t, Connecting, "", "enrolled here, not yet confirmed by this computer")
			}
			add = append(add, cpt.Pin)
		}
		for pin, t := range s.targets {
			if !listed[pin] && t.paired {
				t.paired = false
				setStatus(t, Disconnected, "not_paired", "the node no longer lists this computer")
				remove = append(remove, pin)
			}
		}
	})
	for _, pin := range remove {
		w.Remove(pin)
	}
	for _, pin := range add {
		w.Add(pin)
	}
	for _, pin := range resume {
		w.Resume(pin)
	}
	return nil
}

// readBoard reads every page of the active items, and each item whose revision changed.
func (s *SignedSource) readBoard(ctx context.Context, sig *Signed, pin string) error {
	var counts Counts
	var listed []Summary
	var after int64
	for first := true; ; first = false {
		args := map[string]any{"filter": "active", "limit": 100}
		if after > 0 {
			args["after"] = after
		}
		var p boardPage
		if err := s.call(ctx, sig, pin, "board_list", args, &p); err != nil {
			return err
		}
		if first {
			counts = p.Counts
		}
		listed = append(listed, p.Items...)
		if !p.More {
			break
		}
		if p.NextAfter == nil || *p.NextAfter <= after {
			return fmt.Errorf("%w: board_list answered more without a next page", ErrProtocol)
		}
		after = *p.NextAfter
	}
	s.mu.Lock()
	have := s.target(pin).items
	s.mu.Unlock()
	kept := map[string]Item{}
	items := make([]Item, 0, len(listed))
	for _, sum := range listed {
		item, ok := have[sum.Reference]
		if !ok || item.Revision != sum.Revision {
			item = Item{}
			err := s.call(ctx, sig, pin, "decision_get", map[string]any{"reference": sum.Reference}, &item)
			if Code(err) == "not_found" {
				continue // it left the board after the list
			}
			if err != nil {
				return err
			}
		}
		if item.State != "active" {
			continue
		}
		kept[item.Reference] = item
		items = append(items, item)
	}
	s.update(func() {
		t := s.target(pin)
		t.items, t.state.Items, t.state.Counts = kept, items, counts
	})
	return nil
}

// readInbox reads the messages after the last one read and keeps those not acknowledged.
func (s *SignedSource) readInbox(ctx context.Context, sig *Signed, pin string) error {
	s.mu.Lock()
	t := s.target(pin)
	after, msgs := t.inboxAfter, t.inbox
	s.mu.Unlock()
	msgs = append([]Message(nil), msgs...)
	var acked int64
	for {
		var p inboxPage
		if err := s.call(ctx, sig, pin, "inbox", map[string]any{"after": after, "limit": 100}, &p); err != nil {
			return err
		}
		acked = p.AckedThrough
		msgs = append(msgs, p.Messages...)
		if n := len(p.Messages); n > 0 {
			after = p.Messages[n-1].Seq
		}
		if !p.More || len(p.Messages) == 0 {
			break
		}
	}
	open := make([]Message, 0, len(msgs))
	for _, m := range msgs {
		if m.Seq > acked {
			open = append(open, m)
		}
	}
	s.update(func() {
		t := s.target(pin)
		t.inboxAfter, t.inbox, t.state.Inbox = after, open, open
	})
	return nil
}

// readRooms lists the rooms; a room no longer listed is dropped.
func (s *SignedSource) readRooms(ctx context.Context, sig *Signed, pin string) error {
	var listed []Room
	var after int64
	for {
		var p struct {
			Rooms     []Room `json:"rooms"`
			More      bool   `json:"more"`
			NextAfter *int64 `json:"next_after"`
		}
		args := map[string]any{"limit": 100}
		if after > 0 {
			args["after"] = after
		}
		if err := s.call(ctx, sig, pin, "room_list", args, &p); err != nil {
			return err
		}
		listed = append(listed, p.Rooms...)
		if !p.More {
			break
		}
		if p.NextAfter == nil || *p.NextAfter <= after {
			return fmt.Errorf("%w: room_list answered more without a next page", ErrProtocol)
		}
		after = *p.NextAfter
	}
	s.update(func() {
		t := s.target(pin)
		rooms := map[string]*roomCursor{}
		order := make([]string, 0, len(listed))
		for _, r := range listed {
			rc, ok := t.rooms[r.RoomID]
			if !ok {
				rc = &roomCursor{}
			}
			rc.room = r
			rooms[r.RoomID] = rc
			order = append(order, r.RoomID)
		}
		t.rooms, t.order = rooms, order
		publishRooms(t)
	})
	return nil
}

// readRoom reads a room's posts after the last one read. A room that is gone is dropped.
func (s *SignedSource) readRoom(ctx context.Context, sig *Signed, pin, id string) error {
	s.mu.Lock()
	rc, ok := s.target(pin).rooms[id]
	var after int64
	var posts []Post
	if ok {
		after, posts = rc.after, rc.posts
	}
	s.mu.Unlock()
	if !ok {
		return nil
	}
	posts = append([]Post(nil), posts...)
	for {
		var p roomPage
		err := s.call(ctx, sig, pin, "room_read", map[string]any{"room": id, "after": after, "limit": 100}, &p)
		if Code(err) == "room_not_found" {
			s.update(func() {
				t := s.target(pin)
				delete(t.rooms, id)
				publishRooms(t)
			})
			return nil
		}
		if err != nil {
			return err
		}
		posts = append(posts, p.Posts...)
		if n := len(p.Posts); n > 0 {
			after = p.Posts[n-1].Seq
		}
		if !p.More || len(p.Posts) == 0 {
			break
		}
	}
	s.update(func() {
		t := s.target(pin)
		if rc, ok := t.rooms[id]; ok {
			t.rooms[id] = &roomCursor{room: rc.room, after: after, posts: posts}
			publishRooms(t)
		}
	})
	return nil
}

// publishRooms sets a target's rooms in list order from its cursors.
func publishRooms(t *target) {
	rooms := make([]RoomState, 0, len(t.order))
	for _, id := range t.order {
		if rc, ok := t.rooms[id]; ok {
			rooms = append(rooms, RoomState{Room: rc.room, Posts: rc.posts})
		}
	}
	t.state.Rooms = rooms
}

// refusalCode is a refusal's code, or def.
func refusalCode(err error, def string) string {
	var r *Refusal
	if errors.As(err, &r) {
		return r.Code
	}
	return def
}

func copyAll(v map[string]map[string]string) map[string]map[string]string {
	c := make(map[string]map[string]string, len(v))
	for pin, m := range v {
		c[pin] = copyVersions(m)
	}
	return c
}
