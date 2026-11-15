package dashboard

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/rwcii/koinon-voice/internal/koinon"
)

const (
	pinA = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	pinB = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
)

// fakeSigned is a signed row with fixed states that records its calls.
type fakeSigned struct {
	states []koinon.State
	result koinon.VerdictResult

	mu        sync.Mutex
	verdicts  []koinon.VerdictRequest
	dismissed []string
}

func (f *fakeSigned) States() []koinon.State { return f.states }
func (f *fakeSigned) Verdict(_ context.Context, r koinon.VerdictRequest) koinon.VerdictResult {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.verdicts = append(f.verdicts, r)
	return f.result
}
func (f *fakeSigned) NewIntent() string { return "intent-1" }
func (f *fakeSigned) Dismiss(request string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.dismissed = append(f.dismissed, request)
	if request != "req-final" {
		return errors.New("no such record")
	}
	return nil
}

var signedItem = koinon.Item{Reference: "P-1", Revision: 3, Kind: "task", Title: "Do it", Context: "ctx",
	Commands: []string{"make", "test"}, State: "active"}

func signedRows() (*fakeSigned, *fakeSigned) {
	a := &fakeSigned{states: []koinon.State{
		{Host: "desk", Pin: pinA, Node: "studio", Status: koinon.Connected},
		{Host: "lab-1", Pin: pinB, Node: "studio", Status: koinon.Connected, Items: []koinon.Item{signedItem},
			Pending: []koinon.PendingWrite{{Request: "req-final", Reference: "P-0", Verdict: "approved", Final: true, Reason: "read the record"}}},
	}, result: koinon.VerdictResult{State: koinon.Recorded}}
	// Another node shows a computer with the same name and reference, by another pin.
	b := &fakeSigned{states: []koinon.State{{Host: "lab-1", Pin: pinA, Node: "other", Status: koinon.Connected, Items: []koinon.Item{signedItem}}},
		result: koinon.VerdictResult{State: koinon.Recorded}}
	return a, b
}

func serveSigned(t *testing.T, sources ...SignedSource) (*Server, *httptest.Server, *http.Client, string, string) {
	t.Helper()
	s, ts, c := serve(t)
	s.SetSigned(sources)
	login(t, s, c)
	_, body := get(t, c, ts.URL+"/")
	m := csrfField.FindStringSubmatch(body)
	if m == nil {
		t.Fatal("no csrf field")
	}
	return s, ts, c, m[1], body
}

