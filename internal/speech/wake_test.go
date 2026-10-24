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
	if music.Wakes != 3 || music.Hours != 0.5 || music.PerHour() != 6 {
		t.Errorf("music %+v %v", music, music.PerHour())
	}
	if (WakeScore{}).PerHour() != 0 {
		t.Error("no hours")
	}
}

func TestScoreInterruption(t *testing.T) {
	bridge := "Codex asks you to approve pull request twelve. It recommends merging after the tests pass."
	clean := ScoreInterruption(bridge, "Stop, wait, I have a question.", "Stop. Wait, I have a question.")
	if !clean.Passed() || len(clean.Heard) != 4 {
		t.Errorf("clean %+v", clean)
	}
	leak := ScoreInterruption(bridge, "Stop, wait, I have a question.", "approve pull request stop wait I have a question")
	if leak.Passed() || strings.Join(leak.Leaked, ",") != "approve,pull,request" {
		t.Errorf("leak %+v", leak)
	}
	missed := ScoreInterruption(bridge, "Stop, wait, I have a question.", "stop")
	if missed.Passed() || strings.Join(missed.Missed, ",") != "have,question,wait" {
		t.Errorf("missed %+v", missed)
	}
	// The echo control: nothing said, so only leaks count.
	if c := ScoreInterruption(bridge, "", ""); !c.Passed() {
		t.Errorf("silent control %+v", c)
	}
	if c := ScoreInterruption(bridge, "", "It recommends merging."); c.Passed() || strings.Join(c.Leaked, ",") != "merging,recommends" {
		t.Errorf("leaky control %+v", c)
	}
}
