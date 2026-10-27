package dashboard

import (
	"bufio"
	"context"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rwcii/koinon-voice/internal/koinon"
)

// fakeHost is a connector with a fixed state that records verdicts.
type fakeHost struct {
	mu       sync.Mutex
	state    koinon.State
	err      error
	verdicts []string
}

func (h *fakeHost) State() koinon.State {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.state
}

func (h *fakeHost) Verdict(ctx context.Context, ref string, rev int64, verdict, note string) (koinon.Verdict, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.err != nil {
		return koinon.Verdict{}, h.err
	}
	h.verdicts = append(h.verdicts, ref+"|"+verdict+"|"+note+"|"+strconv.FormatInt(rev, 10))
	return koinon.Verdict{Revision: rev, Verdict: verdict, Note: note, Via: "mcp:koinon-voice"}, nil
}

const script = `<script>alert(1)</script>`

func hosts() (*fakeHost, *fakeHost) {
	a := &fakeHost{state: koinon.State{Host: "local", Status: koinon.Connected,
		Counts: koinon.Counts{Waiting: 1, OpenFYI: 1},
		Items: []koinon.Item{
			{Reference: "p-1", Kind: "decision", Revision: 2, Title: "Approve " + script, Context: "Context " + script, Recommendation: "Approve it", AuthorName: "codex-synthetic-a4"},
			{Reference: "f-1", Kind: "fyi", Revision: 1, Title: "An fyi", Context: "fyi text"},
		},
		Inbox: []koinon.Message{{Seq: 3, SenderName: "codex-synthetic-a4", Body: "message " + script, CreatedAt: 1793125072521}},
	}}
	b := &fakeHost{state: koinon.State{Host: "devbox", Status: koinon.Disconnected, Code: "maintainer_secret_invalid", Reason: "the maintainer secret is not correct"}}
	return a, b
}

// serve starts the dashboard on a loopback listener and returns it with a client that
// does not follow redirects.
func serve(t *testing.T, hs ...Host) (*Server, *httptest.Server, *http.Client) {
	t.Helper()
	ts := httptest.NewUnstartedServer(nil)
	s, err := New(hs, ts.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	ts.Config.Handler = s.Handler()
	ts.Start()
	t.Cleanup(ts.Close)
	jar, _ := cookiejar.New(nil)
	c := &http.Client{Jar: jar, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	return s, ts, c
}

func get(t *testing.T, c *http.Client, u string) (*http.Response, string) {
	t.Helper()
	resp, err := c.Get(u)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return resp, string(body)
}

func login(t *testing.T, s *Server, c *http.Client) {
	t.Helper()
	resp, _ := get(t, c, s.LoginLink())
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("login: %d", resp.StatusCode)
	}
}

var csrfField = regexp.MustCompile(`name="csrf" value="([0-9a-f]+)"`)

func TestLogin(t *testing.T) {
	a, b := hosts()
	s, ts, c := serve(t, a, b)
	if resp, body := get(t, c, ts.URL+"/"); resp.StatusCode != http.StatusUnauthorized || !strings.Contains(body, "login link") {
		t.Errorf("no session: %d", resp.StatusCode)
	}
	link := s.LoginLink()
	resp, _ := get(t, c, link)
	if resp.StatusCode != http.StatusSeeOther || resp.Header.Get("Location") != "/" {
		t.Fatalf("login %d %s", resp.StatusCode, resp.Header.Get("Location"))
	}
	cookie := resp.Cookies()[0]
	if cookie.Name != CookieName || !cookie.HttpOnly || cookie.SameSite != http.SameSiteStrictMode {
		t.Errorf("cookie %+v", cookie)
	}
	if resp, _ := get(t, c, ts.URL+"/"); resp.StatusCode != http.StatusOK {
		t.Errorf("with the session: %d", resp.StatusCode)
	}
	// The link works once.
	other, _ := cookiejar.New(nil)
	c2 := &http.Client{Jar: other, CheckRedirect: c.CheckRedirect}
	if resp, _ := get(t, c2, link); resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("second use: %d", resp.StatusCode)
	}
	// And only within LinkLifetime.
	start := time.Now()
	s.now = func() time.Time { return start }
	late := s.LoginLink()
	s.now = func() time.Time { return start.Add(LinkLifetime + time.Second) }
	if resp, _ := get(t, c2, late); resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("late use: %d", resp.StatusCode)
	}
	if resp, _ := get(t, c2, ts.URL+"/login?token=bogus"); resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("bogus token: %d", resp.StatusCode)
	}
}

