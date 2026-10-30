package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rwcii/koinon-voice/internal/speech"
)

// benchCorpus writes a phrase list and a clean set of synthetic tones for phrases 1 to 3;
// phrase 4 is not recorded.
func benchCorpus(t *testing.T) (phrases, dir string) {
	t.Helper()
	tmp := t.TempDir()
	phrases = filepath.Join(tmp, "phrases.md")
	os.WriteFile(phrases, []byte("01. [Koinon], read my inbox.\n02. [Approve] pull request [twelve].\n03. Stop.\n04. Go on.\n05. [codex-koinon-voice-af]\n"), 0o600)
	dir = filepath.Join(tmp, "corpus")
	c := speech.Corpus{Dir: dir}
	if _, err := c.Prepare("clean"); err != nil {
		t.Fatal(err)
	}
	m := map[int]speech.Entry{}
	tone := make([]int16, 16000)
	for i := range tone {
		tone[i] = int16((i % 40) * 400)
	}
	for _, n := range []int{1, 2, 3} {
		if err := speech.WriteWAV(c.Path("clean", n), 16000, tone); err != nil {
			t.Fatal(err)
		}
		sum, _ := speech.Hash(c.Path("clean", n))
		m[n] = speech.Entry{N: n, SHA256: sum, Seconds: 1}
	}
	if err := c.WriteManifest("clean", m); err != nil {
		t.Fatal(err)
	}
	return phrases, dir
}

func TestBenchSTT(t *testing.T) {
	t.Setenv(fakeHelperMode, "1")
	phrases, dir := benchCorpus(t)
	out := filepath.Join(t.TempDir(), "card.md")
	var stdout, stderr bytes.Buffer
	code := run([]string{"bench", "stt", "--phrases", phrases, "--dir", dir, "--sets", "clean", "--helper", os.Args[0],
		"--python", os.Args[0], "--engines", "/nonexistent", "--out", out}, nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d: %s%s", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "skipped clean 04: not recorded") {
		t.Errorf("stderr %q", stderr.String())
	}
	card, _ := os.ReadFile(out)
	for _, want := range []string{
		"| apple | fake 0.0.1 |", "| apple-keywords | fake 0.0.1 |", "| whistle-keywords | fake 0.0.1 |",
		"biased to: Koinon, codex koinon voice af",
		// Phrase 1 right, phrase 2 one error on a term, and phrase 3 failed: its word is a
		// deletion.
		"| parakeet | clean | 3 | 1 | 22.2% (2/9) | 33.3% (1/3) |",
		"| 02 | clean | apple | 1/4 | 1/2 |", "Approved pull request 12.",
		"| 03 | clean | whistle | 1/1 | 0/0 |", "failed: engine_error: decoder failed",
	} {
		if !strings.Contains(string(card), want) {
			t.Errorf("scorecard lacks %q:\n%s", want, card)
		}
	}

	// An engine whose helper cannot start is recorded as not runnable; the others run.
	stdout.Reset()
	code = run([]string{"bench", "stt", "--phrases", phrases, "--dir", dir, "--sets", "clean", "--helper", os.Args[0],
		"--python", "/nonexistent/python", "--engines", "/nonexistent", "--engine", "apple,parakeet", "--out", out}, nil, &stdout, &stderr)
	card, _ = os.ReadFile(out)
	if code != 0 || !strings.Contains(stdout.String(), "parakeet: not runnable") || !strings.Contains(string(card), "| apple | clean | 3 |") ||
		strings.Contains(string(card), "| parakeet | clean |") || strings.Contains(string(card), "whistle") {
		t.Errorf("exit %d:\n%s\n%s", code, stdout.String(), card)
	}
}

func TestBenchSTTNoFiles(t *testing.T) {
	phrases, _ := benchCorpus(t)
	var stdout, stderr bytes.Buffer
	if code := run([]string{"bench", "stt", "--phrases", phrases, "--dir", t.TempDir(), "--helper", "/h"}, nil, &stdout, &stderr); code != 1 ||
		!strings.Contains(stderr.String(), "no corpus files to score") {
		t.Errorf("exit %d: %s", code, stderr.String())
	}
}

