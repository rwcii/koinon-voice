package koinon

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// URIs of the maintainer mode's resources.
const (
	InboxURI     = "koinon://maintainer/inbox"
	DecisionsURI = "koinon://maintainer/decisions"
	roomURI      = "koinon://maintainer/rooms/"
)

// Status is a host's connection state.
type Status string

const (
	Connecting   Status = "connecting"
	Connected    Status = "connected"
	Disconnected Status = "disconnected"
)

// RoomPosts is the number of a room's latest posts that a state keeps.
const RoomPosts = 50

// State is what a connector knows of its host. A connector replaces its slices and never
// changes them in place, so a State is safe to keep and read.
type State struct {
	Host   string
	Status Status
	// Code and Reason say why the host is disconnected; Code is Koinon's code when there
	// is one. Neither ever holds the secret.
	Code   string
	Reason string
	Since  time.Time
	// Attempts counts the starts of the maintainer mode.
	Attempts int
	Counts   Counts
	// Items are the board's active items, decision and fyi, in board order, each read in
	// full.
	Items []Item
	// Inbox holds the maintainer's messages that are not acknowledged.
	Inbox []Message
	Rooms []RoomState
}

// RoomState is a room with its latest posts.
type RoomState struct {
	Room
	Posts []Post
}

// Secrets gives a host's maintainer secret by its Keychain name.
type Secrets interface {
	Get(ctx context.Context, name string) (string, error)
}

// Options configure a connector. Zero values take the defaults below.
type Options struct {
	Secrets Secrets
	// SSH is the ssh binary; empty is "ssh" on PATH.
	SSH string
	// Version is sent as the client version in initialize.
	Version string
	// RequestTimeout bounds each request (10 s).
	RequestTimeout time.Duration
	// Liveness is the idle time after which the connector makes one read (30 s), so a
	// daemon restart is seen when no exit comes.
	Liveness time.Duration
	// MinDelay and MaxDelay bound the wait before a new start (1 s to 60 s). The wait
	// doubles after each start that did not connect.
	MinDelay, MaxDelay time.Duration
	// Grace is how long a process has to exit after its input closes (2 s).
	Grace time.Duration
	// OnChange receives each new state, in order, on the connector's goroutine.
	OnChange func(State)
}

// ErrDisconnected is an action on a host that has no connection.
var ErrDisconnected = errors.New("the host is not connected")

// Down is why a session of the maintainer mode ended.
type Down struct {
	Code   string
	Reason string
	// connected is true when the session had connected before it ended.
	connected bool
	// retryAfter is Koinon's wait after wrong secrets, when it gave one.
	retryAfter time.Duration
}

func (d *Down) Error() string {
	if d.Code == "" || strings.HasPrefix(d.Reason, d.Code) {
		return d.Reason
	}
	return d.Code + ": " + d.Reason
}

// Connector holds one host's connection through the maintainer mode.
type Connector struct {
	host Host
	opts Options

	mu     sync.Mutex
	state  State
	client *Client
	// refresh is signalled after a verdict, so the board is read again at once.
	refresh chan struct{}
}

// New returns a connector for host. Run starts it.
func New(host Host, opts Options) *Connector {
	if opts.RequestTimeout == 0 {
		opts.RequestTimeout = 10 * time.Second
	}
	if opts.Liveness == 0 {
		opts.Liveness = 30 * time.Second
	}
	if opts.MinDelay == 0 {
		opts.MinDelay = time.Second
	}
	if opts.MaxDelay == 0 {
		opts.MaxDelay = 60 * time.Second
	}
	if opts.Grace == 0 {
		opts.Grace = 2 * time.Second
	}
	return &Connector{
		host:    host,
		opts:    opts,
		state:   State{Host: host.Label, Status: Connecting, Since: time.Now()},
		refresh: make(chan struct{}, 1),
	}
}

// Host returns the connector's host.
func (c *Connector) Host() Host { return c.host }

// State returns the current state.
func (c *Connector) State() State {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.state
}

// Run holds the connection until ctx ends, starting the maintainer mode again after each
// end, with a wait that grows while starts fail.
func (c *Connector) Run(ctx context.Context) {
	delay := c.opts.MinDelay
	for ctx.Err() == nil {
		d := c.session(ctx)
		if ctx.Err() != nil {
			return
		}
		c.update(func(s *State) {
			*s = State{Host: s.Host, Status: Disconnected, Code: d.Code, Reason: d.Reason, Since: time.Now(), Attempts: s.Attempts}
		})
		wait := delay
		switch {
		case d.connected:
			wait, delay = c.opts.MinDelay, c.opts.MinDelay
		case d.Code == "maintainer_secret_invalid" || d.Code == "maintainer_secret_unset":
			wait = c.opts.MaxDelay
		default:
			delay = min(delay*2, c.opts.MaxDelay)
		}
		if d.retryAfter > wait {
			wait = d.retryAfter
		}
		t := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			t.Stop()
			return
		case <-t.C:
		}
	}
}

