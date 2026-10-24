package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/rwcii/koinon-voice/internal/speech"
)

// wantChannel is the input channel the next recordings must ask for.
var wantChannel = 1

// fakeRecorder writes a synthetic file per recording in place of the helper: a tone, or
// what the next entry of plan says ("silent", "fail", "8k").
func fakeRecorder(t *testing.T, plan ...string) *[]float64 {
	t.Helper()
	var seconds []float64
	old := recordPhrase
	recordPhrase = func(_ context.Context, helper string, secs float64, path string, channel int, _ io.Writer) error {
		if helper != "/fake/helper" || channel != wantChannel {
			t.Errorf("helper %q, channel %d", helper, channel)
		}
		seconds = append(seconds, secs)
		kind := "tone"
		if len(plan) > 0 {
			kind, plan = plan[0], plan[1:]
		}
		samples := make([]int16, 16000)
		if kind != "silent" {
			for i := range samples {
				samples[i] = int16((i % 40) * 400)
			}
		}
		switch kind {
		case "fail":
			return errors.New("exit status 1")
		case "8k":
			return speech.WriteWAV(path, 8000, samples)
		}
		return speech.WriteWAV(path, 16000, samples)
	}
	t.Cleanup(func() { recordPhrase = old })
	return &seconds
}

func corpusSetup(t *testing.T) (phrases, dir string) {
	t.Helper()
	tmp := t.TempDir()
	phrases = filepath.Join(tmp, "phrases.md")
	os.WriteFile(phrases, []byte("01. [Koinon], what's waiting?\n02. [Approve] pull request [twelve].\n03. Stop.\n"), 0o600)
	return phrases, filepath.Join(tmp, "corpus")
}

func TestCorpusRecord(t *testing.T) {
	phrases, dir := corpusSetup(t)
	// Phrase 1 fails once, then is silent, then is kept; phrase 2 is recorded again with r,
	// then kept; phrase 3 is skipped.
	seconds := fakeRecorder(t, "fail", "silent", "tone", "tone", "tone")
	var stdout, stderr bytes.Buffer
	stdin := strings.NewReader("\n\n\n\n\nr\n\n\ns\n")
	code := run([]string{"corpus", "record", "--set", "clean", "--phrases", phrases, "--dir", dir, "--helper", "/fake/helper"}, stdin, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d: %s%s", code, stdout.String(), stderr.String())
	}
	out := stdout.String()
	for _, want := range []string{"[1/3] 01. Koinon, what's waiting?", "Recording failed: exit status 1", "is silent", "Done: 2 of 3 phrases are in the clean set."} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
	if len(*seconds) != 5 || (*seconds)[0] != 3.5 {
		t.Errorf("recordings %v", *seconds)
	}
	c := speech.Corpus{Dir: dir}
	m, err := c.Manifest("clean")
	if err != nil || len(m) != 2 || m[1].Seconds != 1 || m[3].N != 0 {
		t.Fatalf("manifest %+v, %v", m, err)
	}
	for _, e := range m {
		if _, err := c.Verify("clean", e); err != nil {
			t.Error(err)
		}
	}
	// Private: the corpus and the set are the owner's only.
	for _, d := range []string{dir, filepath.Join(dir, "clean")} {
		if fi, err := os.Stat(d); err != nil || fi.Mode().Perm() != 0o700 {
			t.Errorf("%s: %v %v", d, fi.Mode(), err)
		}
	}
	if left, _ := filepath.Glob(filepath.Join(dir, "clean", "*.new.wav")); len(left) != 0 {
		t.Errorf("temporary files left: %v", left)
	}

	// A second session records only what is missing; status reports both sets.
	*seconds = nil
	stdout.Reset()
	if code := run([]string{"corpus", "record", "--set", "clean", "--phrases", phrases, "--dir", dir, "--helper", "/fake/helper"}, strings.NewReader("\n\n"), &stdout, &stderr); code != 0 {
		t.Fatalf("exit %d", code)
	}
	if len(*seconds) != 1 || !strings.Contains(stdout.String(), "[1/1] 03. Stop.") {
		t.Errorf("resume: %v\n%s", *seconds, stdout.String())
	}
	stdout.Reset()
	if code := run([]string{"corpus", "status", "--phrases", phrases, "--dir", dir}, nil, &stdout, &stderr); code != 0 {
		t.Fatalf("status exit %d: %s", code, stdout.String())
	}
	if !strings.Contains(stdout.String(), "clean: 3 of 3 recorded") || !strings.Contains(stdout.String(), "noisy: 0 of 3 recorded") {
		t.Errorf("status:\n%s", stdout.String())
	}

	// A file changed after recording is reported, not scored.
	os.WriteFile(c.Path("clean", 2), []byte("RIFF"), 0o600)
	stdout.Reset()
	if code := run([]string{"corpus", "status", "--phrases", phrases, "--dir", dir}, nil, &stdout, &stderr); code != 1 || !strings.Contains(stdout.String(), "changed since it was recorded") {
		t.Errorf("exit %d:\n%s", code, stdout.String())
	}

	// --only records a phrase again; q stops.
	*seconds = nil
	stdout.Reset()
	if code := run([]string{"corpus", "record", "--set", "clean", "--phrases", phrases, "--dir", dir, "--helper", "/fake/helper", "--only", "2,3"}, strings.NewReader("\n\nq\n"), &stdout, &stderr); code != 0 {
		t.Fatalf("exit %d", code)
	}
	if len(*seconds) != 1 || !strings.Contains(stdout.String(), "Stopped; 3 of 3") {
		t.Errorf("only: %v\n%s", *seconds, stdout.String())
	}
	if code := run([]string{"corpus", "status", "--phrases", phrases, "--dir", dir}, nil, &stdout, &stderr); code != 0 {
		t.Errorf("phrase 2 recorded again is still unusable: %s", stdout.String())
	}
}