func TestBenchTTS(t *testing.T) {
	t.Setenv(fakeHelperMode, "1")
	tmp := t.TempDir()
	texts := filepath.Join(tmp, "texts.md")
	os.WriteFile(texts, []byte("01. Codex asks you to merge. It passes its checks. [SYN-01, test]\n02. Claude asks you to choose. [SYN-02, test]\n"), 0o600)
	var played []string
	oldPlay, oldShuffle := playAudio, shuffle
	playAudio = func(_ context.Context, path string) error {
		played = append(played, filepath.Base(path))
		return nil
	}
	shuffle = func(n int, swap func(i, j int)) { swap(0, 1) } // kokoro is A
	t.Cleanup(func() { playAudio, shuffle = oldPlay, oldShuffle })

	out := filepath.Join(tmp, "card.md")
	var stdout, stderr bytes.Buffer
	code := run([]string{"bench", "tts", "--texts", texts, "--dir", filepath.Join(tmp, "audio"), "--helper", os.Args[0],
		"--kokoro-helper", os.Args[0], "--engines", "/nonexistent", "--out", out}, strings.NewReader("4\n2\n5\nq\n"), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d: %s%s", code, stdout.String(), stderr.String())
	}
	got := stdout.String()
	blind := got[:strings.Index(got, "Engines:")]
	if strings.Contains(blind, "kokoro") || strings.Contains(blind, "apple") {
		t.Errorf("engine names shown before the key:\n%s", blind)
	}
	if strings.Join(played, ",") != "01-kokoro.wav,01-apple.caf,02-kokoro.wav,02-apple.caf" {
		t.Errorf("played %v", played)
	}
	card, _ := os.ReadFile(out)
	for _, want := range []string{"Text 01 A = kokoro", "Text 02 A = kokoro", "Rating stopped",
		"| kokoro | 2 | 0 | 7 ms | 4.5 (2) |", "| apple | 2 | 0 | 2 ms | 2.0 (1) |", "| 01 | kokoro | 7 ms | 2.0 s | 4 | 4 |", "| 02 | kokoro | 7 ms | 1.0 s | 2 | 5 |", "| 01 | apple | 2 ms | 0.0 s |  | 2 |",
		"Kokoro-82M on MLX Swift through koinon-voice-kokoro, voice af_heart, speed 1.1"} {
		if !strings.Contains(got+string(card), want) {
			t.Errorf("output lacks %q:\n%s\n%s", want, got, card)
		}
	}
	// The synthesized audio is private to its owner.
	if fi, err := os.Stat(filepath.Join(tmp, "audio")); err != nil || fi.Mode().Perm() != 0o700 {
		t.Errorf("audio directory %v %v", fi.Mode(), err)
	}
}

func TestBenchUsage(t *testing.T) {
	for _, args := range [][]string{{"bench"}, {"bench", "listen"}, {"bench", "stt", "extra"}, {"bench", "wake", "extra"}, {"bench", "tts", "--nope"}} {
		var stdout, stderr bytes.Buffer
		if code := run(args, nil, &stdout, &stderr); code != 1 || !isCommandError(stderr.String()) {
			t.Errorf("%q: exit %d", args, code)
		}
	}
}

func TestBenchWake(t *testing.T) {
	t.Setenv(fakeHelperMode, "1")
	phrases, _ := benchCorpus(t)
	dir := filepath.Join(t.TempDir(), "corpus")
	c := speech.Corpus{Dir: dir}
	tone := make([]int16, 16000)
	for i := range tone {
		tone[i] = int16((i % 40) * 400)
	}
	record := func(set string, ns ...int) {
		if _, err := c.Prepare(set); err != nil {
			t.Fatal(err)
		}
		m := map[int]speech.Entry{}
		for _, n := range ns {
			speech.WriteWAV(c.Path(set, n), 16000, tone)
			sum, _ := speech.Hash(c.Path(set, n))
			m[n] = speech.Entry{N: n, SHA256: sum, Seconds: 1}
		}
		c.WriteManifest(set, m)
	}
	record("wake/near", 1, 2)
	record("wake/far", 1)
	record("background/music", 1)
	record("background/podcast", 1)
	out := filepath.Join(t.TempDir(), "card.md")
	var stdout, stderr bytes.Buffer
	code := run([]string{"bench", "wake", "--phrases", phrases, "--dir", dir, "--python", os.Args[0], "--engines", "/nonexistent",
		"--model", "/nonexistent/koinon.onnx", "--out", out}, nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d: %s%s", code, stdout.String(), stderr.String())
	}
	card, _ := os.ReadFile(out)
	for _, want := range []string{
		"| openwakeword | wake/near | 2 | 0 | 1 | 1 |", "| phrase | wake/far | 1 | 0 | 1 | 0 |",
		// One second of music is 1/3600 hour: one false wake is 3600 per hour.
		"| openwakeword | background/music | 1 | 0 | 0.00 | 0.00 | 1 | 3600.00 |", "| openwakeword | background/podcast | 1 | 0 | 0.00 | 0.00 | 0 | 0.00 |",
		"| openwakeword | all | 2 | 0 | 0.00 | 0.00 | 1 | 1800.00 |",
		"threshold 0.35", "wakes on the word koinon",
		"| openwakeword | wake/near | 01 | 1 | 0.88 0.96 | 0.900 |",
	} {
		if !strings.Contains(string(card), want) {
			t.Errorf("scorecard lacks %q:\n%s", want, card)
		}
	}

	var e bytes.Buffer
	if code := run([]string{"bench", "wake", "--phrases", phrases, "--dir", t.TempDir(), "--python", os.Args[0]}, nil, &stdout, &e); code != 1 ||
		!strings.Contains(e.String(), "no wake takes or background recordings") {
		t.Errorf("empty corpus: exit %d %s", code, e.String())
	}
}