// Verdict gives the maintainer's verdict on an item at the revision the maintainer saw.
// Koinon records it with the client mcp:koinon-voice. A refusal, such as stale_revision or
// not_found, is a *Refusal.
func (c *Connector) Verdict(ctx context.Context, reference string, revision int64, verdict, note string) (Verdict, error) {
	c.mu.Lock()
	client := c.client
	c.mu.Unlock()
	if client == nil {
		return Verdict{}, ErrDisconnected
	}
	args := map[string]any{"reference": reference, "revision": revision, "verdict": verdict}
	if note != "" {
		args["note"] = note
	}
	ctx, cancel := context.WithTimeout(ctx, c.opts.RequestTimeout)
	defer cancel()
	var v Verdict
	err := client.Tool(ctx, "decision_verdict", args, &v)
	select {
	case c.refresh <- struct{}{}:
	default:
	}
	return v, err
}

func (c *Connector) update(f func(*State)) {
	c.mu.Lock()
	f(&c.state)
	s := c.state
	c.mu.Unlock()
	if c.opts.OnChange != nil {
		c.opts.OnChange(s)
	}
}

// session runs one maintainer-mode process until it ends, and says why.
func (c *Connector) session(ctx context.Context) *Down {
	c.update(func(s *State) { s.Status, s.Attempts = Connecting, s.Attempts+1 })
	if c.opts.Secrets == nil {
		return &Down{Code: "no_secret_store", Reason: "no secret store"}
	}
	secret, err := c.opts.Secrets.Get(ctx, c.host.Secret)
	if err != nil {
		return &Down{Code: "secret_unavailable", Reason: err.Error()}
	}
	name, args := c.host.Command(c.opts.SSH)
	cmd := exec.Command(name, args...)
	setGroup(cmd)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return &Down{Reason: err.Error()}
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return &Down{Reason: err.Error()}
	}
	stderr := &tail{max: 4096}
	cmd.Stderr = stderr
	if err := cmd.Start(); err != nil {
		return &Down{Code: "start_failed", Reason: redact(err.Error(), secret)}
	}
	exited := make(chan struct{})
	go func() {
		_ = cmd.Wait()
		close(exited)
	}()
	defer func() {
		stdin.Close()
		select {
		case <-exited:
		case <-time.After(c.opts.Grace):
		}
		killGroup(cmd)
		<-exited
	}()

	// The secret is the first line of standard input, never an argument.
	if _, err := io.WriteString(stdin, secret+"\n"); err != nil {
		return c.ended(nil, exited, stderr, secret, err, false)
	}
	client := NewClient(stdout, stdin)
	s := &session{c: c, client: client}
	connected, err := s.start(ctx)
	if err == nil {
		c.mu.Lock()
		c.client = client
		c.mu.Unlock()
		err = s.loop(ctx)
	}
	c.mu.Lock()
	c.client = nil
	c.mu.Unlock()
	if ctx.Err() != nil {
		return &Down{Reason: "stopped"}
	}
	return c.ended(client, exited, stderr, secret, err, connected)
}

// ended describes the end of a session from the error and what the process wrote to
// standard error, with the secret removed from any text.
func (c *Connector) ended(client *Client, exited chan struct{}, stderr *tail, secret string, err error, connected bool) *Down {
	if client != nil && errors.Is(err, ErrClosed) {
		// Give the process a moment to finish its standard error.
		select {
		case <-exited:
		case <-time.After(c.opts.Grace):
		}
	}
	d := &Down{connected: connected, Code: Code(err)}
	if err != nil {
		d.Reason = err.Error()
	}
	if code, detail, retry, ok := stderr.koinonError(); ok && (d.Code == "" || errors.Is(err, ErrClosed)) {
		d.Code, d.Reason, d.retryAfter = code, detail, retry
	} else if line := stderr.lastLine(); line != "" && d.Code == "" {
		d.Reason = line
	}
	d.Reason = redact(d.Reason, secret)
	if d.Reason == "" {
		d.Reason = "the maintainer mode ended"
	}
	return d
}

func redact(s, secret string) string {
	if secret == "" {
		return s
	}
	return strings.ReplaceAll(s, secret, "[secret]")
}

