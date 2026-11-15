package koinontest

import (
	"bufio"
	"crypto/ed25519"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/rwcii/koinon-voice/internal/signed"
)

// SignedNode is a synthetic `koinon mcp --maintainer --signed` that acts out the Koinon
// contract (rwcii/koinon docs/sprints/2026-11-14-signed-maintainer/decision.md at fa95824):
// it checks the session envelope and every action's signature, key, target, window and
// floor, keeps receipts per request, and serves its own pin and paired targets.
type SignedNode struct {
	Pin  string
	Key  ed25519.PublicKey
	Now  func() time.Time
	Wait time.Duration // the longest changes wait; 0 is the envelope's wait_seconds

	mu      sync.Mutex
	targets map[string]*Target
	// Seen holds every envelope the node received, as the client sent it.
	Seen []signed.Envelope
}

// Target is one computer's data on the fake node.
type Target struct {
	Items    map[string]map[string]any
	Verdicts []map[string]any
	Versions map[string]int64 // served as 20-digit strings
	Offline  bool
	// Auth is the current authorization ID on this computer (D1.7).
	Auth string
	// DropNext drops the answer of the next write after it is recorded (outcome_unknown).
	DropNext bool
	// Name is the computer's name in computers; State and Reason its link state there
	// (connected unless Offline).
	Name   string
	Reason string
	// Inbox holds the maintainer's messages, by ascending seq; AckedThrough the cursor.
	Inbox        []map[string]any
	AckedThrough int64
	// Rooms lists the rooms; Posts holds each room's posts, by ascending seq.
	Rooms []map[string]any
	Posts map[string][]map[string]any
	// Fail makes the next reads of an operation fail with code internal, a count per op.
	Fail map[string]int
	// Reads counts the reads of each operation the target answered or refused.
	Reads    map[string]int
	receipts map[string]receipt
	bump     chan struct{}
}

type receipt struct {
	digest string
	result json.RawMessage
	code   string
}

// NewSignedNode returns a node with its own pin as an empty target.
func NewSignedNode(pin string, key ed25519.PublicKey, now func() time.Time) *SignedNode {
	n := &SignedNode{Pin: pin, Key: key, Now: now, targets: map[string]*Target{}}
	n.AddTarget(pin)
	return n
}

// AddTarget adds a computer that the node serves.
func (n *SignedNode) AddTarget(pin string) *Target {
	n.mu.Lock()
	defer n.mu.Unlock()
	t := &Target{Items: map[string]map[string]any{}, Versions: map[string]int64{}, receipts: map[string]receipt{}, bump: make(chan struct{}),
		Posts: map[string][]map[string]any{}, Fail: map[string]int{}, Reads: map[string]int{}}
	n.targets[pin] = t
	return t
}

// Change runs f on a target under the node's lock and wakes its changes waits.
func (n *SignedNode) Change(pin string, f func(t *Target)) {
	n.mu.Lock()
	t := n.targets[pin]
	f(t)
	close(t.bump)
	t.bump = make(chan struct{})
	n.mu.Unlock()
}

// Serve runs one session on the streams: the session envelope line, then MCP.
func (n *SignedNode) Serve(r io.Reader, w io.Writer) error {
	br := bufio.NewReaderSize(r, 1<<20)
	line, err := br.ReadBytes('\n')
	if err != nil {
		return err
	}
	var session signed.Envelope
	if err := json.Unmarshal(line, &session); err != nil {
		return fmt.Errorf("session line: %w", err)
	}
	if code := n.check(session); code != "" || session.Op != "session" || session.Target != n.Pin {
		fmt.Fprintf(w, "")
		return fmt.Errorf("session refused: %s", code)
	}
	var wmu sync.Mutex
	write := func(v any) {
		b, _ := json.Marshal(v)
		wmu.Lock()
		w.Write(append(b, '\n'))
		wmu.Unlock()
	}
	var wg sync.WaitGroup
	defer wg.Wait()
	for {
		line, err := br.ReadBytes('\n')
		if err != nil {
			return nil
		}
		var m struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
		}
		if json.Unmarshal(line, &m) != nil {
			continue
		}
		switch m.Method {
		case "initialize":
			write(map[string]any{"jsonrpc": "2.0", "id": m.ID, "result": map[string]any{"protocolVersion": "2025-06-18", "capabilities": map[string]any{"tools": map[string]any{}}}})
		case "tools/call":
			wg.Add(1)
			go func(id json.RawMessage, params json.RawMessage) {
				defer wg.Done()
				text, isError := n.call(params)
				write(map[string]any{"jsonrpc": "2.0", "id": id, "result": map[string]any{
					"content": []map[string]any{{"type": "text", "text": text}}, "isError": isError}})
			}(m.ID, m.Params)
		}
	}
}

