package speech

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/rwcii/koinon-voice/internal/sidecar"
)

// Recognizer is one recognition engine of the bench: a helper it starts, and the request it
// sends for a file.
type Recognizer struct {
	Name string
	Note string
	// Start runs the engine's helper.
	Start func(ctx context.Context) (*sidecar.Helper, error)
	// Params is the transcribe request for one file.
	Params func(path string) any
}

// Transcript is one engine's result for one corpus file.
type Transcript struct {
	Engine  string
	Set     string
	Phrase  Phrase
	Seconds float64 // audio
	Text    string
	Latency time.Duration // request to result, measured by the bench
	Score   Score
	Err     error
}

// RTF is the real-time factor: processing time over audio time.
func (t Transcript) RTF() float64 {
	if t.Seconds == 0 {
		return 0
	}
	return t.Latency.Seconds() / t.Seconds
}

// EngineInfo is what a helper said about itself, for the scorecard.
type EngineInfo struct {
	Name, Note, Helper, Version string
	Info                        json.RawMessage
	Err                         error // the engine could not run
}

// BenchOptions bounds the bench.
type BenchOptions struct {
	// Timeout is each request's; the warm-up request, which loads the model, gets Warmup.
	Timeout, Warmup time.Duration
	// Progress, if set, hears each transcript as it is made.
	Progress func(Transcript)
}

// File is one corpus recording to score.
type File struct {
	Set    string
	Phrase Phrase
	Path   string
	Audio  Audio
}

// CorpusFiles lists every recording of the sets that the manifest records and that passes
// its checks, and why each other phrase is left out.
func CorpusFiles(c Corpus, list []Phrase, sets []string) ([]File, []string, error) {
	var files []File
	var skipped []string
	for _, set := range sets {
		m, err := c.Manifest(set)
		if err != nil {
			return nil, nil, err
		}
		for _, p := range list {
			e, ok := m[p.N]
			if !ok {
				skipped = append(skipped, fmt.Sprintf("%s %02d: not recorded", set, p.N))
				continue
			}
			a, err := c.Verify(set, e)
			if err != nil {
				skipped = append(skipped, fmt.Sprintf("%s %02d: %v", set, p.N, err))
				continue
			}
			files = append(files, File{Set: set, Phrase: p, Path: c.Path(set, p.N), Audio: a})
		}
	}
	return files, skipped, nil
}

// RunSTT runs each engine over the files, one request at a time. An engine that cannot
// start is recorded with its reason and the others run. The first file is sent once more
// before scoring, to load the model, and that result is not scored.
func RunSTT(ctx context.Context, engines []Recognizer, files []File, opts BenchOptions) ([]EngineInfo, []Transcript) {
	var infos []EngineInfo
	var out []Transcript
	for _, e := range engines {
		info := EngineInfo{Name: e.Name, Note: e.Note}
		h, err := e.Start(ctx)
		if err != nil {
			info.Err = err
			infos = append(infos, info)
			continue
		}
		hello := h.Hello()
		info.Helper, info.Version, info.Info = hello.Helper, hello.Version, hello.Info
		if !hello.Has("transcribe") {
			info.Err = fmt.Errorf("the helper does not offer transcribe")
		} else if len(files) > 0 {
			wctx, cancel := context.WithTimeout(ctx, opts.Warmup)
			_, err := h.Call(wctx, "transcribe", e.Params(files[0].Path), nil)
			cancel()
			if err != nil {
				info.Err = fmt.Errorf("warm-up: %w", err)
			}
		}
		infos = append(infos, info)
		if info.Err != nil {
			h.Close()
			continue
		}
		for _, f := range files {
			t := Transcript{Engine: e.Name, Set: f.Set, Phrase: f.Phrase, Seconds: f.Audio.Seconds()}
			if h == nil || h.Err() != nil {
				// The last request killed the helper (a deadline), or a restart failed; start
				// a new one. A file with no helper fails and counts as nothing heard.
				if h != nil {
					h.Close()
					h = nil
				}
				nh, err := e.Start(ctx)
				if err != nil {
					t.Err = fmt.Errorf("restart: %w", err)
					t.Score = Compare(f.Phrase, "")
					if opts.Progress != nil {
						opts.Progress(t)
					}
					out = append(out, t)
					continue
				}
				h = nh
			}
			rctx, cancel := context.WithTimeout(ctx, opts.Timeout)
			start := time.Now()
			data, err := h.Call(rctx, "transcribe", e.Params(f.Path), nil)
			t.Latency = time.Since(start)
			cancel()
			if err == nil {
				var r struct{ Text string }
				if err = json.Unmarshal(data, &r); err == nil {
					t.Text = r.Text
				}
			}
			t.Err = err
			t.Score = Compare(f.Phrase, t.Text)
			if opts.Progress != nil {
				opts.Progress(t)
			}
			out = append(out, t)
		}
		if h != nil {
			h.Close()
		}
	}
	return infos, out
}