// session is one connected process: its client and what it has read.
type session struct {
	c          *Connector
	client     *Client
	items      map[string]Item
	inbox      []Message
	inboxAfter int64
	rooms      map[string]*roomCursor
	order      []string
}

type roomCursor struct {
	room  Room
	after int64
	posts []Post
}

func (s *session) call(ctx context.Context, f func(context.Context) error) error {
	ctx, cancel := context.WithTimeout(ctx, s.c.opts.RequestTimeout)
	defer cancel()
	return f(ctx)
}

// start initializes, subscribes and reads everything. It reports whether it connected.
func (s *session) start(ctx context.Context) (bool, error) {
	s.items = map[string]Item{}
	s.rooms = map[string]*roomCursor{}
	if err := s.call(ctx, func(ctx context.Context) error { return s.client.Initialize(ctx, s.c.opts.Version) }); err != nil {
		return false, err
	}
	for _, uri := range []string{InboxURI, DecisionsURI} {
		if err := s.call(ctx, func(ctx context.Context) error { return s.client.Subscribe(ctx, uri) }); err != nil {
			return false, err
		}
	}
	if err := s.readRooms(ctx); err != nil {
		return false, err
	}
	if err := s.readBoard(ctx); err != nil {
		return false, err
	}
	if err := s.readInbox(ctx); err != nil {
		return false, err
	}
	for _, id := range s.order {
		if err := s.readRoom(ctx, id); err != nil {
			return false, err
		}
	}
	// Notices from the subscriptions above are covered by these reads.
	s.client.TakeNotices()
	s.publish(true)
	return true, nil
}

// loop follows notices until the process ends or a request fails.
func (s *session) loop(ctx context.Context) error {
	idle := time.NewTimer(s.c.opts.Liveness)
	defer idle.Stop()
	for {
		var err error
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-s.client.Done():
			return s.client.Err()
		case <-s.client.Notice():
			err = s.follow(ctx, s.client.TakeNotices())
		case <-s.c.refresh:
			err = s.follow(ctx, []string{DecisionsURI})
		case <-idle.C:
			// The liveness read; it also finds rooms created since the last read.
			err = s.readRooms(ctx)
			if err == nil {
				s.publish(false)
			}
		}
		if err != nil {
			return err
		}
		idle.Reset(s.c.opts.Liveness)
	}
}

func (s *session) follow(ctx context.Context, uris []string) error {
	for _, uri := range uris {
		var err error
		switch {
		case uri == DecisionsURI:
			err = s.readBoard(ctx)
		case uri == InboxURI:
			err = s.readInbox(ctx)
		case strings.HasPrefix(uri, roomURI):
			err = s.readRoom(ctx, strings.TrimPrefix(uri, roomURI))
		}
		if err != nil {
			return err
		}
	}
	s.publish(false)
	return nil
}

