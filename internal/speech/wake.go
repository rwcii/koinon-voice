package speech

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/rwcii/koinon-voice/internal/sidecar"
)

// Refractory is how close two firings of a detector are to count as one wake: a detector
// fires on several frames of one "Koinon", and the bridge would wake once.
const Refractory = 2.0 // seconds

// WakeDetector is one wake detector of the bench.
type WakeDetector struct {
	Name, Note string
	Start      func(ctx context.Context) (*sidecar.Helper, error)
	Params     func(path string) any
}

// WakeFile is one file the wake bench scores: a take of "Koinon" (Positive) or a background
// recording with no "Koinon" in it.
type WakeFile struct {
	Set      string
	N        int
	Path     string
	Seconds  float64
	Positive bool
}

// WakeRun is one detector's firings on one file.
type WakeRun struct {
	Detector string
	File     WakeFile
	Events   []float64 // seconds into the file
	MaxScore float64
	Latency  time.Duration
	Err      error
}

// Wakes merges firings closer than Refractory into one wake each and returns their count.
func Wakes(events []float64) int {
	s := append([]float64(nil), events...)
	sort.Float64s(s)
	n := 0
	last := 0.0
	for i, e := range s {
		if i == 0 || e-last >= Refractory {
			n++
			last = e
		}
	}
	return n
}

// WakeScore is a detector's score on one set: for takes, how many it woke on; for background,
// its wakes and their rate over the hours it scored. A failed recording's hours are kept apart
// in FailedHours and are not in the rate.
type WakeScore struct {
	Files, Failed, Woke, Wakes int
	Hours, FailedHours         float64
}

// Misses is takes the detector did not wake on (failed files count as misses).
func (s WakeScore) Misses() int { return s.Files - s.Woke }

// PerHour is false wakes per scored hour of background audio; ok is false when no hour was
// scored, as when every recording failed.
func (s WakeScore) PerHour() (rate float64, ok bool) {
	if s.Hours == 0 {
		return 0, false
	}
	return float64(s.Wakes) / s.Hours, true
}

// perHourCell is PerHour for the scorecard.
func (s WakeScore) perHourCell() string {
	rate, ok := s.PerHour()
	if !ok {
		return "unavailable"
	}
	return fmt.Sprintf("%.2f", rate)
}

// ScoreWake sums a detector's runs on one set.
func ScoreWake(runs []WakeRun, detector, set string) WakeScore {
	var s WakeScore
	for _, r := range runs {
		if r.Detector != detector || r.File.Set != set {
			continue
		}
		s.Files++
		if r.Err != nil {
			s.Failed++
			s.FailedHours += r.File.Seconds / 3600
			continue
		}
		s.Hours += r.File.Seconds / 3600
		w := Wakes(r.Events)
		s.Wakes += w
		if w > 0 {
			s.Woke++
		}
	}
	return s
}

// WakeFiles lists the recorded wake takes and background recordings that pass their checks,
// and why each other recording is left out.
func WakeFiles(c Corpus) ([]WakeFile, []string, error) {
	var files []WakeFile
	var skipped []string
	for _, set := range append(append([]string(nil), WakeSets...), BackgroundSets...) {
		m, err := c.Manifest(set)
		if err != nil {
			return nil, nil, err
		}
		var ns []int
		for n := range m {
			ns = append(ns, n)
		}
		sort.Ints(ns)
		for _, n := range ns {
			a, err := c.Verify(set, m[n])
			if err != nil {
				skipped = append(skipped, fmt.Sprintf("%s %02d: %v", set, n, err))
				continue
			}
			files = append(files, WakeFile{Set: set, N: n, Path: c.Path(set, n), Seconds: a.Seconds(), Positive: strings.HasPrefix(set, "wake/")})
		}
	}
	return files, skipped, nil
}

