package speech

import (
	"errors"
	"strings"
	"testing"
)

func TestWakes(t *testing.T) {
	for _, c := range []struct {
		events []float64
		want   int
	}{
		{nil, 0},
		{[]float64{1.2}, 1},
		// One "Koinon" fires on several frames: one wake.
		{[]float64{0.80, 0.88, 0.96, 1.04}, 1},
		// Out of order, and 2 s apart or more: separate wakes.
		{[]float64{10, 0.5, 3.0, 0.6, 3.1}, 3},
		{[]float64{0, 2.0}, 2},
	} {
		if got := Wakes(c.events); got != c.want {
			t.Errorf("Wakes(%v) = %d, want %d", c.events, got, c.want)
		}
	}
}

// Misses and false wakes from a synthetic event log.
func TestScoreWake(t *testing.T) {
	take := func(n int) WakeFile { return WakeFile{Set: "wake/far", N: n, Seconds: 3, Positive: true} }
	bg := WakeFile{Set: "background/music", N: 1, Seconds: 1800}
	runs := []WakeRun{
		{Detector: "a", File: take(1), Events: []float64{0.9, 1.0}},
		{Detector: "a", File: take(2)},
		{Detector: "a", File: take(3), Err: errors.New("helper exited")},
		{Detector: "b", File: take(1), Events: []float64{1}},
		{Detector: "a", File: bg, Events: []float64{100, 101, 900, 1700}},
	}
	far := ScoreWake(runs, "a", "wake/far")
	if far.Files != 3 || far.Failed != 1 || far.Woke != 1 || far.Misses() != 2 {
		t.Errorf("far %+v", far)
	}
	music := ScoreWake(runs, "a", "background/music")
	if rate, ok := music.PerHour(); music.Wakes != 3 || music.Hours != 0.5 || rate != 6 || !ok {
		t.Errorf("music %+v %v", music, rate)
	}
	if _, ok := (WakeScore{}).PerHour(); ok {
		t.Error("a rate with no hours")
	}
}

// A failed recording's hours are not in the false-wake rate (peer review F1).
func TestScoreWakeFailedBackground(t *testing.T) {
	hour := func(set string, n int) WakeFile { return WakeFile{Set: set, N: n, Seconds: 3600} }
	runs := []WakeRun{
		{Detector: "a", File: hour("background/music", 1), Events: []float64{100}},
		{Detector: "a", File: hour("background/music", 2), Err: errors.New("helper exited")},
		{Detector: "a", File: hour("background/podcast", 1), Err: errors.New("helper exited")},
	}
	mixed := ScoreWake(runs, "a", "background/music")
	if rate, ok := mixed.PerHour(); rate != 1 || !ok || mixed.Hours != 1 || mixed.FailedHours != 1 || mixed.Failed != 1 {
		t.Errorf("mixed %+v %v %v", mixed, rate, ok)
	}
	failed := ScoreWake(runs, "a", "background/podcast")
	if _, ok := failed.PerHour(); ok || failed.FailedHours != 1 {
		t.Errorf("all failed %+v", failed)
	}
	var b strings.Builder
	WriteWakeScorecard(&b, []EngineInfo{{Name: "a"}}, runs)
	for _, want := range []string{"| a | background/music | 2 | 1 | 1.00 | 1.00 | 1 | 1.00 |", "| a | background/podcast | 1 | 1 | 0.00 | 1.00 | 0 | unavailable |", "| a | all | 3 | 2 | 1.00 | 2.00 | 1 | 1.00 |"} {
		if !strings.Contains(b.String(), want) {
			t.Errorf("scorecard lacks %q:\n%s", want, b.String())
		}
	}
}

