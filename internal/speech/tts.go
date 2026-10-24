package speech

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rwcii/koinon-voice/internal/sidecar"
)

// Text is one numbered synthesis text. A trailing "[source]" names where it came from and
// is not spoken.
type Text struct {
	N      int
	Text   string
	Source string
}

var sourceNote = regexp.MustCompile(`\s*\[([^\]]+)\]$`)

// ParseTexts reads the synthesis list: one text per line as "NN. text [source]".
func ParseTexts(r io.Reader) ([]Text, error) {
	var out []Text
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64<<10), 1<<20)
	for sc.Scan() {
		m := phraseLine.FindStringSubmatch(strings.TrimSpace(sc.Text()))
		if m == nil {
			continue
		}
		n, _ := strconv.Atoi(m[1])
		t := Text{N: n, Text: m[2]}
		if s := sourceNote.FindStringSubmatch(t.Text); s != nil {
			t.Source = s[1]
			t.Text = strings.TrimSpace(t.Text[:len(t.Text)-len(s[0])])
		}
		out = append(out, t)
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no texts")
	}
	return out, nil
}

// Synthesizer is one synthesis engine of the bench.
type Synthesizer struct {
	Name, Note string
	// Ext is the audio file extension the engine writes.
	Ext   string
	Start func(ctx context.Context) (*sidecar.Helper, error)
	// Speak synthesizes text into the file at output and reports its timing.
	Speak func(ctx context.Context, h *sidecar.Helper, text, output string) (Spoken, error)
}

// Spoken is one synthesis's timing: FirstAudio as the helper measured it, from its request
// to the first audio buffer, and the audio's length.
type Spoken struct {
	FirstAudio   time.Duration
	AudioSeconds float64
}

// Rendition is one engine speaking one text.
type Rendition struct {
	Engine string
	Text   Text
	Path   string
	Spoken
	Rating int
	Err    error
}

// TTSOptions bounds the synthesis bench.
type TTSOptions struct {
	Timeout, Warmup time.Duration
	// Order shuffles the engines for each text (rand.Shuffle's shape); nil keeps their order.
	Order func(n int, swap func(i, j int))
	// After runs once each rendition is made, in the shuffled order: the blind rating
	// plays and rates it there. An error stops the bench.
	After func(*Rendition) error
}

// RunTTS speaks every text with every engine into dir, one at a time, in an order shuffled
// per text. Each engine is warmed up with a short text first, which is not scored.
func RunTTS(ctx context.Context, synths []Synthesizer, texts []Text, dir string, opts TTSOptions) ([]EngineInfo, []Rendition, error) {
	infos := make([]EngineInfo, len(synths))
	helpers := make([]*sidecar.Helper, len(synths))
	var mu sync.Mutex
	defer func() {
		mu.Lock()
		defer mu.Unlock()
		for _, h := range helpers {
			if h != nil {
				h.Close()
			}
		}
	}()
	start := func(i int) error {
		s := synths[i]
		h, err := s.Start(ctx)
		if err != nil {
			return err
		}
		mu.Lock()
		helpers[i] = h
		mu.Unlock()
		hello := h.Hello()
		infos[i].Helper, infos[i].Version, infos[i].Info = hello.Helper, hello.Version, hello.Info
		return nil
	}
	for i, s := range synths {
		infos[i] = EngineInfo{Name: s.Name, Note: s.Note}
		if err := start(i); err != nil {
			infos[i].Err = err
			continue
		}
		wctx, cancel := context.WithTimeout(ctx, opts.Warmup)
		_, err := s.Speak(wctx, helpers[i], "Warming up.", filepath.Join(dir, "warmup-"+s.Name+s.Ext))
		cancel()
		if err != nil {
			infos[i].Err = fmt.Errorf("warm-up: %w", err)
		}
	}
	var out []Rendition
	for _, t := range texts {
		order := make([]int, len(synths))
		for i := range order {
			order[i] = i
		}
		if opts.Order != nil {
			opts.Order(len(order), func(i, j int) { order[i], order[j] = order[j], order[i] })
		}
		for _, i := range order {
			s := synths[i]
			if infos[i].Err != nil {
				continue
			}
			r := Rendition{Engine: s.Name, Text: t, Path: filepath.Join(dir, fmt.Sprintf("%02d-%s%s", t.N, s.Name, s.Ext))}
			if helpers[i].Err() != nil {
				helpers[i].Close()
				if err := start(i); err != nil {
					r.Err = err
				}
			}
			if r.Err == nil {
				rctx, cancel := context.WithTimeout(ctx, opts.Timeout)
				r.Spoken, r.Err = s.Speak(rctx, helpers[i], t.Text, r.Path)
				cancel()
			}
			out = append(out, r)
			if opts.After != nil {
				if err := opts.After(&out[len(out)-1]); err != nil {
					return infos, out, err
				}
			}
		}
	}
	return infos, out, nil
}

