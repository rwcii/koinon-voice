package koinon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/rwcii/koinon-voice/internal/koinon/koinontest"
)

func TestMain(m *testing.M) {
	koinontest.Main()
	os.Exit(m.Run())
}

// secret is a sentinel: no argument, error, state or output may hold it.
const secret = "synthetic-maintainer-secret-4b1d"

type secrets map[string]string

func (s secrets) Get(ctx context.Context, name string) (string, error) {
	if v, ok := s[name]; ok {
		return v, nil
	}
	return "", fmt.Errorf("Keychain item koinon-voice-%s not found (security exit 44)", name)
}

// fastOptions keep the waits short.
func fastOptions() Options {
	return Options{
		Secrets:        secrets{"maintainer-test": secret},
		RequestTimeout: 2 * time.Second,
		Liveness:       time.Hour,
		MinDelay:       20 * time.Millisecond,
		MaxDelay:       200 * time.Millisecond,
		Grace:          300 * time.Millisecond,
	}
}

func newHost(t *testing.T, label string) (Host, koinontest.Control) {
	t.Helper()
	c := koinontest.NewControl(t.TempDir(), secret)
	path := koinontest.WriteKoinon(filepath.Join(c.Dir, "koinon"), c)
	return Host{Label: label, Koinon: path, Secret: "maintainer-test"}, c
}

// recorder keeps every state a connector published.
type recorder struct {
	mu     sync.Mutex
	states []State
}

func (r *recorder) add(s State) {
	r.mu.Lock()
	r.states = append(r.states, s)
	r.mu.Unlock()
}

func (r *recorder) saw(f func(State) bool) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, s := range r.states {
		if f(s) {
			return true
		}
	}
	return false
}

// start runs a connector until the test ends, and checks then that no state held the
// secret.
func start(t *testing.T, h Host, opts Options) (*Connector, *recorder) {
	t.Helper()
	rec := &recorder{}
	opts.OnChange = rec.add
	conn := New(h, opts)
	ctx, cancel := context.WithCancel(context.Background())
	stopped := make(chan struct{})
	go func() {
		conn.Run(ctx)
		close(stopped)
	}()
	t.Cleanup(func() {
		cancel()
		select {
		case <-stopped:
		case <-time.After(5 * time.Second):
			t.Error("Run did not return")
		}
		rec.mu.Lock()
		raw, _ := json.Marshal(rec.states)
		rec.mu.Unlock()
		if strings.Contains(string(raw), secret) {
			t.Error("a state holds the secret")
		}
	})
	return conn, rec
}