func TestScoreInterruption(t *testing.T) {
	bridge := "Codex asks you to approve pull request twelve. It recommends merging after the tests pass."
	clean := ScoreInterruption(bridge, "Stop, wait, I have a question.", "Stop. Wait, I have a question.")
	if !clean.Passed() || clean.Said != 6 || len(clean.Heard) != 6 {
		t.Errorf("clean %+v", clean)
	}
	leak := ScoreInterruption(bridge, "Stop, wait, I have a question.", "approve pull request stop wait I have a question")
	if leak.Passed() || strings.Join(leak.Leaked, ",") != "approve,pull,request" {
		t.Errorf("leak %+v", leak)
	}
	missed := ScoreInterruption(bridge, "Stop, wait, I have a question.", "stop")
	if missed.Passed() || strings.Join(missed.Missed, ",") != "a,have,i,question,wait" {
		t.Errorf("missed %+v", missed)
	}
	// A common word is still the interruption's content (peer review F3).
	if no := ScoreInterruption(bridge, "No.", ""); no.Passed() || strings.Join(no.Missed, ",") != "no" {
		t.Errorf("missed no %+v", no)
	}
	if no := ScoreInterruption(bridge, "No.", "No!"); !no.Passed() {
		t.Errorf("heard no %+v", no)
	}
	// The echo control: nothing said, so only leaks count.
	if c := ScoreInterruption(bridge, "", ""); !c.Passed() {
		t.Errorf("silent control %+v", c)
	}
	if c := ScoreInterruption(bridge, "", "It recommends merging."); c.Passed() || strings.Join(c.Leaked, ",") != "merging,recommends" {
		t.Errorf("leaky control %+v", c)
	}
}

// The whole check needs voice processing, and an interruption run a barge-in during playback
// and a non-empty interruption (peer review F2 and F3).
func TestInterruptionVerdict(t *testing.T) {
	bridge := "Codex asks you to approve pull request twelve."
	heard := ScoreInterruption(bridge, "Stop, wait.", "Stop, wait.")
	for _, c := range []struct {
		name                  string
		score                 Interruption
		control, bargedIn, vp bool
		want                  string
	}{
		{"passed", heard, false, true, true, ""},
		{"spoken after the bridge finished", heard, false, false, true, "no barge-in during playback"},
		{"no voice processing", heard, false, true, false, "voice processing was off"},
		{"nothing to check", ScoreInterruption(bridge, "", ""), false, true, true, "the interruption has no words to check"},
		{"missed", ScoreInterruption(bridge, "No.", ""), false, true, true, "missed no"},
		{"silent control", ScoreInterruption(bridge, "", ""), true, false, true, ""},
		{"leaky control", ScoreInterruption(bridge, "", "approve twelve"), true, false, true, "the bridge's words leaked: 12, approve"},
		{"control without voice processing", ScoreInterruption(bridge, "", ""), true, false, false, "voice processing was off"},
	} {
		if got := strings.Join(c.score.Verdict(c.control, c.bargedIn, c.vp), "; "); got != c.want {
			t.Errorf("%s: %q, want %q", c.name, got, c.want)
		}
	}
}

// A phrase-matching miss is told apart by whether voice activity found speech (#10).
func TestWakeSilentMisses(t *testing.T) {
	take := func(n int) WakeFile { return WakeFile{Set: "wake/far", N: n, Seconds: 3, Positive: true} }
	zero, one := 0, 1
	runs := []WakeRun{
		{Detector: "phrase", File: take(1), Events: []float64{1}, Segments: &one},
		{Detector: "phrase", File: take(2), Segments: &zero},
		{Detector: "phrase", File: take(3), Segments: &one},
		{Detector: "oww", File: take(1)},
	}
	if s := ScoreWake(runs, "phrase", "wake/far"); !s.Segmented || s.Silent != 1 || s.Misses() != 2 {
		t.Errorf("phrase %+v", s)
	}
	if s := ScoreWake(runs, "oww", "wake/far"); s.Segmented || s.Silent != 0 {
		t.Errorf("oww %+v", s)
	}
	var b strings.Builder
	WriteWakeScorecard(&b, []EngineInfo{{Name: "phrase"}, {Name: "oww"}}, runs)
	for _, want := range []string{
		"| phrase | wake/far | 3 | 0 | 1 | 2 | 1 |", "| oww | wake/far | 1 | 0 | 0 | 1 |  |",
		"| phrase | wake/far | 02 | 0 |  | 0.000 | 0 | 0 ms |", "| oww | wake/far | 01 | 0 |  | 0.000 |  | 0 ms |",
	} {
		if !strings.Contains(b.String(), want) {
			t.Errorf("scorecard lacks %q:\n%s", want, b.String())
		}
	}
}