func TestCorpusWrongRate(t *testing.T) {
	phrases, dir := corpusSetup(t)
	fakeRecorder(t, "8k")
	wantChannel = 3
	t.Cleanup(func() { wantChannel = 1 })
	var stdout, stderr bytes.Buffer
	run([]string{"corpus", "record", "--set", "noisy", "--channel", "3", "--phrases", phrases, "--dir", dir, "--helper", "/fake/helper"}, strings.NewReader("\nq\n"), &stdout, &stderr)
	if !strings.Contains(stdout.String(), "is 8000 Hz, 1 channels, 16 bits; want 16000 Hz mono 16-bit") {
		t.Errorf("output:\n%s", stdout.String())
	}
}

func TestCorpusUsage(t *testing.T) {
	phrases, dir := corpusSetup(t)
	for _, args := range [][]string{
		{"corpus"},
		{"corpus", "play"},
		{"corpus", "record", "--phrases", phrases, "--dir", dir},
		{"corpus", "status", "--set", "clean", "--phrases", phrases},
		{"corpus", "record", "--set", "clean", "--only", "x", "--phrases", phrases, "--dir", dir, "--helper", "/h"},
		{"corpus", "record", "--set", "clean", "--channel", "0", "--phrases", phrases, "--dir", dir, "--helper", "/h"},
	} {
		var stdout, stderr bytes.Buffer
		if code := run(args, nil, &stdout, &stderr); code != 2 {
			t.Errorf("%q: exit %d", args, code)
		}
	}
	var stdout, stderr bytes.Buffer
	if code := run([]string{"corpus", "record", "--set", "loud", "--phrases", phrases, "--dir", dir, "--helper", "/h"}, nil, &stdout, &stderr); code != 1 || !strings.Contains(stderr.String(), `corpus set "loud"`) {
		t.Errorf("exit %d: %s", code, stderr.String())
	}
}

func TestCorpusWakeAndBackground(t *testing.T) {
	phrases, dir := corpusSetup(t)
	seconds := fakeRecorder(t)
	var stdout, stderr bytes.Buffer
	if code := run([]string{"corpus", "wake", "--distance", "far", "--count", "2", "--phrases", phrases, "--dir", dir, "--helper", "/fake/helper"},
		strings.NewReader("\n\n\n\n"), &stdout, &stderr); code != 0 {
		t.Fatalf("wake exit %d: %s%s", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), `[1/2] 01. Say "Koinon" once`) || !strings.Contains(stdout.String(), "Done: 2 of 2 takes are in the wake/far set.") {
		t.Errorf("wake:\n%s", stdout.String())
	}
	stdout.Reset()
	if code := run([]string{"corpus", "background", "--source", "podcast", "--count", "1", "--minutes", "0.5", "--phrases", phrases, "--dir", dir, "--helper", "/fake/helper"},
		strings.NewReader("\n\n"), &stdout, &stderr); code != 0 {
		t.Fatalf("background exit %d: %s%s", code, stdout.String(), stderr.String())
	}
	if fmtFloats(*seconds) != "3,3,30" {
		t.Errorf("recording lengths %v", *seconds)
	}
	for _, d := range []string{filepath.Join(dir, "wake"), filepath.Join(dir, "wake", "far"), filepath.Join(dir, "background", "podcast")} {
		if fi, err := os.Stat(d); err != nil || fi.Mode().Perm() != 0o700 {
			t.Errorf("%s: %v %v", d, fi.Mode(), err)
		}
	}
	stdout.Reset()
	run([]string{"corpus", "status", "--phrases", phrases, "--dir", dir}, nil, &stdout, &stderr)
	for _, want := range []string{"wake/far: 2 takes", "wake/near: 0 takes", "background/podcast: 1 recordings, 0 minutes"} {
		if !strings.Contains(stdout.String(), want) {
			t.Errorf("status lacks %q:\n%s", want, stdout.String())
		}
	}
	for _, args := range [][]string{
		{"corpus", "wake", "--phrases", phrases},
		{"corpus", "background", "--phrases", phrases},
		{"corpus", "wake", "--distance", "near", "--set", "clean", "--phrases", phrases},
		{"corpus", "background", "--source", "music", "--minutes", "0", "--phrases", phrases},
	} {
		if code := run(args, nil, &stdout, &stderr); code != 2 {
			t.Errorf("%q: exit %d", args, code)
		}
	}
	if code := run([]string{"corpus", "wake", "--distance", "roof", "--phrases", phrases, "--dir", dir, "--helper", "/h"}, strings.NewReader(""), &stdout, &stderr); code != 1 ||
		!strings.Contains(stderr.String(), `corpus set "wake/roof"`) {
		t.Errorf("bad distance: exit %d %s", code, stderr.String())
	}
}

func fmtFloats(fs []float64) string {
	var parts []string
	for _, f := range fs {
		parts = append(parts, strconv.FormatFloat(f, 'f', -1, 64))
	}
	return strings.Join(parts, ",")
}
