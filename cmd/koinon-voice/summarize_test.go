package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rwcii/koinon-voice/internal/summarize"
)

type fakeKeys map[string]string

func (k fakeKeys) Key(ctx context.Context, provider string) (string, error) {
	if key, ok := k[provider]; ok {
		return key, nil
	}
	return "", fmt.Errorf("no key for %s: Keychain item koinon-voice-%s not found", provider, provider)
}

// server answers every chat completion with the given text.
func server(t *testing.T, text string) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprintf(w, "data: {\"choices\":[{\"delta\":{\"content\":%q}}]}\n\n", text)
		fmt.Fprint(w, "data: {\"choices\":[],\"usage\":{\"prompt_tokens\":10,\"completion_tokens\":5}}\n\ndata: [DONE]\n\n")
	}))
	t.Cleanup(srv.Close)
	return srv.URL + "/v1"
}

func setup(t *testing.T) (dir, engines string) {
	t.Helper()
	dir = t.TempDir()
	item := `{"item":{"reference":"F-1","title":"Merge pull request 12","author_family":"codex","author_name":"codex-1"},` +
		`"key":{"asker":[["codex"]],"ask":[["merge"]]}}`
	if err := os.WriteFile(filepath.Join(dir, "f-1.json"), []byte(item), 0o600); err != nil {
		t.Fatal(err)
	}
	engines = filepath.Join(dir, "engines.toml")
	body := fmt.Sprintf(`voice = "file-voice"

[[engine]]
name = "local-a"
adapter = "openai"
base_url = %q
model = "a"
local = true

[[engine]]
name = "local-b"
adapter = "openai"
base_url = %q
model = "b"
local = true

[[engine]]
name = "keyless"
adapter = "openai"
base_url = "http://127.0.0.1:1/v1"
model = "c"
provider = "OpenRouter"
key = "openrouter"
`, server(t, "Codex asks you to merge."), server(t, "Codex wants a merge."))
	if err := os.WriteFile(engines, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	keys = fakeKeys{}
	t.Cleanup(func() { keys = fakeKeys{} })
	return dir, engines
}

// fakeSpeaker records what would be spoken; fail makes every utterance fail.
type fakeSpeaker struct {
	mu    sync.Mutex
	said  []string
	fail  error
	voice string
}

func (f *fakeSpeaker) Start(ctx context.Context) (summarize.Speech, error) {
	if f.fail != nil {
		return nil, f.fail
	}
	return &fakeSpeech{f: f}, nil
}

type fakeSpeech struct {
	f     *fakeSpeaker
	text  strings.Builder
	first time.Time
}

func (s *fakeSpeech) Write(delta string) {
	if s.first.IsZero() {
		s.first = time.Now()
	}
	s.text.WriteString(delta)
}

func (s *fakeSpeech) Finish() (summarize.Spoken, error) {
	s.f.mu.Lock()
	defer s.f.mu.Unlock()
	s.f.said = append(s.f.said, s.text.String())
	return summarize.Spoken{FirstSentence: s.first, FirstAudio: s.first.Add(time.Millisecond), Voice: "fake"}, nil
}

func stubSpeak(t *testing.T) *fakeSpeaker {
	t.Helper()
	f := &fakeSpeaker{}
	old := newSpeaker
	newSpeaker = func(_ *summarize.Launcher, voice string) summarize.Speaker {
		f.voice = voice
		return f
	}
	t.Cleanup(func() { newSpeaker = old })
	return f
}

func TestSummarizeFixtures(t *testing.T) {
	dir, engines := setup(t)
	said := stubSpeak(t)
	out := filepath.Join(dir, "scorecard.md")
	var stdout, stderr bytes.Buffer
	code := run([]string{"summarize", "--engines", engines, "--fixtures", dir, "--out", out, "--speak"}, nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "keyless: no key for openrouter") {
		t.Errorf("stderr %q", stderr.String())
	}
	lines := stdout.String()
	for _, want := range []string{"F-1\tlocal-a\t", "Codex asks you to merge.", "F-1\tlocal-b\t", "F-1\tkeyless\tfailed: no key"} {
		if !strings.Contains(lines, want) {
			t.Errorf("stdout lacks %q:\n%s", want, lines)
		}
	}
	if len(said.said) != 2 || !strings.Contains(lines, "first spoken word") {
		t.Errorf("spoke %q\n%s", said.said, lines)
	}
	card, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(card), "| F-1 | local-a | ok | ok | none | none | none |") {
		t.Errorf("scorecard:\n%s", card)
	}
}