func TestBenchInterrupt(t *testing.T) {
	t.Setenv(fakeHelperMode, "1")
	tmp := t.TempDir()
	texts := filepath.Join(tmp, "texts.md")
	os.WriteFile(texts, []byte("04. Codex asks you to decide whether to approve the dashboard login hotfix. [SYN-06, test]\n"), 0o600)
	out := filepath.Join(tmp, "run.md")
	var stdout, stderr bytes.Buffer
	code := run([]string{"bench", "interrupt", "--texts", texts, "--text", "4", "--helper", os.Args[0], "--engines", "/nonexistent",
		"--dir", filepath.Join(tmp, "audio"), "--out", out}, strings.NewReader("\n"), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d: %s%s", code, stdout.String(), stderr.String())
	}
	report, _ := os.ReadFile(out)
	for _, want := range []string{"## interrupt run", `Interruption said: "Stop, wait, I have a question."`, "Barge-in: 3100 ms after the start.",
		"Interruption words heard: [a have i question stop wait]; missed: [].", "Bridge words in the transcript: [hotfix].",
		"Result: failed: the bridge's words leaked: hotfix.",
		"voice processing true"} {
		if !strings.Contains(string(report), want) {
			t.Errorf("report lacks %q:\n%s", want, report)
		}
	}
	if fi, err := os.Stat(filepath.Join(tmp, "audio")); err != nil || fi.Mode().Perm() != 0o700 {
		t.Errorf("capture directory %v %v", fi.Mode(), err)
	}
	// The control appends a second run, which fails on the fake's leak.
	if code := run([]string{"bench", "interrupt", "--control", "--texts", texts, "--text", "4", "--helper", os.Args[0], "--engines", "/nonexistent",
		"--dir", filepath.Join(tmp, "audio"), "--out", out}, strings.NewReader("\n"), &stdout, &stderr); code != 0 {
		t.Fatalf("control exit %d", code)
	}
	report, _ = os.ReadFile(out)
	if strings.Count(string(report), "## ") != 2 || !strings.Contains(string(report), "## control run") || !strings.Contains(string(report), "Result: failed: the bridge's words leaked: hotfix.") {
		t.Errorf("report:\n%s", report)
	}
	// Every word heard, but after the bridge finished: not an interruption (peer review F2).
	t.Setenv(fakeDuplexAfter, "1")
	after := filepath.Join(tmp, "after.md")
	if code := run([]string{"bench", "interrupt", "--texts", texts, "--text", "4", "--helper", os.Args[0], "--engines", "/nonexistent",
		"--dir", filepath.Join(tmp, "audio"), "--out", after}, strings.NewReader("\n"), &stdout, &stderr); code != 0 {
		t.Fatalf("after exit %d", code)
	}
	report, _ = os.ReadFile(after)
	if !strings.Contains(string(report), "Barge-in: none.") || !strings.Contains(string(report), "missed: [].") ||
		!strings.Contains(string(report), "Result: failed: no barge-in during playback.") {
		t.Errorf("after report:\n%s", report)
	}
	if code := run([]string{"bench", "interrupt", "--texts", texts, "--text", "9", "--helper", os.Args[0]}, strings.NewReader("\n"), &stdout, &stderr); code != 1 {
		t.Errorf("missing text: exit %d", code)
	}
}

// bench names resolves the names in a scorecard's transcripts without recordings.
func TestBenchNames(t *testing.T) {
	dir := t.TempDir()
	phrases := filepath.Join(dir, "phrases.md")
	card := filepath.Join(dir, "card.md")
	near := filepath.Join(dir, "near.txt")
	out := filepath.Join(dir, "names.md")
	for path, text := range map[string]string{
		phrases: "01. [Koinon], what's waiting?\n02. Ask [codex-koinon-voice-af].\n",
		card:    "| 01 | clean | e | 1/3 | 1/1 | 9 ms | Quinan, what's waiting? |\n| 02 | clean | e | 3/6 | 3/5 | 9 ms | Ask codex coin invoice AF. |\n",
		near:    "Fire the cannon.\nCarry on.\n",
	} {
		if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	var stdout, stderr bytes.Buffer
	if code := run([]string{"bench", "names", "--phrases", phrases, "--scorecard", card, "--ordinary", near, "--out", out}, nil, &stdout, &stderr); code != 0 {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	got, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"Koinon, codex koinon voice af",
		"| e | clean | 2 | 37.5% (3/8) | 0.0% (0/8) | 60.0% (3/5) | 0.0% (0/5) | 2 | 0 | 0 |",
		"| near.txt | 2 | 1 | Fire the cannon.: cannon → koinon |",
	} {
		if !strings.Contains(string(got), want) {
			t.Errorf("report lacks %q:\n%s", want, got)
		}
	}
	if code := run([]string{"bench", "names", "--phrases", phrases, "--scorecard", filepath.Join(dir, "missing.md"), "--out", out}, nil, &stdout, &stderr); code != 1 {
		t.Errorf("missing scorecard: exit %d", code)
	}
}