// readBoard reads every page of the active items and each item whose revision changed.
func (s *session) readBoard(ctx context.Context) error {
	var counts Counts
	var listed []Summary
	var after *int64
	for {
		args := map[string]any{"filter": "active", "limit": 100}
		if after != nil {
			args["after"] = *after
		}
		var p boardPage
		if err := s.call(ctx, func(ctx context.Context) error { return s.client.Tool(ctx, "board_list", args, &p) }); err != nil {
			return err
		}
		if after == nil {
			counts = p.Counts
		}
		listed = append(listed, p.Items...)
		if !p.More || p.NextAfter == nil {
			break
		}
		after = p.NextAfter
	}
	items := make([]Item, 0, len(listed))
	kept := map[string]Item{}
	for _, sum := range listed {
		item, ok := s.items[sum.Reference]
		if !ok || item.Revision != sum.Revision {
			err := s.call(ctx, func(ctx context.Context) error {
				return s.client.Tool(ctx, "decision_get", map[string]any{"reference": sum.Reference}, &item)
			})
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
	s.items = kept
	s.c.mu.Lock()
	s.c.state.Counts, s.c.state.Items = counts, items
	s.c.mu.Unlock()
	return nil
}

// readInbox reads the messages after the last one read, and keeps those not acknowledged.
func (s *session) readInbox(ctx context.Context) error {
	msgs := s.inbox
	var acked int64
	for {
		var p inboxPage
		args := map[string]any{"after": s.inboxAfter, "limit": 100}
		if err := s.call(ctx, func(ctx context.Context) error { return s.client.Tool(ctx, "inbox", args, &p) }); err != nil {
			return err
		}
		acked = p.AckedThrough
		msgs = append(msgs, p.Messages...)
		if n := len(p.Messages); n > 0 {
			s.inboxAfter = p.Messages[n-1].Seq
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
	s.inbox = open
	s.c.mu.Lock()
	s.c.state.Inbox = open
	s.c.mu.Unlock()
	return nil
}

// readRooms lists the rooms and subscribes to each new one.
func (s *session) readRooms(ctx context.Context) error {
	var r struct {
		Rooms []Room `json:"rooms"`
	}
	if err := s.call(ctx, func(ctx context.Context) error { return s.client.Tool(ctx, "room_list", map[string]any{}, &r) }); err != nil {
		return err
	}
	present := map[string]bool{}
	var order []string
	for _, room := range r.Rooms {
		present[room.RoomID] = true
		order = append(order, room.RoomID)
		if rc, ok := s.rooms[room.RoomID]; ok {
			rc.room = room
			continue
		}
		err := s.call(ctx, func(ctx context.Context) error { return s.client.Subscribe(ctx, roomURI+room.RoomID) })
		if err != nil {
			return err
		}
		// The subscription's own notice reads the room's posts.
		s.rooms[room.RoomID] = &roomCursor{room: room}
	}
	for id := range s.rooms {
		if !present[id] {
			delete(s.rooms, id)
		}
	}
	s.order = order
	return nil
}

// readRoom reads a room's posts after the last one read. A room that is gone is dropped.
func (s *session) readRoom(ctx context.Context, id string) error {
	rc, ok := s.rooms[id]
	if !ok {
		return nil
	}
	for {
		var p roomPage
		args := map[string]any{"room_id": id, "after": rc.after, "limit": 100}
		err := s.call(ctx, func(ctx context.Context) error { return s.client.Tool(ctx, "room_read", args, &p) })
		if Code(err) == "room_not_found" {
			delete(s.rooms, id)
			return nil
		}
		if err != nil {
			return err
		}
		posts := append(append([]Post(nil), rc.posts...), p.Posts...)
		if len(posts) > RoomPosts {
			posts = posts[len(posts)-RoomPosts:]
		}
		rc.posts = posts
		if n := len(p.Posts); n > 0 {
			rc.after = p.Posts[n-1].Seq
		}
		if !p.More || len(p.Posts) == 0 {
			return nil
		}
	}
}

// publish sets the connected state from what the session has read.
func (s *session) publish(first bool) {
	rooms := make([]RoomState, 0, len(s.order))
	for _, id := range s.order {
		if rc, ok := s.rooms[id]; ok {
			rooms = append(rooms, RoomState{Room: rc.room, Posts: rc.posts})
		}
	}
	s.c.update(func(st *State) {
		if first || st.Status != Connected {
			st.Since = time.Now()
		}
		st.Status, st.Code, st.Reason, st.Rooms = Connected, "", "", rooms
	})
}

// tail keeps the last bytes a process wrote to standard error.
type tail struct {
	mu  sync.Mutex
	max int
	buf []byte
}

func (t *tail) Write(p []byte) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.buf = append(t.buf, p...)
	if len(t.buf) > t.max {
		t.buf = t.buf[len(t.buf)-t.max:]
	}
	return len(p), nil
}

func (t *tail) lastLine() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	lines := strings.Split(strings.TrimSpace(string(t.buf)), "\n")
	line := strings.TrimSpace(lines[len(lines)-1])
	if len(line) > 300 {
		line = line[:300]
	}
	return line
}

// koinonError reads Koinon's last JSON error line: {"code": ..., "detail": ...}.
func (t *tail) koinonError() (code, detail string, retry time.Duration, ok bool) {
	t.mu.Lock()
	lines := bytes.Split(bytes.TrimSpace(t.buf), []byte("\n"))
	t.mu.Unlock()
	for i := len(lines) - 1; i >= 0; i-- {
		var e struct {
			Code    string `json:"code"`
			Detail  string `json:"detail"`
			Details struct {
				RetryAfterSeconds float64 `json:"retry_after_seconds"`
			} `json:"details"`
		}
		if json.Unmarshal(lines[i], &e) == nil && e.Code != "" {
			// Koinon wraps the daemon's code in daemon_error with the code in the detail.
			if e.Code == "daemon_error" {
				for _, c := range []string{"maintainer_secret_invalid", "maintainer_secret_unset", "maintainer_secret_backoff", "maintainer_token_expired"} {
					if strings.Contains(e.Detail, c) {
						e.Code = c
					}
				}
			}
			return e.Code, e.Detail, time.Duration(e.Details.RetryAfterSeconds * float64(time.Second)), true
		}
	}
	return "", "", 0, false
}
