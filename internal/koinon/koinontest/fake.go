// Package koinontest is a synthetic `koinon mcp --maintainer` and a synthetic ssh for tests
// (WDR 006, Decision 2). A test binary runs them from TestMain when Mode's environment
// variable is set, and a test controls them through files in a directory. They act out
// what Koinon v0.5.0 did on 2026-10-27 (docs/sprints/2026-10-27-connectors/
// definition-of-done.md).
package koinontest

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Environment variables that select a fake. Each holds the control directory.
const (
	KoinonEnv = "KOINON_VOICE_FAKE_KOINON"
	SSHEnv    = "KOINON_VOICE_FAKE_SSH"
)

// Main runs a fake when its environment variable is set, and reports whether it did. A
// TestMain calls it first and exits when it returns true.
func Main() bool {
	if dir := os.Getenv(SSHEnv); dir != "" {
		os.Exit(fakeSSH(dir))
	}
	if dir := os.Getenv(KoinonEnv); dir != "" {
		os.Exit(fakeKoinon(dir))
	}
	return false
}

// Board is the daemon's data in the control directory's board.json.
type Board struct {
	Items []map[string]any `json:"items"`
	Inbox []map[string]any `json:"inbox"`
	Acked int64            `json:"acked"`
	Rooms []FakeRoom       `json:"rooms"`
}

// FakeRoom is a room with its posts.
type FakeRoom struct {
	RoomID string           `json:"room_id"`
	Name   string           `json:"name"`
	Posts  []map[string]any `json:"posts"`
}

// Control is a fake's control directory.
type Control struct{ Dir string }

// NewControl makes a control directory with the expected secret and an empty board.
func NewControl(dir, secret string) Control {
	c := Control{Dir: dir}
	c.Write("secret", secret)
	c.SetBoard(Board{})
	return c
}

// Write writes one control file atomically.
func (c Control) Write(name, content string) {
	tmp := filepath.Join(c.Dir, "."+name+".tmp")
	if err := os.WriteFile(tmp, []byte(content), 0o600); err != nil {
		panic(err)
	}
	if err := os.Rename(tmp, filepath.Join(c.Dir, name)); err != nil {
		panic(err)
	}
}

// Read reads a control file, or "" when it does not exist.
func (c Control) Read(name string) string {
	b, _ := os.ReadFile(filepath.Join(c.Dir, name))
	return string(b)
}

// Remove removes a control file.
func (c Control) Remove(name string) { _ = os.Remove(filepath.Join(c.Dir, name)) }

// SetBoard replaces the daemon's data.
func (c Control) SetBoard(b Board) {
	raw, err := json.Marshal(b)
	if err != nil {
		panic(err)
	}
	c.Write("board.json", string(raw))
}

// Board reads the daemon's data.
func (c Control) Board() Board {
	var b Board
	_ = json.Unmarshal([]byte(c.Read("board.json")), &b)
	return b
}

// Restart acts out a daemon restart: each running fake's token ends.
func (c Control) Restart() {
	n, _ := strconv.Atoi(strings.TrimSpace(c.Read("epoch")))
	c.Write("epoch", strconv.Itoa(n+1))
}

// Starts counts the fake's starts.
func (c Control) Starts() int {
	return strings.Count(c.Read("starts"), "\n")
}

// Item returns a decision item as decision_get gives it.
func Item(reference, kind string, revision int64, title, context, recommendation string) map[string]any {
	return map[string]any{
		"reference": reference, "order": 0, "title": title, "where": "", "agent_text": "",
		"context": context, "recommendation": recommendation, "kind": kind, "state": "active",
		"revision": revision, "commands": []string{}, "recipients": []any{},
		"author_family": "codex", "author_name": "codex-synthetic-a4",
		"created_at": 1793125063611, "updated_at": 1793125063611, "verdict": nil, "verdicts": 0,
	}
}

type fake struct {
	c      Control
	client string
	epoch  string
	out    *bufio.Writer
	outMu  sync.Mutex
	mu     sync.Mutex
	subs   map[string]bool
	seen   map[string]string
}