// check verifies an envelope as the target does (D3.3): its version, its authorization on
// the target, and its signature.
func (n *SignedNode) check(e signed.Envelope) string {
	if e.V != signed.Version {
		return "version"
	}
	n.mu.Lock()
	t, ok := n.targets[e.Target]
	auth := ""
	if ok {
		auth = t.Auth
	}
	n.mu.Unlock()
	if !ok {
		return "target"
	}
	if e.Auth != auth {
		return "authorization_changed"
	}
	if signed.Verify(n.Key, e) != nil {
		return "signature"
	}
	return ""
}

func answer(result any) (string, bool) {
	b, _ := json.Marshal(map[string]any{"ok": true, "result": result})
	return string(b), false
}

func refuse(code string, details any) (string, bool) {
	b, _ := json.Marshal(map[string]any{"ok": false, "code": code, "error": code, "details": details})
	return string(b), true
}

func (n *SignedNode) call(params json.RawMessage) (string, bool) {
	var p struct {
		Name      string `json:"name"`
		Arguments struct {
			Envelope signed.Envelope `json:"envelope"`
		} `json:"arguments"`
	}
	dec := json.NewDecoder(bytesReader(params))
	dec.UseNumber()
	if err := dec.Decode(&p); err != nil || p.Name != "signed" {
		return refuse("invalid_request", nil)
	}
	e := p.Arguments.Envelope
	e.Args = normalize(e.Args)
	n.mu.Lock()
	n.Seen = append(n.Seen, e)
	n.mu.Unlock()
	if code := n.check(e); code != "" {
		return refuse("signature_refused", map[string]string{"reason": code})
	}
	n.mu.Lock()
	t, ok := n.targets[e.Target]
	n.mu.Unlock()
	if !ok {
		return refuse("signature_refused", map[string]string{"reason": "target"})
	}
	if t.Offline && e.Target != n.Pin {
		return refuse("computer_offline", map[string]int64{"last_seen": n.Now().Unix() - 60})
	}
	now := n.Now().Unix()
	write := e.Op == "decision_verdict" || e.Op == "ack" || e.Op == "send" || e.Op == "room_post"
	if write {
		return n.write(t, e, now)
	}
	if now-e.Created > 300 || e.Created-now > 300 {
		return refuse("signature_refused", map[string]string{"reason": "time"})
	}
	switch e.Op {
	case "decision_get":
		n.mu.Lock()
		t.Reads[e.Op]++
		it, ok := t.Items[fmt.Sprint(e.Args["reference"])]
		n.mu.Unlock()
		if !ok {
			return refuse("not_found", nil)
		}
		return answer(it)
	case "outcome":
		req := fmt.Sprint(e.Args["request"])
		n.mu.Lock()
		t.Reads[e.Op]++
		r, ok := t.receipts[req]
		n.mu.Unlock()
		switch {
		case !ok:
			return answer(map[string]any{"state": "unknown_request", "now": now})
		case r.code != "":
			return answer(map[string]any{"state": "refused", "code": r.code})
		default:
			return answer(map[string]any{"state": "recorded", "result": r.result})
		}
	case "changes":
		return n.changes(t, e)
	}
	return n.read(e.Target, t, e)
}