// Keywords is the Whistle biasing list seeded from the phrase list's peer and repository
// names (chunk 04): every bracketed term with a hyphen that is not a number, in its spoken
// form, with "rwcii" and "Koinon".
func Keywords(raw string) []string {
	seen := map[string]bool{"koinon": true}
	out := []string{"Koinon"}
	for _, line := range strings.Split(raw, "\n") {
		for {
			open := strings.IndexByte(line, '[')
			end := strings.IndexByte(line, ']')
			if open < 0 || end < open {
				break
			}
			term := line[open+1 : end]
			line = line[end+1:]
			if !strings.Contains(term, "-") && strings.ToLower(term) != "rwcii" {
				continue
			}
			words := Normalize(term)
			if len(words) == 1 {
				if _, err := strconv.Atoi(words[0]); err == nil {
					continue // a number such as "thirty-eight"
				}
			}
			spoken := strings.Join(words, " ")
			if !seen[spoken] {
				seen[spoken] = true
				out = append(out, spoken)
			}
		}
	}
	sort.Strings(out[1:])
	return out
}

// WriteSTTScorecard writes the recognition scorecard: engines, totals per engine and set,
// and every transcript.
func WriteSTTScorecard(w io.Writer, infos []EngineInfo, ts []Transcript, sets []string) {
	b := &strings.Builder{}
	b.WriteString("## Engines\n\n| Engine | Helper | Info | Note |\n| --- | --- | --- | --- |\n")
	for _, e := range infos {
		helper := e.Helper + " " + e.Version
		if e.Err != nil {
			helper = "not runnable: " + cell(e.Err.Error())
		}
		fmt.Fprintf(b, "| %s | %s | %s | %s |\n", e.Name, helper, cell(string(e.Info)), cell(e.Note))
	}
	b.WriteString("\n## Totals\n\n| Engine | Set | Files | Failed | WER | Koinon-term WER | Median latency | Median RTF |\n| --- | --- | --- | --- | --- | --- | --- | --- |\n")
	for _, e := range infos {
		if e.Err != nil {
			continue
		}
		for _, set := range sets {
			var s Score
			var lat []time.Duration
			var rtf []float64
			files, failed := 0, 0
			for _, t := range ts {
				if t.Engine != e.Name || t.Set != set {
					continue
				}
				files++
				if t.Err != nil {
					failed++
				}
				s.Add(t.Score)
				lat = append(lat, t.Latency)
				rtf = append(rtf, t.RTF())
			}
			if files == 0 {
				continue
			}
			fmt.Fprintf(b, "| %s | %s | %d | %d | %.1f%% (%d/%d) | %.1f%% (%d/%d) | %d ms | %.3f |\n", e.Name, set, files, failed,
				100*s.WER(), s.Errors, s.Words, 100*s.TermWER(), s.TermErrors, s.TermWords, medianDuration(lat).Milliseconds(), medianFloat(rtf))
		}
	}
	b.WriteString("\n## Transcripts\n\nErrors are word errors over reference words, then the same on Koinon terms.\n\n| Phrase | Set | Engine | Errors | Term errors | Latency | Transcript |\n| --- | --- | --- | --- | --- | --- | --- |\n")
	sorted := append([]Transcript(nil), ts...)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].Phrase.N != sorted[j].Phrase.N {
			return sorted[i].Phrase.N < sorted[j].Phrase.N
		}
		return sorted[i].Set < sorted[j].Set
	})
	for _, t := range sorted {
		text := cell(t.Text)
		if t.Err != nil {
			text = "failed: " + cell(t.Err.Error())
		}
		fmt.Fprintf(b, "| %02d | %s | %s | %d/%d | %d/%d | %d ms | %s |\n", t.Phrase.N, t.Set, t.Engine, t.Score.Errors, t.Score.Words,
			t.Score.TermErrors, t.Score.TermWords, t.Latency.Milliseconds(), text)
	}
	io.WriteString(w, b.String())
}

func cell(s string) string {
	return strings.NewReplacer("|", "\\|", "\n", " ").Replace(s)
}

// medianDuration is the sample median: the middle value, or the mean of the two middle
// values of an even count.
func medianDuration(ds []time.Duration) time.Duration {
	fs := make([]float64, len(ds))
	for i, d := range ds {
		fs[i] = float64(d)
	}
	return time.Duration(medianFloat(fs))
}

// medianFloat is the sample median of fs.
func medianFloat(fs []float64) float64 {
	if len(fs) == 0 {
		return 0
	}
	s := append([]float64(nil), fs...)
	sort.Float64s(s)
	if n := len(s); n%2 == 0 {
		return (s[n/2-1] + s[n/2]) / 2
	}
	return s[len(s)/2]
}