func waitFor(t *testing.T, conn *Connector, what string, f func(State) bool) State {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if s := conn.State(); f(s) {
			return s
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("no state with %s; last %+v", what, conn.State())
	return State{}
}

func connected(s State) bool { return s.Status == Connected }

func noSecretInStarts(t *testing.T, c koinontest.Control) {
	t.Helper()
	if strings.Contains(c.Read("starts"), secret) {
		t.Error("the secret is in the process arguments")
	}
}

func message(seq int64, body string) map[string]any {
	return map[string]any{"id": seq, "seq": seq, "sender_family": "codex", "sender_name": "codex-synthetic-a4", "body": body, "created_at": 1793125072521}
}

func post(seq int64, body string) map[string]any {
	return map[string]any{"seq": seq, "sender_family": "codex", "sender_name": "codex-synthetic-a4", "origin": "agent", "body": body, "created_at": 1793125063845}
}

func TestConnectorReadsEverything(t *testing.T) {
	h, c := newHost(t, "local")
	c.Write("page", "2")
	board := koinontest.Board{
		Items: []map[string]any{
			koinontest.Item("p-1", "decision", 1, "Item 1", "ctx 1", "rec 1"),
			koinontest.Item("p-2", "decision", 1, "Item 2", "ctx 2", "rec 2"),
			koinontest.Item("p-3", "decision", 1, "Item 3", "ctx 3", "rec 3"),
			koinontest.Item("p-4", "decision", 1, "Item 4", "ctx 4", "rec 4"),
			koinontest.Item("f-1", "fyi", 1, "An fyi", "fyi ctx", ""),
		},
		Inbox: []map[string]any{message(1, "first"), message(2, "second"), message(3, "third")},
		Acked: 1,
		Rooms: []koinontest.FakeRoom{{RoomID: "r1", Name: "probe-room", Posts: []map[string]any{post(1, "a"), post(2, "b"), post(3, "c")}}},
	}
	c.SetBoard(board)
	conn, _ := start(t, h, fastOptions())
	s := waitFor(t, conn, "five items", func(s State) bool { return connected(s) && len(s.Items) == 5 })
	if s.Items[2].Context != "ctx 3" || s.Items[2].Recommendation != "rec 3" || s.Items[4].Kind != "fyi" {
		t.Errorf("items not read in full: %+v", s.Items)
	}
	if s.Counts.Waiting != 4 || s.Counts.OpenFYI != 1 {
		t.Errorf("counts %+v", s.Counts)
	}
	if len(s.Inbox) != 2 || s.Inbox[0].Body != "second" {
		t.Errorf("inbox %+v", s.Inbox)
	}
	if len(s.Rooms) != 1 || len(s.Rooms[0].Posts) != 3 || s.Rooms[0].Name != "probe-room" {
		t.Errorf("rooms %+v", s.Rooms)
	}

	// An edit that changes only the context shows after its notice.
	board.Items[2] = koinontest.Item("p-3", "decision", 2, "Item 3", "ctx 3 edited", "rec 3")
	c.SetBoard(board)
	waitFor(t, conn, "the edited context", func(s State) bool {
		return len(s.Items) == 5 && s.Items[2].Context == "ctx 3 edited" && s.Items[2].Revision == 2
	})
	// A new item, a message, an acknowledgement and a post.
	board.Items = append(board.Items, koinontest.Item("p-5", "decision", 1, "Item 5", "ctx 5", ""))
	board.Inbox = append(board.Inbox, message(4, "fourth"))
	board.Acked = 2
	board.Rooms[0].Posts = append(board.Rooms[0].Posts, post(4, "d"))
	c.SetBoard(board)
	waitFor(t, conn, "the new data", func(s State) bool {
		return len(s.Items) == 6 && len(s.Inbox) == 2 && s.Inbox[1].Body == "fourth" && len(s.Rooms) == 1 && len(s.Rooms[0].Posts) == 4
	})
	if c.Starts() != 1 {
		t.Errorf("%d starts", c.Starts())
	}
	noSecretInStarts(t, c)
}

func TestConnectorVerdicts(t *testing.T) {
	h, c := newHost(t, "local")
	c.SetBoard(koinontest.Board{Items: []map[string]any{
		koinontest.Item("p-1", "decision", 1, "Item 1", "ctx", ""),
		koinontest.Item("p-2", "decision", 3, "Item 2", "ctx", ""),
		koinontest.Item("f-1", "fyi", 1, "An fyi", "ctx", ""),
	}})
	conn, _ := start(t, h, fastOptions())
	waitFor(t, conn, "three items", func(s State) bool { return connected(s) && len(s.Items) == 3 })
	ctx := context.Background()

	v, err := conn.Verdict(ctx, "p-1", 1, "approved", "from a test")
	if err != nil || v.Via != "mcp:koinon-voice" || v.Note != "from a test" {
		t.Fatalf("verdict %+v %v", v, err)
	}
	waitFor(t, conn, "p-1 gone", func(s State) bool { return len(s.Items) == 2 })

	_, err = conn.Verdict(ctx, "p-2", 2, "declined", "")
	var r *Refusal
	if !errors.As(err, &r) || r.Code != "stale_revision" {
		t.Fatalf("stale: %v", err)
	}
	if rev, ok := r.CurrentRevision(); !ok || rev != 3 {
		t.Errorf("current revision %d", rev)
	}
	if _, err := conn.Verdict(ctx, "p-9", 1, "approved", ""); Code(err) != "not_found" {
		t.Errorf("not found: %v", err)
	}
	if _, err := conn.Verdict(ctx, "f-1", 1, "approved", ""); Code(err) != "invalid_verdict" {
		t.Errorf("invalid: %v", err)
	}
	if v, err := conn.Verdict(ctx, "f-1", 1, "dismissed", ""); err != nil || v.Verdict != "dismissed" {
		t.Errorf("dismiss %+v %v", v, err)
	}
	if n := strings.Count(c.Read("verdicts.jsonl"), "\n"); n != 2 {
		t.Errorf("%d verdicts recorded", n)
	}
}

func TestConnectorRestartWhileSubscribed(t *testing.T) {
	h, c := newHost(t, "local")
	c.SetBoard(koinontest.Board{Items: []map[string]any{koinontest.Item("p-1", "decision", 1, "Item 1", "ctx", "")}})
	conn, rec := start(t, h, fastOptions())
	waitFor(t, conn, "connected", connected)
	c.Restart()
	waitFor(t, conn, "a second connection", func(s State) bool { return connected(s) && s.Attempts == 2 && len(s.Items) == 1 })
	if !rec.saw(func(s State) bool { return s.Status == Disconnected && s.Code == "maintainer_token_expired" }) {
		t.Error("the restart was not shown as disconnected with maintainer_token_expired")
	}
	if c.Starts() != 2 {
		t.Errorf("%d starts", c.Starts())
	}
}

func TestConnectorTokenEndsOnRequest(t *testing.T) {
	h, c := newHost(t, "local")
	c.Write("poll-blind", "1") // the fake does not exit by itself
	opts := fastOptions()
	opts.Liveness = 100 * time.Millisecond
	conn, rec := start(t, h, opts)
	waitFor(t, conn, "connected", connected)
	c.Restart()
	waitFor(t, conn, "a second connection", func(s State) bool { return connected(s) && s.Attempts == 2 })
	if !rec.saw(func(s State) bool { return s.Status == Disconnected && s.Code == "maintainer_token_expired" }) {
		t.Error("the liveness read did not see maintainer_token_expired")
	}
}

func TestConnectorStartsAgain(t *testing.T) {
	for _, tc := range []struct {
		file, code string
	}{
		{"subscribe-unavailable", "daemon_unavailable"},
		{"junk", ""},
		{"hang", ""},
		{"down", "daemon_error"},
	} {
		t.Run(tc.file, func(t *testing.T) {
			h, c := newHost(t, "local")
			opts := fastOptions()
			opts.RequestTimeout = time.Second
			opts.Liveness = 100 * time.Millisecond
			conn, rec := start(t, h, opts)
			n := waitFor(t, conn, "connected", connected).Attempts
			c.Write(tc.file, "1")
			c.Restart() // the subscribe case needs a new start to subscribe again
			if tc.file == "junk" || tc.file == "hang" {
				c.Write("poll-blind", "1")
			}
			// The start after the restart fails.
			waitFor(t, conn, "a failed start", func(s State) bool { return s.Status == Disconnected && s.Attempts > n+1 })
			if tc.code != "" && !rec.saw(func(s State) bool { return s.Status == Disconnected && s.Code == tc.code }) {
				t.Errorf("no disconnected state with %s", tc.code)
			}
			c.Remove(tc.file)
			waitFor(t, conn, "connected again", connected)
		})
	}
}

func TestConnectorRefusedSecret(t *testing.T) {
	h, c := newHost(t, "local")
	opts := fastOptions()
	wrong := "wrong-secret-sentinel-91c2"
	opts.Secrets = secrets{"maintainer-test": wrong}
	opts.MaxDelay = time.Hour
	conn, _ := start(t, h, opts)
	s := waitFor(t, conn, "refused", func(s State) bool { return s.Status == Disconnected })
	if s.Code != "maintainer_secret_invalid" || strings.Contains(s.Reason, wrong) {
		t.Errorf("state %+v", s)
	}
	time.Sleep(300 * time.Millisecond)
	if n := c.Starts(); n != 1 {
		t.Errorf("%d starts during the longest wait", n)
	}
}

func TestConnectorCannotStart(t *testing.T) {
	h, _ := newHost(t, "local")
	h.Koinon = filepath.Join(t.TempDir(), "no-koinon")
	conn, _ := start(t, h, fastOptions())
	s := waitFor(t, conn, "disconnected", func(s State) bool { return s.Status == Disconnected })
	if s.Code != "start_failed" {
		t.Errorf("missing koinon: %+v", s)
	}

	h2, _ := newHost(t, "other")
	h2.Secret = "maintainer-absent"
	conn2, _ := start(t, h2, fastOptions())
	s = waitFor(t, conn2, "disconnected", func(s State) bool { return s.Status == Disconnected })
	if s.Code != "secret_unavailable" || !strings.Contains(s.Reason, "koinon-voice-maintainer-absent") {
		t.Errorf("missing secret: %+v", s)
	}
	if _, err := conn2.Verdict(context.Background(), "p-1", 1, "approved", ""); !errors.Is(err, ErrDisconnected) {
		t.Errorf("verdict while disconnected: %v", err)
	}
}

func TestConnectorSSH(t *testing.T) {
	h, c := newHost(t, "devbox")
	// A remote path with a space and a quote, and a state directory with a space.
	odd := filepath.Join(c.Dir, "a b'c")
	if err := os.Mkdir(odd, 0o700); err != nil {
		t.Fatal(err)
	}
	h.Koinon = koinontest.WriteKoinon(filepath.Join(odd, "koinon"), c)
	h.SSH = "devbox.example"
	h.StateDir = "/srv/koinon state"
	opts := fastOptions()
	opts.SSH = koinontest.WriteSSH(filepath.Join(c.Dir, "ssh"), c)
	conn, _ := start(t, h, opts)
	waitFor(t, conn, "connected", connected)

	var args []string
	if err := json.Unmarshal([]byte(c.Read("ssh-args")), &args); err != nil {
		t.Fatal(err)
	}
	want := []string{"-o", "BatchMode=yes", "-T", "--", "devbox.example", shellQuote(h.Koinon), "'mcp'"}
	if len(args) < len(want) || strings.Join(args[:len(want)], "|") != strings.Join(want, "|") {
		t.Errorf("ssh arguments %q", args)
	}
	if strings.Contains(c.Read("ssh-args"), secret) {
		t.Error("the secret is in the ssh arguments")
	}
	var remote []string
	_ = json.Unmarshal([]byte(strings.SplitN(c.Read("starts"), "\n", 2)[0]), &remote)
	if strings.Join(remote, "|") != "mcp|--maintainer|--client|koinon-voice|--secret-stdin|--state-dir|/srv/koinon state" {
		t.Errorf("remote arguments %q", remote)
	}
	noSecretInStarts(t, c)
}

func TestConnectorSSHUnreachable(t *testing.T) {
	h, c := newHost(t, "devbox")
	h.SSH, h.Koinon = "unreachable", "/usr/local/bin/koinon"
	opts := fastOptions()
	opts.SSH = koinontest.WriteSSH(filepath.Join(c.Dir, "ssh"), c)
	conn, _ := start(t, h, opts)
	s := waitFor(t, conn, "disconnected", func(s State) bool { return s.Status == Disconnected })
	if !strings.Contains(s.Reason, "Connection refused") {
		t.Errorf("state %+v", s)
	}
}

func TestConnectorsKeepTheirHosts(t *testing.T) {
	ha, ca := newHost(t, "a")
	hb, cb := newHost(t, "b")
	for _, c := range []koinontest.Control{ca, cb} {
		c.SetBoard(koinontest.Board{Items: []map[string]any{koinontest.Item("p-1", "decision", 1, "Same reference", "ctx", "")}})
	}
	a, _ := start(t, ha, fastOptions())
	b, _ := start(t, hb, fastOptions())
	waitFor(t, a, "a connected", func(s State) bool { return connected(s) && len(s.Items) == 1 })
	waitFor(t, b, "b connected", func(s State) bool { return connected(s) && len(s.Items) == 1 })
	if _, err := b.Verdict(context.Background(), "p-1", 1, "approved", ""); err != nil {
		t.Fatal(err)
	}
	if ca.Read("verdicts.jsonl") != "" || cb.Read("verdicts.jsonl") == "" {
		t.Error("the verdict reached the wrong host")
	}
	waitFor(t, b, "b empty", func(s State) bool { return len(s.Items) == 0 })
	if len(a.State().Items) != 1 {
		t.Error("host a lost its item")
	}
}

func TestConnectorKillsAStubbornProcess(t *testing.T) {
	h, c := newHost(t, "local")
	c.Write("stubborn", "1")
	rec := &recorder{}
	opts := fastOptions()
	opts.OnChange = rec.add
	conn := New(h, opts)
	ctx, cancel := context.WithCancel(context.Background())
	stopped := make(chan struct{})
	go func() {
		conn.Run(ctx)
		close(stopped)
	}()
	waitFor(t, conn, "connected", connected)
	pid, err := strconv.Atoi(c.Read("pid"))
	if err != nil {
		t.Fatal(err)
	}
	cancel()
	select {
	case <-stopped:
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return")
	}
	if err := syscall.Kill(pid, 0); err == nil {
		t.Errorf("process %d is still running", pid)
	}
}

// A notice that comes while start reads stays for the loop (review of 922e3f7, F2): here
// the board changes after its read, while the inbox is read.
func TestStartKeepsNoticesDuringReads(t *testing.T) {
	client, srv := newPipeClient(t)
	s := &session{c: New(Host{Label: "test"}, Options{RequestTimeout: 2 * time.Second}), client: client}
	done := make(chan error, 1)
	go func() {
		_, err := s.start(context.Background())
		done <- err
	}()
	answers := map[string]string{
		"room_list":  `{"ok":true,"result":{"rooms":[]}}`,
		"board_list": `{"ok":true,"result":{"counts":{},"items":[],"more":false,"next_after":null}}`,
		"inbox":      `{"ok":true,"result":{"messages":[],"last_seq":0,"acked_through":0,"more":false}}`,
	}
	for read := 0; read < 3; {
		line, err := srv.in.ReadString('\n')
		if err != nil {
			t.Fatal(err)
		}
		var m struct {
			ID     *int64 `json:"id"`
			Method string `json:"method"`
			Params struct {
				Name string `json:"name"`
				URI  string `json:"uri"`
			} `json:"params"`
		}
		_ = json.Unmarshal([]byte(line), &m)
		if m.ID == nil {
			continue
		}
		var result any = map[string]any{}
		switch m.Method {
		case "resources/subscribe":
			srv.send(t, map[string]any{"jsonrpc": "2.0", "id": *m.ID, "result": result})
			srv.send(t, map[string]any{"jsonrpc": "2.0", "method": "notifications/resources/updated", "params": map[string]string{"uri": m.Params.URI}})
			continue
		case "tools/call":
			read++
			if m.Params.Name == "inbox" {
				srv.send(t, map[string]any{"jsonrpc": "2.0", "method": "notifications/resources/updated", "params": map[string]string{"uri": DecisionsURI}})
				time.Sleep(50 * time.Millisecond)
			}
			result = toolResult(answers[m.Params.Name], false)
		}
		srv.send(t, map[string]any{"jsonrpc": "2.0", "id": *m.ID, "result": result})
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if got := client.TakeNotices(); len(got) != 1 || got[0] != DecisionsURI {
		t.Errorf("notices after start %v", got)
	}
}

// A state holds every post that Koinon retains, over more than one page (review of
// 922e3f7, F3).
func TestConnectorKeepsEveryPost(t *testing.T) {
	h, c := newHost(t, "local")
	c.Write("page", "20")
	var posts []map[string]any
	for i := int64(1); i <= 75; i++ {
		posts = append(posts, post(i, fmt.Sprint(i)))
	}
	c.SetBoard(koinontest.Board{Rooms: []koinontest.FakeRoom{{RoomID: "r1", Name: "busy", Posts: posts}}})
	conn, _ := start(t, h, fastOptions())
	s := waitFor(t, conn, "the room", func(s State) bool { return connected(s) && len(s.Rooms) == 1 && len(s.Rooms[0].Posts) > 0 })
	if n := len(s.Rooms[0].Posts); n != 75 || s.Rooms[0].Posts[0].Body != "1" {
		t.Errorf("%d of 75 posts", n)
	}
}