// RunWake runs each detector over the files, one request at a time. A detector whose
// helper cannot start, or whose first file fails, is recorded with the reason.
func RunWake(ctx context.Context, detectors []WakeDetector, files []WakeFile, opts BenchOptions) ([]EngineInfo, []WakeRun) {
	var infos []EngineInfo
	var out []WakeRun
	for _, d := range detectors {
		info := EngineInfo{Name: d.Name, Note: d.Note}
		h, err := d.Start(ctx)
		if err != nil {
			info.Err = err
			infos = append(infos, info)
			continue
		}
		hello := h.Hello()
		info.Helper, info.Version, info.Info = hello.Helper, hello.Version, hello.Info
		if !hello.Has("wake") {
			info.Err = fmt.Errorf("the helper does not offer wake")
			infos = append(infos, info)
			h.Close()
			continue
		}
		infos = append(infos, info)
		first := true
		for _, f := range files {
			r := WakeRun{Detector: d.Name, File: f}
			if h == nil || h.Err() != nil {
				if h != nil {
					h.Close()
					h = nil
				}
				nh, err := d.Start(ctx)
				if err != nil {
					r.Err = fmt.Errorf("restart: %w", err)
					out = append(out, r)
					continue
				}
				h = nh
			}
			// The first request loads the model, and a long background file takes a while.
			timeout := opts.Timeout + time.Duration(f.Seconds*float64(time.Second))
			if first {
				timeout += opts.Warmup
				first = false
			}
			rctx, cancel := context.WithTimeout(ctx, timeout)
			start := time.Now()
			data, err := h.Call(rctx, "wake", d.Params(f.Path), nil)
			r.Latency = time.Since(start)
			cancel()
			if err == nil {
				var res struct {
					Events   []float64 `json:"events"`
					MaxScore float64   `json:"max_score"`
				}
				if err = json.Unmarshal(data, &res); err == nil {
					r.Events, r.MaxScore = res.Events, res.MaxScore
				}
			}
			r.Err = err
			out = append(out, r)
		}
		if h != nil {
			h.Close()
		}
	}
	return infos, out
}

// WriteWakeScorecard writes the wake scorecard: detectors, misses per distance, false wakes
// per background source, and every take.
func WriteWakeScorecard(w io.Writer, infos []EngineInfo, runs []WakeRun) {
	b := &strings.Builder{}
	b.WriteString("## Detectors\n\n| Detector | Helper | Info | Note |\n| --- | --- | --- | --- |\n")
	for _, e := range infos {
		helper := e.Helper + " " + e.Version
		if e.Err != nil {
			helper = "not runnable: " + cell(e.Err.Error())
		}
		fmt.Fprintf(b, "| %s | %s | %s | %s |\n", e.Name, helper, cell(string(e.Info)), cell(e.Note))
	}
	b.WriteString("\n## Misses\n\nTakes of \"Koinon\" the detector did not wake on; a failed file counts as a miss.\n\n| Detector | Set | Takes | Failed | Woke | Misses |\n| --- | --- | --- | --- | --- | --- |\n")
	for _, e := range infos {
		if e.Err != nil {
			continue
		}
		for _, set := range WakeSets {
			s := ScoreWake(runs, e.Name, set)
			if s.Files == 0 {
				continue
			}
			fmt.Fprintf(b, "| %s | %s | %d | %d | %d | %d |\n", e.Name, set, s.Files, s.Failed, s.Woke, s.Misses())
		}
	}
	fmt.Fprintf(b, "\n## False wakes\n\nWakes on background audio, with firings closer than %.0f s counted once. The rate is over the hours scored; a failed recording's hours are not in it.\n\n| Detector | Set | Recordings | Failed | Hours scored | Hours failed | False wakes | Per hour |\n| --- | --- | --- | --- | --- | --- | --- | --- |\n", Refractory)
	for _, e := range infos {
		if e.Err != nil {
			continue
		}
		var total WakeScore
		for _, set := range BackgroundSets {
			s := ScoreWake(runs, e.Name, set)
			if s.Files == 0 {
				continue
			}
			total.Files += s.Files
			total.Failed += s.Failed
			total.Wakes += s.Wakes
			total.Hours += s.Hours
			total.FailedHours += s.FailedHours
			fmt.Fprintf(b, "| %s | %s | %d | %d | %.2f | %.2f | %d | %s |\n", e.Name, set, s.Files, s.Failed, s.Hours, s.FailedHours, s.Wakes, s.perHourCell())
		}
		if total.Files > 0 {
			fmt.Fprintf(b, "| %s | all | %d | %d | %.2f | %.2f | %d | %s |\n", e.Name, total.Files, total.Failed, total.Hours, total.FailedHours, total.Wakes, total.perHourCell())
		}
	}
	b.WriteString("\n## Runs\n\n| Detector | Set | File | Wakes | Firings | Max score | Latency |\n| --- | --- | --- | --- | --- | --- | --- |\n")
	for _, r := range runs {
		firings := ""
		if r.Err != nil {
			firings = "failed: " + cell(r.Err.Error())
		} else {
			var ts []string
			for _, e := range r.Events {
				ts = append(ts, fmt.Sprintf("%.2f", e))
			}
			if len(ts) > 12 {
				ts = append(ts[:12], fmt.Sprintf("… %d in all", len(r.Events)))
			}
			firings = strings.Join(ts, " ")
		}
		fmt.Fprintf(b, "| %s | %s | %02d | %d | %s | %.3f | %d ms |\n", r.Detector, r.File.Set, r.File.N, Wakes(r.Events), firings, r.MaxScore, r.Latency.Milliseconds())
	}
	io.WriteString(w, b.String())
}