func TestPage(t *testing.T) {
	a, b := hosts()
	s, ts, c := serve(t, a, b)
	login(t, s, c)
	resp, body := get(t, c, ts.URL+"/")
	for _, want := range []string{
		`data-host="local"`, `data-host="devbox"`, "maintainer_secret_invalid", "the maintainer secret is not correct",
		"1 waiting", "Approve &lt;script&gt;", "Context &lt;script&gt;", "Approve it", "codex-synthetic-a4",
		`value="approved"`, `value="declined"`, `value="dismissed"`, "message &lt;script&gt;",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("page lacks %q", want)
		}
	}
	if strings.Contains(body, script) {
		t.Error("agent text is not escaped")
	}
	for k, v := range map[string]string{"X-Frame-Options": "DENY", "X-Content-Type-Options": "nosniff", "Cache-Control": "no-store"} {
		if resp.Header.Get(k) != v {
			t.Errorf("%s %q", k, resp.Header.Get(k))
		}
	}
	if csp := resp.Header.Get("Content-Security-Policy"); !strings.Contains(csp, "default-src 'none'") || !strings.Contains(csp, "frame-ancestors 'none'") {
		t.Errorf("csp %q", csp)
	}
	if resp, _ := get(t, c, ts.URL+"/static/app.js"); resp.StatusCode != http.StatusOK {
		t.Errorf("script: %d", resp.StatusCode)
	}
}

func TestHostAndOrigin(t *testing.T) {
	a, b := hosts()
	s, ts, c := serve(t, a, b)
	login(t, s, c)
	port := ts.Listener.Addr().String()[strings.LastIndex(ts.Listener.Addr().String(), ":"):]
	for _, host := range []string{"evil.example" + port, "localhost" + port, "127.0.0.1:1"} {
		req, _ := http.NewRequest("GET", ts.URL+"/", nil)
		req.Host = host
		resp, err := c.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusForbidden {
			t.Errorf("Host %s: %d", host, resp.StatusCode)
		}
	}
	req, _ := http.NewRequest("GET", ts.URL+"/", nil)
	req.Header.Set("Origin", "http://evil.example")
	if resp, _ := c.Do(req); resp.StatusCode != http.StatusForbidden {
		t.Errorf("foreign origin: %d", resp.StatusCode)
	}
}

func postVerdict(t *testing.T, c *http.Client, ts *httptest.Server, origin string, form url.Values) *http.Response {
	t.Helper()
	req, _ := http.NewRequest("POST", ts.URL+"/verdict", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	resp, err := c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	return resp
}

func TestVerdict(t *testing.T) {
	a, b := hosts()
	s, ts, c := serve(t, a, b)
	login(t, s, c)
	_, body := get(t, c, ts.URL+"/")
	m := csrfField.FindStringSubmatch(body)
	if m == nil {
		t.Fatal("no csrf field")
	}
	origin := "http://" + ts.Listener.Addr().String()
	form := url.Values{"csrf": {m[1]}, "host": {"local"}, "reference": {"p-1"}, "revision": {"2"}, "verdict": {"approved"}, "note": {"yes " + script}}

	for name, tc := range map[string]struct {
		origin string
		form   url.Values
	}{
		"no origin":    {"", form},
		"wrong origin": {"http://evil.example", form},
		"no csrf":      {origin, url.Values{"host": {"local"}, "reference": {"p-1"}, "revision": {"2"}, "verdict": {"approved"}}},
		"wrong csrf":   {origin, url.Values{"csrf": {strings.Repeat("0", 64)}, "host": {"local"}, "reference": {"p-1"}, "revision": {"2"}, "verdict": {"approved"}}},
	} {
		if resp := postVerdict(t, c, ts, tc.origin, tc.form); resp.StatusCode != http.StatusForbidden {
			t.Errorf("%s: %d", name, resp.StatusCode)
		}
	}
	// Without the session cookie.
	bare := &http.Client{CheckRedirect: c.CheckRedirect}
	if resp := postVerdict(t, bare, ts, origin, form); resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("no session: %d", resp.StatusCode)
	}
	if len(a.verdicts) != 0 {
		t.Fatalf("a refused request gave a verdict: %v", a.verdicts)
	}

	resp := postVerdict(t, c, ts, origin, form)
	if resp.StatusCode != http.StatusSeeOther || !strings.Contains(resp.Header.Get("Location"), "notice=recorded") {
		t.Errorf("verdict: %d %s", resp.StatusCode, resp.Header.Get("Location"))
	}
	if len(a.verdicts) != 1 || a.verdicts[0] != "p-1|approved|yes "+script+"|2" {
		t.Errorf("verdicts %q", a.verdicts)
	}
	_, page := get(t, c, ts.URL+resp.Header.Get("Location"))
	if !strings.Contains(page, "Verdict recorded on p-1 (local)") {
		t.Error("no recorded notice")
	}
}