func (n *SignedNode) write(t *Target, e signed.Envelope, now int64) (string, bool) {
	b, _ := signed.SignedBytes(e)
	digest := fmt.Sprintf("%x", b)
	n.mu.Lock()
	defer n.mu.Unlock()
	if r, ok := t.receipts[e.Request]; ok {
		if r.digest != digest {
			return refuse("request_conflict", nil)
		}
		if r.code != "" {
			return refuse(r.code, nil)
		}
		return answer(r.result)
	}
	if now-e.Created > 300 || e.Created-now > 300 {
		return refuse("signature_refused", map[string]string{"reason": "time"})
	}
	ref := fmt.Sprint(e.Args["reference"])
	it, ok := t.Items[ref]
	if !ok {
		t.receipts[e.Request] = receipt{digest: digest, code: "not_found"}
		return refuse("not_found", nil)
	}
	if rev, _ := e.Args["revision"].(int64); rev != it["revision"].(int64) {
		t.receipts[e.Request] = receipt{digest: digest, code: "stale_revision"}
		return refuse("stale_revision", map[string]int64{"current_revision": it["revision"].(int64)})
	}
	if e.Args["snapshot"] != snapshotOf(it) {
		t.receipts[e.Request] = receipt{digest: digest, code: "snapshot_changed"}
		return refuse("snapshot_changed", nil)
	}
	v := map[string]any{"reference": ref, "revision": it["revision"], "verdict": e.Args["verdict"], "via": "key:koinon-voice"}
	t.Verdicts = append(t.Verdicts, v)
	result, _ := json.Marshal(v)
	t.receipts[e.Request] = receipt{digest: digest, result: result}
	t.Versions["decisions"]++
	close(t.bump)
	t.bump = make(chan struct{})
	if t.DropNext {
		t.DropNext = false
		return refuse("outcome_unknown", nil)
	}
	return answer(json.RawMessage(result))
}

func (n *SignedNode) changes(t *Target, e signed.Envelope) (string, bool) {
	have, _ := e.Args["versions"].(map[string]any)
	wait := time.Duration(0)
	if s, ok := e.Args["wait_seconds"].(int64); ok {
		wait = time.Duration(s) * time.Second
	}
	if n.Wait > 0 {
		wait = n.Wait
	}
	deadline := time.After(wait)
	for {
		n.mu.Lock()
		changed := false
		cur := map[string]string{}
		list := []string{}
		for topic, v := range t.Versions {
			cur[topic] = fmt.Sprintf("%020d", v)
			h, _ := have[topic].(string)
			if cur[topic] != h {
				changed = true
				list = append(list, topic)
			}
		}
		// A topic the client has and the target no longer has is changed too.
		for topic := range have {
			if _, ok := t.Versions[topic]; !ok {
				changed = true
				list = append(list, topic)
			}
		}
		sort.Strings(list)
		bump := t.bump
		n.mu.Unlock()
		if changed {
			return answer(map[string]any{"versions": cur, "changed": list})
		}
		select {
		case <-bump:
		case <-deadline:
			return answer(map[string]any{"versions": cur, "changed": []string{}})
		}
	}
}

// snapshotOf is the fake target's own digest of an item (D2.2).
func snapshotOf(it map[string]any) string {
	str := func(k string) string { s, _ := it[k].(string); return s }
	cmds, _ := it["commands"].([]string)
	d, _ := signed.Snapshot{Reference: str("reference"), Revision: it["revision"].(int64), Kind: str("kind"),
		Title: str("title"), Where: str("where"), Context: str("context"), Recommendation: str("recommendation"),
		AgentText: str("agent_text"), Commands: cmds}.Digest()
	return d
}

// normalize turns json.Number values into int64, as the target reads v1 integers.
func normalize(m map[string]any) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		switch x := v.(type) {
		case json.Number:
			i, err := x.Int64()
			if err != nil {
				out[k] = x.String()
			} else {
				out[k] = i
			}
		case map[string]any:
			out[k] = normalize(x)
		default:
			out[k] = v
		}
	}
	return out
}

func bytesReader(b []byte) io.Reader { return &sliceReader{b: b} }

type sliceReader struct{ b []byte }

func (r *sliceReader) Read(p []byte) (int, error) {
	if len(r.b) == 0 {
		return 0, io.EOF
	}
	n := copy(p, r.b)
	r.b = r.b[n:]
	return n, nil
}

// SignedConfig is the control file of a fake signed process: the node's pin, the public
// key and the authorization of each target.
type SignedConfig struct {
	Pin       string            `json:"pin"`
	PublicKey []byte            `json:"public_key"`
	Auths     map[string]string `json:"auths"`
}

