package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/rwcii/koinon-voice/internal/koinon"
	"github.com/rwcii/koinon-voice/internal/koinon/koinontest"
	"github.com/rwcii/koinon-voice/internal/signed"
)

const (
	cmdNodePin  = "a0a1a2a3a4a5a6a7a8a9aaabacadaeafb0b1b2b3b4b5b6b7b8b9babbbcbdbebf"
	cmdOtherPin = "b0b1b2b3b4b5b6b7b8b9babbbcbdbebfc0c1c2c3c4c5c6c7c8c9cacbcccdcecf"
	cmdGonePin  = "c0c1c2c3c4c5c6c7c8c9cacbcccdcecfd0d1d2d3d4d5d6d7d8d9dadbdcdddedf"
)

// signedHosts adds a signed row, "studio", to the board's hosts file: a fake node "desk"
// paired with "lab-1", which holds a decision item. With withKey the install has a key
// enrolled on both; the journal holds one verdict through a node that no row names.
func signedHosts(t *testing.T, withKey bool) (string, *koinontest.SignedNode) {
	t.Helper()
	hosts := boardHosts(t)
	dir := t.TempDir()
	store := &memoryStore{items: map[string]string{}}
	oldStore, oldEnr, oldJournal, oldOpts := keyStore, keyEnrollments, signedJournal, sourceOptions
	keyStore = store
	keyEnrollments = signed.Enrollments{Path: filepath.Join(dir, "enrollments.json")}
	signedJournal = koinon.Journal{Path: filepath.Join(dir, "pending.json")}
	t.Cleanup(func() { keyStore, keyEnrollments, signedJournal, sourceOptions = oldStore, oldEnr, oldJournal, oldOpts })

	key, _ := signed.Create(context.Background(), &memoryStore{items: map[string]string{}}, "robert")
	if withKey {
		key, _ = signed.Create(context.Background(), store, "robert")
	}
	node := koinontest.NewSignedNode(cmdNodePin, key.Public(), time.Now)
	node.Wait = 200 * time.Millisecond
	node.AddTarget(cmdOtherPin)
	auths := map[string]string{}
	for _, pin := range []string{cmdNodePin, cmdOtherPin} {
		st, _ := signed.Enrollment(key.Public(), pin, "robert", koinon.ClientName, time.Now().Unix())
		st, _ = signed.Sign(key.Private, st)
		if withKey {
			e, err := keyEnrollments.Record(st, "robert", koinon.ClientName)
			if err != nil {
				t.Fatal(err)
			}
			auths[pin] = e.Auth
		} else {
			auths[pin], _ = signed.AuthID(st)
		}
	}
	node.Change(cmdNodePin, func(tg *koinontest.Target) {
		tg.Auth, tg.Name = auths[cmdNodePin], "desk"
		tg.Versions["computers"], tg.Versions["decisions"], tg.Versions["inbox"] = 1, 1, 1
	})
	node.Change(cmdOtherPin, func(tg *koinontest.Target) {
		tg.Auth, tg.Name = auths[cmdOtherPin], "lab-1"
		tg.Items["S-1"] = map[string]any{"reference": "S-1", "revision": int64(4), "kind": "decision", "title": "Sign the release",
			"where": "lab", "context": "signed ctx", "recommendation": "Yes.", "agent_text": "", "commands": []string{}, "state": "active"}
		tg.Versions["decisions"], tg.Versions["inbox"] = 1, 1
	})
	sourceOptions = koinon.SourceOptions{
		MinDelay: 10 * time.Millisecond, MaxDelay: 50 * time.Millisecond,
		Watch: koinon.WatchOptions{Delay: func(int) time.Duration { return 10 * time.Millisecond }},
		Open: func(ctx context.Context, opts koinon.SignedOptions) (*koinon.Signed, error) {
			cr, cw := io.Pipe()
			nr, nw := io.Pipe()
			go func() { node.Serve(cr, nw); nw.Close() }()
			return koinon.OpenSigned(ctx, opts, nr, cw)
		},
	}
	// A verdict sent through a node that this hosts file no longer names.
	gone := signed.Envelope{V: signed.Version, Key: key.ID(), Target: cmdOtherPin, Op: "decision_verdict",
		Args: map[string]any{"reference": "S-0", "verdict": "approved"}, Request: "req-gone-0000000000", Auth: auths[cmdOtherPin], Created: 1}
	if _, err := signedJournal.Reserve(koinon.Entry{NodePin: cmdGonePin, Node: "old-node", Envelope: gone, FirstSent: 1}); err != nil {
		t.Fatal(err)
	}
	f, _ := os.OpenFile(hosts, os.O_APPEND|os.O_WRONLY, 0o600)
	fmt.Fprintf(f, "[[host]]\nlabel = \"studio\"\nnode_pin = %q\n", cmdNodePin)
	f.Close()
	return hosts, node
}

type signedBoard struct {
	Hosts []struct {
		Host, Status, Code, Reason, Pin, Node string
		Items                                 []koinon.Item
	}
	Unresolved []koinon.Unrouted
	OK         bool
}