func TestVerdictRefused(t *testing.T) {
	a, b := hosts()
	s, ts, c := serve(t, a, b)
	login(t, s, c)
	_, body := get(t, c, ts.URL+"/")
	csrf := csrfField.FindStringSubmatch(body)[1]
	origin := "http://" + ts.Listener.Addr().String()
	for code, want := range map[string]string{
		"stale_revision": "p-1 on local changed after the page showed it",
		"not_found":      "p-1 on local has left the board",
		"decided":        "p-1 on local already has a verdict",
	} {
		a.mu.Lock()
		a.err = &koinon.Refusal{Code: code}
		a.mu.Unlock()
		resp := postVerdict(t, c, ts, origin, url.Values{"csrf": {csrf}, "host": {"local"}, "reference": {"p-1"}, "revision": {"1"}, "verdict": {"approved"}})
		_, page := get(t, c, ts.URL+resp.Header.Get("Location"))
		if !strings.Contains(page, want) {
			t.Errorf("%s: page lacks %q", code, want)
		}
		// The item shows as it is now.
		if code == "stale_revision" && !strings.Contains(page, `name="revision" value="2"`) {
			t.Error("the page does not show the current revision")
		}
	}
	b.err = koinon.ErrDisconnected
	resp := postVerdict(t, c, ts, origin, url.Values{"csrf": {csrf}, "host": {"devbox"}, "reference": {"p-1"}, "revision": {"1"}, "verdict": {"approved"}})
	if _, page := get(t, c, ts.URL+resp.Header.Get("Location")); !strings.Contains(page, "devbox is not connected") {
		t.Error("no disconnected notice")
	}
	resp = postVerdict(t, c, ts, origin, url.Values{"csrf": {csrf}, "host": {"local"}, "reference": {"p-1"}, "revision": {"1"}, "verdict": {"maybe"}})
	if !strings.Contains(resp.Header.Get("Location"), "notice=invalid_verdict") {
		t.Errorf("bad verdict: %s", resp.Header.Get("Location"))
	}
}

func TestEvents(t *testing.T) {
	a, b := hosts()
	s, ts, c := serve(t, a, b)
	login(t, s, c)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, "GET", ts.URL+"/events", nil)
	resp, err := c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	r := bufio.NewReader(resp.Body)
	if line, _ := r.ReadString('\n'); line != ": open\n" {
		t.Fatalf("first line %q", line)
	}
	r.ReadString('\n')
	s.Changed()
	var got strings.Builder
	for !strings.Contains(got.String(), "data: {}\n") {
		line, err := r.ReadString('\n')
		if err != nil {
			t.Fatal(err)
		}
		got.WriteString(line)
	}
	if !strings.Contains(got.String(), "event: change\n") || strings.Contains(got.String(), "p-1") {
		t.Errorf("events %q", got.String())
	}
	_, board := get(t, c, ts.URL+"/board")
	if !strings.Contains(board, `data-host="local"`) || strings.Contains(board, "<html") {
		t.Error("the board fragment is not the board")
	}
}

func TestCheckListen(t *testing.T) {
	for _, addr := range []string{"127.0.0.1:0", "[::1]:47680", "127.0.0.2:1"} {
		if err := CheckListen(addr); err != nil {
			t.Errorf("%s: %v", addr, err)
		}
	}
	for _, addr := range []string{"0.0.0.0:47680", "localhost:47680", "192.168.1.2:47680", "[::]:1", "127.0.0.1"} {
		if err := CheckListen(addr); err == nil {
			t.Errorf("%s accepted", addr)
		}
	}
}