func fakeKoinon(dir string) int {
	c := Control{Dir: dir}
	f, err := os.OpenFile(filepath.Join(dir, "starts"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err == nil {
		args, _ := json.Marshal(os.Args[1:])
		fmt.Fprintf(f, "%s\n", args)
		f.Close()
	}
	c.Write("pid", strconv.Itoa(os.Getpid()))
	in := bufio.NewReader(os.Stdin)
	if c.Read("down") != "" {
		fmt.Fprintln(os.Stderr, `{"code":"daemon_error","detail":"daemon unavailable","ok":false}`)
		return 1
	}
	secret, _ := in.ReadString('\n')
	if strings.TrimSuffix(secret, "\n") != c.Read("secret") || secret == "\n" {
		fmt.Fprintln(os.Stderr, `{"code":"maintainer_secret_invalid","detail":"daemon refused request: maintainer_secret_invalid: the maintainer secret is not correct","ok":false}`)
		return 1
	}
	client := "unknown"
	for i, a := range os.Args {
		if a == "--client" && i+1 < len(os.Args) {
			client = os.Args[i+1]
		}
	}
	k := &fake{c: c, client: client, epoch: c.Read("epoch"), out: bufio.NewWriter(os.Stdout), subs: map[string]bool{}, seen: map[string]string{}}
	go k.poll()
	for {
		line, err := in.ReadString('\n')
		if err != nil {
			if c.Read("stubborn") != "" {
				time.Sleep(time.Minute)
			}
			return 0
		}
		var m struct {
			ID     *int64          `json:"id"`
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
		}
		if json.Unmarshal([]byte(line), &m) != nil {
			continue
		}
		if m.ID == nil {
			continue // a notification
		}
		if c.Read("junk") != "" {
			k.writeRaw("Usage: koinon [flags]")
			continue
		}
		k.handle(*m.ID, m.Method, m.Params)
	}
}

func (k *fake) writeRaw(line string) {
	k.outMu.Lock()
	defer k.outMu.Unlock()
	k.out.WriteString(line + "\n")
	k.out.Flush()
}

func (k *fake) write(v any) {
	raw, _ := json.Marshal(v)
	k.writeRaw(string(raw))
}

func (k *fake) answer(id int64, result any) {
	k.write(map[string]any{"jsonrpc": "2.0", "id": id, "result": result})
}

func (k *fake) rpcError(id int64, code int, message, dataCode string) {
	k.write(map[string]any{"jsonrpc": "2.0", "id": id, "error": map[string]any{"code": code, "message": message, "data": map[string]string{"code": dataCode}}})
}

func (k *fake) toolAnswer(id int64, v any, isError bool) {
	raw, _ := json.Marshal(v)
	k.answer(id, map[string]any{"content": []map[string]any{{"type": "text", "text": string(raw) + "\n"}}, "isError": isError})
}

func refusal(code, message string, details any) map[string]any {
	r := map[string]any{"ok": false, "code": code, "error": message}
	if details != nil {
		r["details"] = details
	}
	return r
}

func ok(result any) map[string]any { return map[string]any{"ok": true, "result": result} }

func (k *fake) expired() bool { return k.c.Read("epoch") != k.epoch }

func (k *fake) exitExpired() {
	fmt.Fprintln(os.Stderr, `{"code":"maintainer_token_expired","detail":"daemon refused request: maintainer_token_expired: the maintainer token has ended; start koinon mcp --maintainer again","ok":false}`)
	os.Exit(1)
}

func (k *fake) handle(id int64, method string, params json.RawMessage) {
	switch method {
	case "initialize":
		k.answer(id, map[string]any{"protocolVersion": "2025-06-18", "capabilities": map[string]any{"resources": map[string]bool{"subscribe": true}, "tools": map[string]any{}}, "serverInfo": map[string]string{"name": "koinon", "version": "v0.5.0"}})
	case "resources/subscribe":
		var p struct {
			URI string `json:"uri"`
		}
		_ = json.Unmarshal(params, &p)
		if k.c.Read("subscribe-unavailable") != "" {
			k.rpcError(id, -32603, "daemon_unavailable", "daemon_unavailable")
			return
		}
		k.mu.Lock()
		k.subs[p.URI] = true
		k.seen[p.URI] = k.version(p.URI)
		k.mu.Unlock()
		k.answer(id, map[string]any{})
		k.write(map[string]any{"jsonrpc": "2.0", "method": "notifications/resources/updated", "params": map[string]string{"uri": p.URI}})
	case "tools/call":
		var p struct {
			Name      string         `json:"name"`
			Arguments map[string]any `json:"arguments"`
		}
		_ = json.Unmarshal(params, &p)
		if k.c.Read("hang") != "" {
			return
		}
		if k.expired() {
			k.toolAnswer(id, refusal("maintainer_token_expired", "the maintainer token has ended; start koinon mcp --maintainer again", nil), true)
			k.exitExpired()
		}
		v, isErr := k.tool(p.Name, p.Arguments)
		k.toolAnswer(id, v, isErr)
	default:
		k.rpcError(id, -32601, "method not found", "")
	}
}

func num(v any) int64 {
	f, _ := v.(float64)
	return int64(f)
}

func (k *fake) pageSize() int {
	n, err := strconv.Atoi(strings.TrimSpace(k.c.Read("page")))
	if err != nil || n < 1 {
		return 100
	}
	return n
}

func (k *fake) tool(name string, args map[string]any) (any, bool) {
	b := k.c.Board()
	switch name {
	case "board_list":
		after := num(args["after"])
		size := min(k.pageSize(), int(max(num(args["limit"]), 1)))
		var counts = map[string]int{"waiting": 0, "in_progress": 0, "open_fyi": 0, "archived": 0}
		var page []map[string]any
		more := false
		var next any
		for i, it := range b.Items {
			if it["kind"] == "fyi" {
				counts["open_fyi"]++
			} else {
				counts["waiting"]++
			}
			if int64(i) < after {
				continue
			}
			if len(page) == size {
				more = true
				continue
			}
			page = append(page, map[string]any{"reference": it["reference"], "title": it["title"], "kind": it["kind"], "state": it["state"], "revision": it["revision"], "order": it["order"]})
			next = int64(i + 1)
		}
		if !more {
			next = nil
		}
		return ok(map[string]any{"counts": counts, "items": page, "more": more, "next_after": next}), false
	case "decision_get":
		for _, it := range b.Items {
			if it["reference"] == args["reference"] {
				return ok(it), false
			}
		}
		return refusal("not_found", "no decision item has this reference", nil), true
	case "decision_verdict":
		for i, it := range b.Items {
			if it["reference"] != args["reference"] {
				continue
			}
			verdict, _ := args["verdict"].(string)
			if (it["kind"] == "fyi") != (verdict == "dismissed") {
				return refusal("invalid_verdict", "a decision item takes approve or decline; an fyi item takes dismiss", nil), true
			}
			if num(it["revision"]) != num(args["revision"]) {
				return refusal("stale_revision", "the decision item changed after the page showed it", map[string]any{"current_revision": num(it["revision"])}), true
			}
			note, _ := args["note"].(string)
			v := map[string]any{"verdict_id": i + 1, "revision": num(it["revision"]), "verdict": verdict, "note": note, "at": time.Now().UnixMilli(), "via": "mcp:" + k.client, "reference": it["reference"]}
			raw, _ := json.Marshal(v)
			f, err := os.OpenFile(filepath.Join(k.c.Dir, "verdicts.jsonl"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
			if err == nil {
				fmt.Fprintf(f, "%s\n", raw)
				f.Close()
			}
			b.Items = append(b.Items[:i], b.Items[i+1:]...)
			k.c.SetBoard(b)
			delete(v, "reference")
			return ok(v), false
		}
		return refusal("not_found", "no decision item has this reference", nil), true
	case "inbox":
		after := num(args["after"])
		var msgs []map[string]any
		var last int64
		more := false
		for _, m := range b.Inbox {
			seq := num(m["seq"])
			last = max(last, seq)
			if seq <= after {
				continue
			}
			if len(msgs) == k.pageSize() {
				more = true
				continue
			}
			msgs = append(msgs, m)
		}
		return ok(map[string]any{"messages": msgs, "last_seq": last, "acked_through": b.Acked, "more": more}), false
	case "room_list":
		rooms := []map[string]any{}
		for _, r := range b.Rooms {
			rooms = append(rooms, map[string]any{"room_id": r.RoomID, "name": r.Name, "revision": 1, "created_at": 1793125063843, "members": []any{}})
		}
		return ok(map[string]any{"rooms": rooms}), false
	case "room_read":
		for _, r := range b.Rooms {
			if r.RoomID != args["room_id"] {
				continue
			}
			after := num(args["after"])
			var posts []map[string]any
			more := false
			for _, p := range r.Posts {
				if num(p["seq"]) <= after {
					continue
				}
				if len(posts) == k.pageSize() {
					more = true
					continue
				}
				posts = append(posts, p)
			}
			return ok(map[string]any{"room_id": r.RoomID, "start": after, "floor": 0, "head": len(r.Posts), "posts": posts, "gaps": []any{}, "more": more}), false
		}
		return refusal("room_not_found", "no room has this ID", nil), true
	}
	return refusal("invalid_arguments", "unknown tool "+name, nil), true
}

// version is a resource's content, standing in for Koinon's version of it.
func (k *fake) version(uri string) string {
	b := k.c.Board()
	var v any
	switch {
	case uri == "koinon://maintainer/decisions":
		v = b.Items
	case uri == "koinon://maintainer/inbox":
		v = []any{b.Inbox, b.Acked}
	case strings.HasPrefix(uri, "koinon://maintainer/rooms/"):
		id := strings.TrimPrefix(uri, "koinon://maintainer/rooms/")
		v = "gone"
		for _, r := range b.Rooms {
			if r.RoomID == id {
				v = r.Posts
			}
		}
	}
	raw, _ := json.Marshal(v)
	return string(raw)
}

// poll acts out Koinon's change poll while a resource is subscribed: a notice per changed
// URI, and an exit when the token ends (a daemon restart).
func (k *fake) poll() {
	for {
		time.Sleep(20 * time.Millisecond)
		k.mu.Lock()
		n := len(k.subs)
		var changed []string
		for uri := range k.subs {
			if v := k.version(uri); v != k.seen[uri] {
				k.seen[uri] = v
				changed = append(changed, uri)
			}
		}
		k.mu.Unlock()
		if n > 0 && k.expired() && k.c.Read("poll-blind") == "" {
			k.exitExpired()
		}
		for _, uri := range changed {
			k.write(map[string]any{"jsonrpc": "2.0", "method": "notifications/resources/updated", "params": map[string]string{"uri": uri}})
		}
	}
}

// fakeSSH records its arguments and runs the remote words with sh, as ssh hands them to
// the remote shell. The destination "unreachable" fails as ssh does.
func fakeSSH(dir string) int {
	args := os.Args[1:]
	raw, _ := json.Marshal(args)
	_ = os.WriteFile(filepath.Join(dir, "ssh-args"), raw, 0o600)
	i := 0
	for i < len(args) && args[i] != "--" {
		i++
	}
	if i+2 > len(args) {
		fmt.Fprintln(os.Stderr, "usage: ssh destination command")
		return 255
	}
	dest := args[i+1]
	if dest == "unreachable" {
		fmt.Fprintln(os.Stderr, "ssh: connect to host unreachable port 22: Connection refused")
		return 255
	}
	cmd := exec.Command("/bin/sh", "-c", strings.Join(args[i+2:], " "))
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	env := []string{}
	for _, e := range os.Environ() {
		if !strings.HasPrefix(e, SSHEnv+"=") {
			env = append(env, e)
		}
	}
	cmd.Env = env
	if err := cmd.Run(); err != nil {
		if exit, ok := err.(*exec.ExitError); ok {
			return exit.ExitCode()
		}
		return 255
	}
	return 0
}

// WriteKoinon writes an executable at path that runs this test binary as the fake Koinon
// of control c, and returns path. Each fake host has its own wrapper, so hosts in one test
// have separate daemons.
func WriteKoinon(path string, c Control) string {
	return writeWrapper(path, KoinonEnv, c.Dir)
}

// WriteSSH writes an executable at path that runs this test binary as the fake ssh, which
// records its arguments in c, and returns path.
func WriteSSH(path string, c Control) string {
	return writeWrapper(path, SSHEnv, c.Dir)
}

func writeWrapper(path, env, dir string) string {
	self, err := os.Executable()
	if err != nil {
		panic(err)
	}
	script := "#!/bin/sh\n" + env + "=" + shellQuote(dir) + " exec " + shellQuote(self) + ` "$@"` + "\n"
	if err := os.WriteFile(path, []byte(script), 0o700); err != nil {
		panic(err)
	}
	return path
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