// fakeSigned runs `koinon mcp --maintainer --signed` on stdio from dir/signed.json, records
// its arguments in dir/args, and answers a refused session as Koinon does: one JSON line on
// standard error and exit status 1.
func fakeSigned(dir string) int {
	os.WriteFile(filepath.Join(dir, "args"), []byte(strings.Join(os.Args[1:], "\n")), 0o600)
	b, err := os.ReadFile(filepath.Join(dir, "signed.json"))
	if err != nil {
		return 2
	}
	var c SignedConfig
	if json.Unmarshal(b, &c) != nil {
		return 2
	}
	n := NewSignedNode(c.Pin, ed25519.PublicKey(c.PublicKey), time.Now)
	for pin, auth := range c.Auths {
		if pin != c.Pin {
			n.AddTarget(pin)
		}
		n.Change(pin, func(t *Target) { t.Auth = auth })
	}
	if err := n.Serve(os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, `{"code":"signature_refused","error":"the session envelope was refused","ok":false}`)
		return 1
	}
	return 0
}

// WriteSigned writes an executable that runs the test binary as a fake signed process with
// control directory dir.
func WriteSigned(path, dir string, c SignedConfig) string {
	b, _ := json.Marshal(c)
	os.WriteFile(filepath.Join(dir, "signed.json"), b, 0o600)
	script := fmt.Sprintf("#!/bin/sh\nexec env %s=%q %q \"$@\"\n", SignedEnv, dir, os.Args[0])
	os.WriteFile(path, []byte(script), 0o700)
	return path
}

// Route is the fake node's Serve route (Koinon contract dfb2ed7, D5.2 and Answer shapes):
// POST /v1/maintainer/signed/session takes a signed session envelope and answers a flat
// token; POST /v1/maintainer/signed takes a bearer token and one envelope. Every refusal is
// HTTP 409; an unknown token is maintainer_token_expired, before the envelope is read.
type Route struct {
	Node *SignedNode

	mu       sync.Mutex
	tokens   map[string]bool
	next     int
	Sessions int // session requests answered
	// RefuseSessions makes the session route refuse with signature_refused.
	RefuseSessions bool
	// Hook, when set, may answer a request itself (for redirects, junk or echoed tokens).
	Hook func(w http.ResponseWriter, r *http.Request) bool
}

// ExpireTokens ends every token, as a node restart does.
func (rt *Route) ExpireTokens() {
	rt.mu.Lock()
	rt.tokens = map[string]bool{}
	rt.mu.Unlock()
}

func (rt *Route) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if rt.Hook != nil && rt.Hook(w, r) {
		return
	}
	reply := func(status int, text string) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		io.WriteString(w, text)
	}
	if r.Method != http.MethodPost || r.Header.Get("Content-Type") != "application/json" {
		reply(400, `{"ok":false,"code":"invalid_request","error":"invalid request"}`)
		return
	}
	switch r.URL.Path {
	case "/v1/maintainer/signed/session":
		var e signed.Envelope
		if json.NewDecoder(io.LimitReader(r.Body, 512<<10)).Decode(&e) != nil {
			reply(400, `{"ok":false,"code":"invalid_request","error":"invalid request"}`)
			return
		}
		rt.mu.Lock()
		rt.Sessions++
		refuse := rt.RefuseSessions
		rt.mu.Unlock()
		if code := rt.Node.check(e); refuse || code != "" || e.Op != "session" || e.Target != rt.Node.Pin {
			reply(409, `{"ok":false,"code":"signature_refused","error":"the session was refused","details":{"reason":"signature"}}`)
			return
		}
		rt.mu.Lock()
		if rt.tokens == nil {
			rt.tokens = map[string]bool{}
		}
		rt.next++
		token := fmt.Sprintf("tok-%d-secretvalue", rt.next)
		rt.tokens[token] = true
		rt.mu.Unlock()
		reply(200, fmt.Sprintf(`{"ok":true,"token":%q,"client":"key:koinon-voice","idle_seconds":86400,"absolute_seconds":604800}`, token))
	case "/v1/maintainer/signed":
		token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		rt.mu.Lock()
		ok := rt.tokens[token]
		rt.mu.Unlock()
		if !ok {
			reply(409, `{"ok":false,"code":"maintainer_token_expired","error":"the session ended"}`)
			return
		}
		b, _ := io.ReadAll(io.LimitReader(r.Body, 512<<10))
		params, _ := json.Marshal(map[string]any{"name": "signed", "arguments": map[string]json.RawMessage{"envelope": b}})
		text, isError := rt.Node.call(params)
		status := 200
		if isError {
			status = 409
		}
		reply(status, text)
	default:
		http.NotFound(w, r)
	}
}