// firstAudio reads a first_audio event's helper-measured time.
func firstAudio(e sidecar.Event) (time.Duration, bool) {
	if e.Name != "first_audio" {
		return 0, false
	}
	var d struct {
		SinceRequestMS float64 `json:"since_request_ms"`
	}
	if json.Unmarshal(e.Data, &d) != nil {
		return 0, false
	}
	return time.Duration(d.SinceRequestMS * float64(time.Millisecond)), true
}

// AppleSpeak speaks through the Apple helper's speak request, writing the file at output.
func AppleSpeak(voice string) func(ctx context.Context, h *sidecar.Helper, text, output string) (Spoken, error) {
	return func(ctx context.Context, h *sidecar.Helper, text, output string) (Spoken, error) {
		var sp Spoken
		var mu sync.Mutex
		params := map[string]string{"output": output}
		if voice != "" {
			params["voice"] = voice
		}
		s, err := h.Open(ctx, "speak", params, func(e sidecar.Event) {
			if d, ok := firstAudio(e); ok {
				mu.Lock()
				if sp.FirstAudio == 0 {
					sp.FirstAudio = d
				}
				mu.Unlock()
			}
		})
		if err != nil {
			return sp, err
		}
		if err := s.Send(map[string]string{"text": text}); err != nil {
			return sp, err
		}
		if err := s.CloseInput(); err != nil {
			return sp, err
		}
		data, err := s.Wait()
		if err != nil {
			return sp, err
		}
		var r struct {
			AudioSeconds float64 `json:"audio_seconds"`
		}
		json.Unmarshal(data, &r)
		mu.Lock()
		defer mu.Unlock()
		sp.AudioSeconds = r.AudioSeconds
		if sp.FirstAudio == 0 {
			return sp, fmt.Errorf("the helper reported no first audio")
		}
		return sp, nil
	}
}

// BenchSpeak speaks through a bench helper's synthesize request.
func BenchSpeak(voice string) func(ctx context.Context, h *sidecar.Helper, text, output string) (Spoken, error) {
	return func(ctx context.Context, h *sidecar.Helper, text, output string) (Spoken, error) {
		var sp Spoken
		var mu sync.Mutex
		data, err := h.Call(ctx, "synthesize", map[string]string{"text": text, "voice": voice, "output": output}, func(e sidecar.Event) {
			if d, ok := firstAudio(e); ok {
				mu.Lock()
				if sp.FirstAudio == 0 {
					sp.FirstAudio = d
				}
				mu.Unlock()
			}
		})
		if err != nil {
			return sp, err
		}
		var r struct {
			AudioSeconds float64 `json:"audio_seconds"`
		}
		json.Unmarshal(data, &r)
		mu.Lock()
		defer mu.Unlock()
		sp.AudioSeconds = r.AudioSeconds
		if sp.FirstAudio == 0 {
			return sp, fmt.Errorf("the helper reported no first audio")
		}
		return sp, nil
	}
}

// WriteTTSScorecard writes the synthesis scorecard: engines, totals and every rendition.
func WriteTTSScorecard(w io.Writer, infos []EngineInfo, rs []Rendition) {
	b := &strings.Builder{}
	b.WriteString("## Engines\n\n| Engine | Helper | Info | Note |\n| --- | --- | --- | --- |\n")
	for _, e := range infos {
		helper := e.Helper + " " + e.Version
		if e.Err != nil {
			helper = "not runnable: " + cell(e.Err.Error())
		}
		fmt.Fprintf(b, "| %s | %s | %s | %s |\n", e.Name, helper, cell(string(e.Info)), cell(e.Note))
	}
	b.WriteString("\n## Totals\n\n| Engine | Texts | Failed | Median first audio | Mean rating |\n| --- | --- | --- | --- | --- |\n")
	for _, e := range infos {
		if e.Err != nil {
			continue
		}
		n, failed, rated, sum := 0, 0, 0, 0
		var first []time.Duration
		for _, r := range rs {
			if r.Engine != e.Name {
				continue
			}
			n++
			if r.Err != nil {
				failed++
				continue
			}
			first = append(first, r.FirstAudio)
			if r.Rating > 0 {
				rated++
				sum += r.Rating
			}
		}
		rating := ""
		if rated > 0 {
			rating = fmt.Sprintf("%.1f (%d)", float64(sum)/float64(rated), rated)
		}
		fmt.Fprintf(b, "| %s | %d | %d | %d ms | %s |\n", e.Name, n, failed, medianDuration(first).Milliseconds(), rating)
	}
	b.WriteString("\n## Renditions\n\n| Text | Engine | First audio | Audio | Rating |\n| --- | --- | --- | --- | --- |\n")
	for _, r := range rs {
		first, audio, rating := "", "", ""
		if r.Err != nil {
			first = "failed: " + cell(r.Err.Error())
		} else {
			first = fmt.Sprintf("%d ms", r.FirstAudio.Milliseconds())
			audio = fmt.Sprintf("%.1f s", r.AudioSeconds)
		}
		if r.Rating > 0 {
			rating = strconv.Itoa(r.Rating)
		}
		fmt.Fprintf(b, "| %02d | %s | %s | %s | %s |\n", r.Text.N, r.Engine, first, audio, rating)
	}
	io.WriteString(w, b.String())
}
