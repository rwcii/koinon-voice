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
	os.WriteFile(texts, []byte("01. Codex asks you to merge. [SYN-01, test]\n02. Claude asks you to choose. [SYN-02, test]\n"), 0o600)
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
		"--python", os.Args[0], "--engines", "/nonexistent", "--out", out}, strings.NewReader("4\n2\n5\nq\n"), &stdout, &stderr)
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
		"| kokoro | 2 | 0 | 7 ms | 4.5 (2) |", "| apple | 2 | 0 | 2 ms | 2.0 (1) |", "| 01 | kokoro | 7 ms | 1.5 s | 4 |"} {
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
	for _, args := range [][]string{{"bench"}, {"bench", "wake"}, {"bench", "stt", "extra"}, {"bench", "tts", "--nope"}} {
		var stdout, stderr bytes.Buffer
		if code := run(args, nil, &stdout, &stderr); code != 2 {
			t.Errorf("%q: exit %d", args, code)
		}
	}
}
