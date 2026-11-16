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
	"strings"
	"testing"
	"time"

	"github.com/rwcii/koinon-voice/internal/koinon"
)

const script = `<script>alert(1)</script>`

// rows are a signed row with a connected computer that holds agent text to escape, an fyi
// item and an inbox message, and a disconnected one.
func rows() *fakeSigned {
	return &fakeSigned{states: []koinon.State{
		{Host: "local", Pin: pinA, Node: "studio", Status: koinon.Connected, Loaded: true,
			Counts: koinon.Counts{Waiting: 1, OpenFYI: 1},
			Items: []koinon.Item{
				{Reference: "p-1", Kind: "decision", Revision: 2, Title: "Approve " + script, Context: "Context " + script, Recommendation: "Approve it", AuthorName: "codex-synthetic-a4", State: "active"},
				{Reference: "f-1", Kind: "fyi", Revision: 1, Title: "An fyi", Context: "fyi text", State: "active"},
			},
			Inbox: []koinon.Message{{Seq: 3, SenderName: "codex-synthetic-a4", Body: "message " + script, CreatedAt: 1793125072521}},
		},
		{Host: "devbox", Pin: pinB, Node: "studio", Status: koinon.Disconnected, Code: "computer_offline", Reason: "the computer is offline"},
	}, result: koinon.VerdictResult{State: koinon.Recorded}}
}

// serve starts the dashboard on a loopback listener and returns it with a client that
// does not follow redirects.
func serve(t *testing.T, sources ...SignedSource) (*Server, *httptest.Server, *http.Client) {
	t.Helper()
	ts := httptest.NewUnstartedServer(nil)
	s, err := New(sources, ts.Listener.Addr().String())
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
	a := rows()
	s, ts, c := serve(t, a)
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
	a := rows()
	s, ts, c := serve(t, a)
	login(t, s, c)
	resp, body := get(t, c, ts.URL+"/")
	for _, want := range []string{
		`data-pin="` + pinA + `"`, `data-pin="` + pinB + `"`, "computer_offline", "the computer is offline",
		"1 waiting", "Approve &lt;script&gt;", "Context &lt;script&gt;", "Approve it", "codex-synthetic-a4",
		`value="approved"`, `value="declined"`, `value="acknowledged"`, "message &lt;script&gt;",
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
	a := rows()
	s, ts, c := serve(t, a)
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
	a := rows()
	s, ts, c := serve(t, a)
	login(t, s, c)
	_, body := get(t, c, ts.URL+"/")
	m := csrfField.FindStringSubmatch(body)
	if m == nil {
		t.Fatal("no csrf field")
	}
	origin := "http://" + ts.Listener.Addr().String()
	fields := func(csrf ...string) url.Values {
		v := url.Values{"source": {"0"}, "pin": {pinA}, "reference": {"p-1"}, "revision": {"2"}, "digest": {"x"},
			"verdict": {"approved"}, "note": {"yes " + script}, "intent": {"intent-1"}}
		if len(csrf) > 0 {
			v.Set("csrf", csrf[0])
		}
		return v
	}
	form := fields(m[1])

	for name, tc := range map[string]struct {
		origin string
		form   url.Values
	}{
		"no origin":    {"", form},
		"wrong origin": {"http://evil.example", form},
		"no csrf":      {origin, fields()},
		"wrong csrf":   {origin, fields(strings.Repeat("0", 64))},
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
	if len(a.verdicts) != 1 || a.verdicts[0].Reference != "p-1" || a.verdicts[0].Note != "yes "+script || a.verdicts[0].Revision != 2 {
		t.Errorf("verdicts %+v", a.verdicts)
	}
	_, page := get(t, c, ts.URL+resp.Header.Get("Location"))
	if !strings.Contains(page, "Verdict recorded on p-1 (local)") {
		t.Error("no recorded notice")
	}
}

func TestEvents(t *testing.T) {
	a := rows()
	s, ts, c := serve(t, a)
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
	if !strings.Contains(board, `data-pin="`+pinA+`"`) || strings.Contains(board, "<html") {
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
