package summarize

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rwcii/koinon-voice/internal/sidecar"
)

// The test binary is also a synthetic Apple helper: with fakeApple set it serves summarize
// and speak instead of running the tests (WDR 006: no model, no audio).
const fakeApple = "KOINON_VOICE_FAKE_APPLE"

func TestMain(m *testing.M) {
	if mode := os.Getenv(fakeApple); mode != "" {
		fakeAppleHelper(mode)
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func fakeAppleHelper(mode string) {
	out := bufio.NewWriter(os.Stdout)
	var mu sync.Mutex
	send := func(m sidecar.Message) {
		mu.Lock()
		defer mu.Unlock()
		sidecar.WriteMessage(out, m)
		out.Flush()
	}
	data := func(v any) json.RawMessage {
		b, _ := json.Marshal(v)
		return b
	}
	send(sidecar.Message{Type: "hello", Protocol: sidecar.Version, Helper: "fake-apple", Capabilities: []string{"speak", "summarize"}})
	in := bufio.NewReader(os.Stdin)
	inputs := map[uint64][]string{}
	for {
		m, err := sidecar.ReadMessage(in)
		if err != nil {
			return
		}
		switch {
		case m.Type == "request" && m.Method == "summarize":
			var p struct{ Instructions, Prompt string }
			json.Unmarshal(m.Params, &p)
			switch {
			case mode == "unavailable":
				send(sidecar.Message{Type: "error", ID: m.ID, Code: "unavailable", Message: "appleIntelligenceNotEnabled"})
			case mode == "refused":
				send(sidecar.Message{Type: "error", ID: m.ID, Code: "refused", Message: "guardrailViolation"})
			case strings.Contains(p.Prompt, "Title: hang"):
				// No answer: the bridge's deadline kills this helper.
			case p.Instructions != Instructions:
				send(sidecar.Message{Type: "error", ID: m.ID, Code: "bad_request", Message: "not chunk 02's instructions"})
			default:
				send(sidecar.Message{Type: "event", ID: m.ID, Event: "delta", Data: data(map[string]string{"text": "Codex asks you "})})
				send(sidecar.Message{Type: "event", ID: m.ID, Event: "delta", Data: data(map[string]string{"text": "to merge."})})
				send(sidecar.Message{Type: "result", ID: m.ID, Data: data(map[string]any{"text": "Codex asks you to merge.", "input_tokens": 120, "output_tokens": 8})})
			}
		case m.Type == "request" && m.Method == "speak":
			inputs[m.ID] = nil
		case m.Type == "input" && m.End:
			send(sidecar.Message{Type: "result", ID: m.ID, Data: data(map[string]any{"voice": "fake-voice", "audio_seconds": 1.5, "said": inputs[m.ID]})})
			delete(inputs, m.ID)
		case m.Type == "input":
			var p struct{ Text string }
			json.Unmarshal(m.Params, &p)
			if len(inputs[m.ID]) == 0 {
				send(sidecar.Message{Type: "event", ID: m.ID, Event: "first_audio"})
			}
			inputs[m.ID] = append(inputs[m.ID], p.Text)
			if mode == "record" {
				f, _ := os.OpenFile(os.Getenv("FAKE_APPLE_LOG"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
				f.WriteString(p.Text + "\n")
				f.Close()
			}
		}
	}
}

func fakeLauncher(t *testing.T, mode string) *Launcher {
	t.Helper()
	t.Setenv(fakeApple, mode)
	l := &Launcher{Path: os.Args[0]}
	t.Cleanup(l.Close)
	return l
}

func TestAppleSummary(t *testing.T) {
	var deltas []string
	s, err := (&Apple{Helper: fakeLauncher(t, "ok")}).Stream(context.Background(), testItem, func(d string) { deltas = append(deltas, d) })
	if err != nil {
		t.Fatal(err)
	}
	if s.Text != "Codex asks you to merge." || strings.Join(deltas, "|") != "Codex asks you |to merge." {
		t.Errorf("summary %q, deltas %q", s.Text, deltas)
	}
	if !s.UsageKnown || s.InputTokens != 120 || s.OutputTokens != 8 || s.FirstToken <= 0 || s.Total < s.FirstToken {
		t.Errorf("summary %+v", s)
	}
}

func TestAppleFailures(t *testing.T) {
	for mode, want := range map[string]error{"unavailable": ErrUnavailable, "refused": ErrRefused} {
		t.Run(mode, func(t *testing.T) {
			_, err := (&Apple{Helper: fakeLauncher(t, mode)}).Summarize(context.Background(), testItem)
			if !errors.Is(err, want) {
				t.Fatalf("error %v, want %v", err, want)
			}
		})
	}
	if _, err := (&Apple{Helper: &Launcher{}}).Summarize(context.Background(), testItem); err == nil || !strings.Contains(err.Error(), "no Apple helper") {
		t.Errorf("no helper: %v", err)
	}
	if _, err := Build(context.Background(), Row{Name: "apple", Adapter: "apple", Model: "system", Local: true}, Env{}); err == nil {
		t.Error("an apple engine was built without a helper")
	}
}

// A request past its deadline kills the helper; the next request starts a new one.
func TestAppleRestartsAfterDeadline(t *testing.T) {
	a := &Apple{Helper: fakeLauncher(t, "ok")}
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	hang := testItem
	hang.Title = "hang"
	if _, err := a.Summarize(ctx, hang); !errors.Is(err, sidecar.ErrDeadline) {
		t.Fatalf("error %v", err)
	}
	if _, err := a.Summarize(context.Background(), testItem); err != nil {
		t.Fatalf("after restart: %v", err)
	}
}

type streamingEngine struct{ parts []string }

func (s streamingEngine) Summarize(ctx context.Context, item Item) (Summary, error) {
	return s.Stream(ctx, item, nil)
}

func (s streamingEngine) Stream(ctx context.Context, item Item, onText func(string)) (Summary, error) {
	t := newTimer(nil, onText)
	for _, p := range s.parts {
		t.add(p)
		time.Sleep(30 * time.Millisecond)
	}
	return t.finish()
}

func TestRunSpeaks(t *testing.T) {
	log := t.TempDir() + "/said"
	t.Setenv("FAKE_APPLE_LOG", log)
	voice := &Voice{Helper: fakeLauncher(t, "record")}
	engine := streamingEngine{parts: []string{"Codex asks you to merge. ", "It recommends ", "merging."}}
	rows := []Row{{Name: "local", Adapter: "openai", Model: "m", Local: true}, {Name: "remote", Adapter: "openai", Model: "m", Provider: "OpenRouter"}}
	engines := []Configured{{Row: rows[0], Engine: engine}, {Row: rows[1], Engine: engine}}
	results, _ := Run(context.Background(), engines, []Fixture{{Name: "one", Item: testItem}}, Options{Timeout: time.Second, Voice: voice, Label: true})
	for _, r := range results {
		if r.Err != nil || r.SpeechErr != nil || r.Spoken == nil {
			t.Fatalf("%s: %+v", r.Row.Name, r)
		}
		// The first sentence was spoken while the engine was still streaming.
		if r.Spoken.FirstWord <= 0 || r.Spoken.FirstWord >= r.Summary.Total || r.Spoken.Synthesis < 0 || r.Spoken.Voice != "fake-voice" {
			t.Errorf("%s: spoken %+v, total %v", r.Row.Name, r.Spoken, r.Summary.Total)
		}
	}
	said, _ := os.ReadFile(log)
	want := "Codex asks you to merge.\nIt recommends merging.\n" +
		"Codex asks you to merge.\nIt recommends merging.\nSummary by OpenRouter.\n"
	if string(said) != want {
		t.Errorf("said %q, want %q", said, want)
	}

	var b strings.Builder
	WriteScorecard(&b, results, rows)
	if !strings.Contains(b.String(), "| local | 1 | 0 | 0/1 | ") || strings.Count(b.String(), " ms |") < 10 {
		t.Errorf("scorecard:\n%s", b.String())
	}
}

func TestRunOrder(t *testing.T) {
	rows := []Row{{Name: "a"}, {Name: "b"}, {Name: "c"}}
	var engines []Configured
	for _, r := range rows {
		engines = append(engines, Configured{Row: r, Engine: stubEngine{text: "x."}})
	}
	reverse := func(n int, swap func(i, j int)) {
		for i := 0; i < n/2; i++ {
			swap(i, n-1-i)
		}
	}
	results, _ := Run(context.Background(), engines, []Fixture{{Name: "one", Item: testItem}}, Options{Timeout: time.Second, Order: reverse})
	var got []string
	for _, r := range results {
		got = append(got, r.Row.Name)
	}
	if strings.Join(got, ",") != "c,b,a" {
		t.Errorf("order %v", got)
	}
}

func TestSpeechFailureKeepsSummary(t *testing.T) {
	voice := &Voice{Helper: &Launcher{}} // no helper configured
	results, _ := Run(context.Background(), []Configured{{Row: Row{Name: "a", Local: true}, Engine: stubEngine{text: "Codex asks."}}},
		[]Fixture{{Name: "one", Item: testItem}}, Options{Timeout: time.Second, Voice: voice})
	if results[0].Err != nil || results[0].SpeechErr == nil || results[0].Spoken != nil {
		t.Fatalf("%+v", results[0])
	}
	var b strings.Builder
	WriteScorecard(&b, results, []Row{{Name: "a", Local: true}})
	if !strings.Contains(b.String(), "speech failed: no Apple helper") {
		t.Errorf("scorecard:\n%s", b.String())
	}
}

func TestRunAfter(t *testing.T) {
	engines := []Configured{{Row: Row{Name: "a"}, Engine: stubEngine{text: "x."}}, {Row: Row{Name: "b"}, Engine: stubEngine{text: "y."}}}
	items := []Fixture{{Name: "one", Item: testItem}, {Name: "two", Item: testItem}}
	seen := 0
	stop := errors.New("stop")
	results, err := Run(context.Background(), engines, items, Options{Timeout: time.Second, After: func(r *Result) error {
		seen++
		r.Rating = seen
		if seen == 3 {
			return stop
		}
		return nil
	}})
	if !errors.Is(err, stop) || len(results) != 3 || results[0].Rating != 1 || results[2].Rating != 3 {
		t.Fatalf("%v, %+v", err, results)
	}
}