// The engines file's voice speaks summaries unless --voice names another.
func TestSummarizeVoice(t *testing.T) {
	for flag, want := range map[string]string{"": "file-voice", "flag-voice": "flag-voice"} {
		dir, engines := setup(t)
		said := stubSpeak(t)
		args := []string{"summarize", "--engines", engines, "--fixtures", dir, "--out", filepath.Join(dir, "card.md"), "--speak"}
		if flag != "" {
			args = append(args, "--voice", flag)
		}
		var stdout, stderr bytes.Buffer
		if code := run(args, nil, &stdout, &stderr); code != 0 {
			t.Fatalf("exit %d: %s", code, stderr.String())
		}
		if said.voice != want {
			t.Errorf("--voice %q: spoke with %q, want %q", flag, said.voice, want)
		}
	}
}

func TestSummarizeBlind(t *testing.T) {
	dir, engines := setup(t)
	said := stubSpeak(t)
	old := shuffle
	shuffle = func(n int, swap func(i, j int)) { // local-b is A
		if n > 1 {
			swap(0, 1)
		}
	}
	t.Cleanup(func() { shuffle = old })

	var stdout, stderr bytes.Buffer
	stdin := strings.NewReader("9\n2\n5\n")
	code := run([]string{"summarize", "--engines", engines, "--fixtures", dir, "--engine", "local-a,local-b", "--blind", "--speak"}, stdin, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	got := stdout.String()
	blindPart := got[:strings.Index(got, "Engines:")]
	if strings.Contains(blindPart, "local-a") || strings.Contains(blindPart, "local-b") {
		t.Errorf("engine names shown before rating:\n%s", blindPart)
	}
	for _, want := range []string{"A: Codex wants a merge.", "B: Codex asks you to merge.", "F-1 A = local-b", "F-1 B = local-a",
		"| local-a | 1 | 0 | 1/1 |", "| 5.0 (1) |", "| 2.0 (1) |"} {
		if !strings.Contains(got, want) {
			t.Errorf("output lacks %q:\n%s", want, got)
		}
	}
	// Each summary is heard once, in the shuffled order, with no provider label.
	if strings.Join(said.said, "|") != "Codex wants a merge.|Codex asks you to merge." {
		t.Errorf("spoke %q", said.said)
	}

	// A summary whose playback failed is not shown or rated, and the next is.
	said.fail = errors.New("no audio device")
	stdout.Reset()
	if code := run([]string{"summarize", "--engines", engines, "--fixtures", dir, "--engine", "local-a,local-b", "--blind", "--speak"}, strings.NewReader("5\n5\n"), &stdout, &stderr); code != 0 {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	got = stdout.String()
	if !strings.Contains(got, "A: playback failed (no audio device); not rated.") || strings.Contains(got, "Rating for") ||
		strings.Contains(got, "A: Codex") || strings.Contains(got, "(1) |") {
		t.Errorf("after a playback failure:\n%s", got)
	}
	said.fail = nil

	// q stops rating and keeps the ratings so far.
	stdout.Reset()
	if code := run([]string{"summarize", "--engines", engines, "--fixtures", dir, "--engine", "local-a,local-b", "--blind"}, strings.NewReader("4\nq\n"), &stdout, &stderr); code != 0 {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	if got := stdout.String(); !strings.Contains(got, "F-1 A = local-b") || strings.Contains(got, "F-1 B =") || !strings.Contains(got, "| 4.0 (1) |") {
		t.Errorf("after q:\n%s", got)
	}

	// Ratings that end early fail the run instead of writing a partial scorecard.
	stdout.Reset()
	if code := run([]string{"summarize", "--engines", engines, "--fixtures", dir, "--engine", "local-a", "--blind"}, strings.NewReader(""), &stdout, &stderr); code != 1 {
		t.Errorf("exit %d", code)
	}
}

func TestSummarizeUsage(t *testing.T) {
	dir, engines := setup(t)
	for _, args := range [][]string{
		{"summarize"},
		{"summarize", "--fixtures", dir, "--item", "x.json"},
		{"summarize", "--fixtures", dir, "extra"},
	} {
		var stdout, stderr bytes.Buffer
		if code := run(args, nil, &stdout, &stderr); code != 2 {
			t.Errorf("%q: exit %d", args, code)
		}
	}
	for _, args := range [][]string{
		{"summarize", "--engines", engines, "--fixtures", t.TempDir()},
		{"summarize", "--engines", filepath.Join(dir, "missing.toml"), "--fixtures", dir},
		{"summarize", "--engines", engines, "--fixtures", dir, "--engine", "nobody"},
	} {
		var stdout, stderr bytes.Buffer
		if code := run(args, nil, &stdout, &stderr); code != 1 || stderr.Len() == 0 {
			t.Errorf("%q: exit %d stderr %q", args, code, stderr.String())
		}
	}
}

func TestSummarizeItem(t *testing.T) {
	dir, engines := setup(t)
	item := filepath.Join(dir, "exported.json")
	if err := os.WriteFile(item, []byte(`{"reference":"K-9","title":"A real item"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if code := run([]string{"summarize", "--engines", engines, "--item", item, "--engine", "local-a"}, nil, &stdout, &stderr); code != 0 {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "| K-9 | local-a | review | review | review | review | review |") {
		t.Errorf("stdout:\n%s", stdout.String())
	}
}

func TestSpeakFailureIsReported(t *testing.T) {
	dir, engines := setup(t)
	stubSpeak(t).fail = errors.New("no audio device")
	var stdout, stderr bytes.Buffer
	if code := run([]string{"summarize", "--engines", engines, "--fixtures", dir, "--engine", "local-a", "--speak"}, nil, &stdout, &stderr); code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !strings.Contains(stdout.String(), "speech failed: no audio device") {
		t.Errorf("stdout %q", stdout.String())
	}
}

func TestSpeakLabelsExternal(t *testing.T) {
	dir, _ := setup(t)
	said := stubSpeak(t)
	engines := filepath.Join(dir, "external.toml")
	body := fmt.Sprintf(`[[engine]]
name = "remote"
adapter = "openai"
base_url = %q
model = "r"
provider = "OpenRouter"
key = "openrouter"

[[engine]]
name = "local"
adapter = "openai"
base_url = %q
model = "l"
local = true
`, server(t, "Codex asks you to merge."), server(t, "Codex wants a merge."))
	if err := os.WriteFile(engines, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	keys = fakeKeys{"openrouter": "synthetic-key"}
	var stdout, stderr bytes.Buffer
	if code := run([]string{"summarize", "--engines", engines, "--fixtures", dir, "--speak"}, nil, &stdout, &stderr); code != 0 {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	// The label follows the summary, so it does not stand in for the first spoken word.
	want := []string{"Codex asks you to merge. Summary by OpenRouter.", "Codex wants a merge."}
	if strings.Join(said.said, "|") != strings.Join(want, "|") {
		t.Errorf("spoke %q, want %q", said.said, want)
	}
	for _, w := range []string{"F-1\tremote\texternal: OpenRouter\t", "F-1\tlocal\tlocal\t"} {
		if !strings.Contains(stdout.String(), w) {
			t.Errorf("stdout lacks %q:\n%s", w, stdout.String())
		}
	}
}