func TestBoardSignedAndSecretRows(t *testing.T) {
	hosts, _ := signedHosts(t, true)
	var stdout, stderr bytes.Buffer
	run([]string{"board", "--hosts", hosts, "--timeout", "20s"}, nil, &stdout, &stderr)
	var out signedBoard
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
		t.Fatalf("%v: %s %s", err, stdout.String(), stderr.String())
	}
	got := map[string]string{}
	for _, h := range out.Hosts {
		got[h.Host] = string(h.Status) + "|" + h.Pin + "|" + h.Node
	}
	if got["local"] != "connected||" || !strings.HasPrefix(got["other"], "disconnected||") ||
		got["desk"] != "connected|"+cmdNodePin+"|studio" || got["lab-1"] != "connected|"+cmdOtherPin+"|studio" {
		t.Errorf("hosts %v", got)
	}
	for _, h := range out.Hosts {
		if h.Host == "lab-1" && (len(h.Items) != 1 || h.Items[0].Title != "Sign the release") {
			t.Errorf("lab-1 items %+v", h.Items)
		}
	}
	if len(out.Unresolved) != 1 || out.Unresolved[0].NodePin != cmdGonePin || out.Unresolved[0].Target != cmdOtherPin ||
		!strings.Contains(out.Unresolved[0].Reason, "no signed row in hosts.toml has node_pin "+cmdGonePin) {
		t.Errorf("unresolved %+v", out.Unresolved)
	}
}

func TestBoardSignedRowWithoutAKey(t *testing.T) {
	hosts, node := signedHosts(t, false)
	var stdout, stderr bytes.Buffer
	run([]string{"board", "--hosts", hosts, "--timeout", "20s"}, nil, &stdout, &stderr)
	var out signedBoard
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
		t.Fatalf("%v: %s", err, stdout.String())
	}
	got := map[string]string{}
	for _, h := range out.Hosts {
		got[h.Host] = string(h.Status) + "|" + h.Code
	}
	if got["local"] != "connected|" || got["studio"] != "disconnected|key_unavailable" || out.OK {
		t.Errorf("hosts %v ok %v", got, out.OK)
	}
	if len(node.Seen) != 0 {
		t.Errorf("a signed row without a key sent %d envelopes", len(node.Seen))
	}
}

func TestDashboardSignedVerdict(t *testing.T) {
	hosts, node := signedHosts(t, true)
	ctx, cancel := context.WithCancel(context.Background())
	watchContext = func() (context.Context, context.CancelFunc) { return ctx, cancel }
	openLink = func(string) error { return nil }
	t.Cleanup(func() {
		watchContext = func() (context.Context, context.CancelFunc) { return context.WithCancel(context.Background()) }
	})
	outR, outW := io.Pipe()
	var stderr bytes.Buffer
	done := make(chan int, 1)
	go func() {
		done <- run([]string{"dashboard", "--hosts", hosts, "--listen", "127.0.0.1:0", "--no-open"}, nil, outW, &stderr)
		outW.Close()
	}()
	var link string
	lines := bufio.NewScanner(outR)
	for lines.Scan() {
		if strings.Contains(lines.Text(), "/login?token=") {
			link = strings.TrimSpace(lines.Text())
			break
		}
	}
	go io.Copy(io.Discard, outR)
	jar, _ := cookiejar.New(nil)
	c := &http.Client{Jar: jar, Timeout: 5 * time.Second}
	resp, err := c.Get(link)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	u, _ := url.Parse(link)
	var page string
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline) && !strings.Contains(page, "Sign the release"); time.Sleep(50 * time.Millisecond) {
		resp, err := c.Get("http://" + u.Host + "/")
		if err != nil {
			t.Fatal(err)
		}
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		page = string(b)
	}
	if !strings.Contains(page, "Sign the release") || !strings.Contains(page, "Pending verdicts through nodes not in hosts.toml") {
		t.Fatalf("page:\n%s", page)
	}
	field := func(name string) string {
		// The signed form is the one with a digest.
		form := page[strings.Index(page, `name="digest"`)-1500:]
		m := regexp.MustCompile(`name="` + name + `" value="([^"]*)"`).FindStringSubmatch(form)
		if m == nil {
			t.Fatalf("no %s field", name)
		}
		return m[1]
	}
	form := url.Values{"csrf": {field("csrf")}, "source": {field("source")}, "pin": {cmdOtherPin}, "reference": {"S-1"},
		"revision": {"4"}, "digest": {field("digest")}, "intent": {field("intent")}, "verdict": {"approved"}, "note": {"go"}}
	req, _ := http.NewRequest("POST", "http://"+u.Host+"/verdict", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", "http://"+u.Host)
	resp, err = c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if !strings.Contains(string(b), "Verdict recorded on S-1 (lab-1)") {
		t.Errorf("after the verdict:\n%s", b)
	}
	n := 0
	node.Change(cmdOtherPin, func(tg *koinontest.Target) { n = len(tg.Verdicts) })
	if n != 1 {
		t.Errorf("%d verdicts recorded", n)
	}
	if m, _ := signedJournal.Load(); len(m) != 1 {
		t.Errorf("journal after a recorded verdict: %d entries", len(m))
	}
	cancel()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("dashboard did not stop")
	}
}