func post(t *testing.T, c *http.Client, ts *httptest.Server, path string, form url.Values) *http.Response {
	t.Helper()
	req, _ := http.NewRequest("POST", ts.URL+path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", "http://"+ts.Listener.Addr().String())
	resp, err := c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	return resp
}

func TestSignedPageShowsTheDigestAndKindVerdicts(t *testing.T) {
	a, b := signedRows()
	_, _, _, _, body := serveSigned(t, a, b)
	d, _ := koinon.SnapshotDigest(signedItem)
	for _, want := range []string{`name="digest" value="` + d + `"`, `name="intent" value="intent-1"`, `name="pin" value="` + pinB + `"`,
		`value="completed"`, `value="feedback"`, "through studio", "Remove this local record", "nothing is sent"} {
		if !strings.Contains(body, want) {
			t.Errorf("page lacks %q", want)
		}
	}
	if strings.Contains(body, `value="approved">approved`) {
		t.Error("a task shows approve")
	}
}

func TestSignedVerdictRoutesByRowAndPin(t *testing.T) {
	a, b := signedRows()
	_, ts, c, csrf, _ := serveSigned(t, a, b)
	d, _ := koinon.SnapshotDigest(signedItem)
	form := url.Values{"csrf": {csrf}, "source": {"0"}, "pin": {pinB}, "reference": {"P-1"}, "revision": {"3"},
		"digest": {d}, "verdict": {"completed"}, "note": {"done"}, "intent": {"intent-1"}}
	resp := post(t, c, ts, "/verdict", form)
	if !strings.Contains(resp.Header.Get("Location"), "notice=recorded") {
		t.Errorf("verdict: %s", resp.Header.Get("Location"))
	}
	want := koinon.VerdictRequest{Pin: pinB, Reference: "P-1", Revision: 3, Digest: d, Verdict: "completed", Note: "done", Intent: "intent-1"}
	if len(a.verdicts) != 1 || a.verdicts[0] != want || len(b.verdicts) != 0 {
		t.Errorf("a %+v b %+v", a.verdicts, b.verdicts)
	}
	// A pin that the named row does not show reaches no row.
	for _, f := range []url.Values{{"source": {"1"}, "pin": {pinB}}, {"source": {"0"}, "pin": {""}}, {"source": {"7"}, "pin": {pinB}}} {
		g := url.Values{}
		for k, v := range form {
			g[k] = v
		}
		for k, v := range f {
			g[k] = v
		}
		resp := post(t, c, ts, "/verdict", g)
		if !strings.Contains(resp.Header.Get("Location"), "notice=invalid_request") {
			t.Errorf("%v: %s", f, resp.Header.Get("Location"))
		}
	}
	if len(a.verdicts) != 1 || len(b.verdicts) != 0 {
		t.Errorf("a misrouted verdict reached a row: a %d b %d", len(a.verdicts), len(b.verdicts))
	}
}

func TestSignedVerdictNotices(t *testing.T) {
	for want, res := range map[string]koinon.VerdictResult{
		"not_run":          {State: koinon.NotRun, Code: "computer_offline"},
		"unknown":          {State: koinon.Unknown},
		"pending":          {State: koinon.Unknown, Pending: true},
		"final_unknown":    {State: koinon.Unknown, Final: true},
		"snapshot_changed": {State: koinon.Refused, Code: "snapshot_changed", Local: true},
		"stale_revision":   {State: koinon.Refused, Code: "stale_revision", Details: []byte(`{"current_revision":987654321}`)},
	} {
		a, _ := signedRows()
		a.result = res
		_, ts, c, csrf, _ := serveSigned(t, a)
		resp := post(t, c, ts, "/verdict", url.Values{"csrf": {csrf}, "source": {"0"}, "pin": {pinB}, "reference": {"P-1"},
			"revision": {"3"}, "digest": {"x"}, "verdict": {"completed"}})
		loc := resp.Header.Get("Location")
		if !strings.Contains(loc, "notice="+want) {
			t.Errorf("%s: %s", want, loc)
			continue
		}
		_, page := get(t, c, ts.URL+loc)
		if strings.Contains(page, "Koinon did not record the verdict") && want != "stale_revision" {
			t.Errorf("%s has no text of its own", want)
		}
		if want == "stale_revision" && !strings.Contains(page, "987654321") {
			t.Error("the refusal's details are not shown")
		}
	}
}

func TestSignedDismissSendsNoVerdict(t *testing.T) {
	a, _ := signedRows()
	_, ts, c, csrf, _ := serveSigned(t, a)
	if resp := post(t, c, ts, "/dismiss", url.Values{"source": {"0"}, "pin": {pinB}, "request": {"req-final"}}); resp.StatusCode != http.StatusForbidden {
		t.Errorf("dismiss without csrf: %d", resp.StatusCode)
	}
	resp := post(t, c, ts, "/dismiss", url.Values{"csrf": {csrf}, "source": {"0"}, "pin": {pinB}, "request": {"req-final"}})
	if !strings.Contains(resp.Header.Get("Location"), "notice=dismissed") || len(a.dismissed) != 1 || len(a.verdicts) != 0 {
		t.Errorf("dismiss: %s %v %v", resp.Header.Get("Location"), a.dismissed, a.verdicts)
	}
}

func TestSignedPageShowsALearnedOutcome(t *testing.T) {
	a, _ := signedRows()
	a.states[1].Pending = append(a.states[1].Pending, koinon.PendingWrite{Request: "req-done", Reference: "P-2", Verdict: "completed",
		Done: true, State: koinon.Refused, Code: "stale_revision", Detail: `{"current_revision":9}`})
	_, _, _, _, body := serveSigned(t, a)
	for _, want := range []string{"refused (<code>stale_revision</code>), learned after its answer was lost", "current_revision", "Remove this notice"} {
		if !strings.Contains(body, want) {
			t.Errorf("page lacks %q", want)
		}
	}
}

func TestSignedRepeatShowsItsResult(t *testing.T) {
	for want, res := range map[string]koinon.VerdictResult{
		"Verdict recorded on P-1": {State: koinon.Recorded, Repeat: true},
		"can no longer be asked":  {State: koinon.Unknown, Final: true, Repeat: true},
		"stale_revision":          {State: koinon.Refused, Code: "stale_revision", Repeat: true},
		"answer did not arrive":   {State: koinon.Unknown, Repeat: true},
	} {
		a, _ := signedRows()
		a.result = res
		_, ts, c, csrf, _ := serveSigned(t, a)
		resp := post(t, c, ts, "/verdict", url.Values{"csrf": {csrf}, "source": {"0"}, "pin": {pinB}, "reference": {"P-1"},
			"revision": {"3"}, "digest": {"x"}, "verdict": {"completed"}, "intent": {"intent-1"}})
		_, page := get(t, c, ts.URL+resp.Header.Get("Location"))
		if !strings.Contains(page, "This form was already sent; nothing new was sent.") || !strings.Contains(page, want) {
			t.Errorf("repeat of %+v does not show %q", res, want)
		}
	}
}