// boardJSON runs board on hosts and decodes its output.
func boardJSON(t *testing.T, args ...string) (signedBoardFull, int) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := run(append([]string{"board"}, args...), nil, &stdout, &stderr)
	var out signedBoardFull
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
		t.Fatalf("%v: %q %q", err, stdout.String(), stderr.String())
	}
	return out, code
}

type signedBoardFull struct {
	Hosts []struct {
		Host, Status, Code, Reason, Pin string
		Counts                          koinon.Counts
		Items                           []koinon.Item
		Pending                         []koinon.PendingWrite
	}
	Unresolved []koinon.Unrouted
	OK         bool
}

// signedOnly writes a hosts file with the signed row alone.
func signedOnly(t *testing.T, row string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "hosts.toml")
	os.WriteFile(p, []byte(row), 0o600)
	return p
}

func TestBoardWaitsForTheNodesOwnItems(t *testing.T) {
	_, node := signedHosts(t, true)
	node.RemoveTarget(cmdOtherPin)
	node.Change(cmdNodePin, func(tg *koinontest.Target) {
		tg.Items["N-1"] = map[string]any{"reference": "N-1", "revision": int64(1), "kind": "fyi", "title": "On the node",
			"state": "active", "commands": []string{}}
		tg.Versions["decisions"]++
	})
	hosts := signedOnly(t, fmt.Sprintf("[[host]]\nlabel = \"studio\"\nnode_pin = %q\n", cmdNodePin))
	out, code := boardJSON(t, "--hosts", hosts, "--timeout", "20s")
	if code != 0 || len(out.Hosts) != 1 || len(out.Hosts[0].Items) != 1 || out.Hosts[0].Counts.Waiting != 1 {
		t.Errorf("exit %d, hosts %+v", code, out.Hosts)
	}
}

func TestBoardShowsAPendingVerdictWithoutAKey(t *testing.T) {
	hosts, _ := signedHosts(t, false)
	e := koinon.Entry{NodePin: cmdNodePin, Node: "studio", FirstSent: 1, Envelope: signed.Envelope{V: signed.Version,
		Target: cmdOtherPin, Op: "decision_verdict", Args: map[string]any{"reference": "S-1", "verdict": "approved"},
		Request: "req-waiting-00000000", Created: 1}}
	if _, err := signedJournal.Reserve(e); err != nil {
		t.Fatal(err)
	}
	out, _ := boardJSON(t, "--hosts", hosts, "--timeout", "20s")
	var found *koinon.PendingWrite
	for _, h := range out.Hosts {
		if h.Host == "studio" {
			for i := range h.Pending {
				found = &h.Pending[i]
			}
		}
	}
	if found == nil || found.Request != e.Request() || found.Target != cmdOtherPin || !strings.Contains(found.Reason, "nothing was sent") {
		t.Errorf("pending verdict on a node without a key: %+v", found)
	}
}

// slowStore answers only when its context ends.
type slowStore struct{}

func (slowStore) Find(ctx context.Context, _, _ string) (string, error) {
	<-ctx.Done()
	return "", ctx.Err()
}
func (slowStore) Add(ctx context.Context, _, _ string, _ []byte) error { return ctx.Err() }

func TestBoardTimeoutBoundsTheKeychain(t *testing.T) {
	hosts, _ := signedHosts(t, true)
	keyStore = slowStore{}
	start := time.Now()
	out, code := boardJSON(t, "--hosts", hosts, "--timeout", "2s")
	if d := time.Since(start); d > 8*time.Second || code != 1 {
		t.Errorf("board took %v, exit %d", d, code)
	}
	got := map[string]string{}
	for _, h := range out.Hosts {
		got[h.Host] = h.Status
	}
	if got["local"] != "connected" || got["studio"] != "connecting" {
		t.Errorf("hosts %v", got)
	}
}

func TestBoardServeRow(t *testing.T) {
	_, node := signedHosts(t, true)
	route := &koinontest.Route{Node: node}
	srv := httptest.NewTLSServer(route)
	t.Cleanup(srv.Close)
	sourceOptions.Open = nil
	sourceOptions.HTTPClient = srv.Client()
	hosts := signedOnly(t, fmt.Sprintf("[[host]]\nlabel = \"remote\"\nnode_pin = %q\nserve = %q\n", cmdNodePin, srv.URL))
	out, code := boardJSON(t, "--hosts", hosts, "--timeout", "20s")
	got := map[string]string{}
	for _, h := range out.Hosts {
		got[h.Host] = h.Status
		if h.Host == "lab-1" && (len(h.Items) != 1 || h.Items[0].Title != "Sign the release") {
			t.Errorf("lab-1 through Serve: %+v", h.Items)
		}
	}
	if code != 0 || got["desk"] != "connected" || got["lab-1"] != "connected" || route.Sessions == 0 {
		t.Errorf("exit %d, hosts %v, sessions %d", code, got, route.Sessions)
	}
}
