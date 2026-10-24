package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/rwcii/koinon-voice/internal/sidecar"
	"github.com/rwcii/koinon-voice/internal/speech"
	"github.com/rwcii/koinon-voice/internal/summarize"
)

const benchUsage = `Usage: koinon-voice bench (stt | tts | wake | interrupt) [flags]

stt runs each recognition engine over the recorded corpus and writes word error rate,
overall and on Koinon terms, latency and real-time factor. tts speaks each synthesis text
with each synthesis engine, in a shuffled order with the engines' names hidden, plays it and
asks for a 1 to 5 listenability rating; it writes time to first audio and the ratings. wake
runs each wake detector over the recorded "Koinon" takes and background recordings and writes
misses per distance and false wakes per hour. interrupt speaks a summary through the
speakers while listening with echo cancellation, for the maintainer to interrupt; with
--control nobody speaks, and any transcript is the bridge's own speech leaking through.

`

const ttsFile = "docs/sprints/2026-10-23-phase0-spike/tts-summaries.md"

// playAudio plays a file to the output device; tests replace it so no test plays audio.
var playAudio = func(ctx context.Context, path string) error {
	return exec.CommandContext(ctx, "afplay", path).Run()
}

func defaultShare(parts ...string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(parts...)
	}
	return filepath.Join(append([]string{home, ".local", "share", "koinon-voice"}, parts...)...)
}

type benchFlags struct {
	engines, helper, python, script, only, out string
	timeout, warmup                            time.Duration
}

func (b *benchFlags) register(fs *flag.FlagSet) {
	fs.StringVar(&b.engines, "engines", defaultEngines(), "engines file, for the Apple helper's path and voice")
	fs.StringVar(&b.helper, "helper", "", "the Apple helper's path (default: helper in the engines file)")
	fs.StringVar(&b.python, "python", defaultShare("venv", "bin", "python"), "Python of the bench's virtual environment")
	fs.StringVar(&b.script, "bench-helper", "helpers/bench/bench_helper.py", "the bench helper script")
	fs.StringVar(&b.only, "engine", "", "comma-separated engine names to run (default all)")
	fs.DurationVar(&b.timeout, "timeout", 2*time.Minute, "timeout for each request")
	fs.DurationVar(&b.warmup, "warmup", 10*time.Minute, "timeout for each engine's first request, which loads its model")
}

// starter starts a bench helper for one engine of the Python bench helper.
func (b *benchFlags) starter(engine string) func(ctx context.Context) (*sidecar.Helper, error) {
	return func(ctx context.Context) (*sidecar.Helper, error) {
		return sidecar.Start(ctx, b.python, b.script, engine)
	}
}

func (b *benchFlags) apple() (func(ctx context.Context) (*sidecar.Helper, error), string, error) {
	config, err := summarize.LoadConfig(b.engines)
	if err != nil && b.helper == "" {
		return nil, "", err
	}
	path := b.helper
	if path == "" {
		path = config.Helper
	}
	return func(ctx context.Context) (*sidecar.Helper, error) {
		if path == "" {
			return nil, errors.New("no Apple helper: set helper in the engines file or pass --helper")
		}
		return sidecar.Start(ctx, path)
	}, config.Voice, nil
}

func selected(only, name string) bool {
	if only == "" {
		return true
	}
	for _, n := range strings.Split(only, ",") {
		if strings.TrimSpace(n) == name {
			return true
		}
	}
	return false
}

func runBench(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 || (args[0] != "stt" && args[0] != "tts" && args[0] != "wake" && args[0] != "interrupt") {
		fmt.Fprint(stderr, benchUsage)
		return 2
	}
	fs := flag.NewFlagSet("bench "+args[0], flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprint(stderr, benchUsage)
		fs.PrintDefaults()
	}
	var b benchFlags
	b.register(fs)
	if args[0] == "stt" {
		phrases := fs.String("phrases", phrasesFile, "phrase list")
		dir := fs.String("dir", defaultCorpus(), "corpus directory")
		sets := fs.String("sets", strings.Join(speech.Sets, ","), "comma-separated corpus sets to score")
		fs.StringVar(&b.out, "out", "scorecard-speech-stt.md", "scorecard file")
		if err := fs.Parse(args[1:]); err != nil || fs.NArg() != 0 {
			fs.Usage()
			return 2
		}
		return benchSTT(&b, *phrases, *dir, strings.Split(*sets, ","), stdout, stderr)
	}
	if args[0] == "interrupt" {
		texts := fs.String("texts", ttsFile, "synthesis texts")
		n := fs.Int("text", 3, "the synthesis text to speak")
		say := fs.String("say", interruptPhrase, "what the maintainer says to interrupt")
		control := fs.Bool("control", false, "nobody speaks: measure the bridge's own speech leaking through")
		seconds := fs.Float64("seconds", 25, "how long to listen")
		dir := fs.String("dir", defaultShare("interruption"), "directory for the captured audio")
		fs.StringVar(&b.out, "out", "interruption-run.md", "report file")
		if err := fs.Parse(args[1:]); err != nil || fs.NArg() != 0 || *seconds <= 0 {
			fs.Usage()
			return 2
		}
		return benchInterrupt(&b, *texts, *n, *say, *control, *seconds, *dir, bufio.NewScanner(stdin), stdout, stderr)
	}
	if args[0] == "wake" {
		phrases := fs.String("phrases", phrasesFile, "phrase list, for the biasing list")
		dir := fs.String("dir", defaultCorpus(), "corpus directory")
		model := fs.String("model", defaultShare("wake-train", "model", "koinon.onnx"), "openWakeWord model")
		threshold := fs.Float64("threshold", 0.5, "openWakeWord score that counts as a wake")
		fs.StringVar(&b.out, "out", "scorecard-wake.md", "scorecard file")
		if err := fs.Parse(args[1:]); err != nil || fs.NArg() != 0 {
			fs.Usage()
			return 2
		}
		return benchWake(&b, *phrases, *dir, *model, *threshold, stdout, stderr)
	}
	texts := fs.String("texts", ttsFile, "synthesis texts")
	dir := fs.String("dir", defaultShare("bench", "tts"), "directory for the synthesized audio")
	kokoroVoice := fs.String("kokoro-voice", "af_heart", "Kokoro voice")
	rate := fs.Bool("rate", true, "play each rendition and ask for a blind rating")
	fs.StringVar(&b.out, "out", "scorecard-speech-tts.md", "scorecard file")
	if err := fs.Parse(args[1:]); err != nil || fs.NArg() != 0 {
		fs.Usage()
		return 2
	}
	return benchTTS(&b, *texts, *dir, *kokoroVoice, *rate, bufio.NewScanner(stdin), stdout, stderr)
}

func benchSTT(b *benchFlags, phrases, dir string, sets []string, stdout, stderr io.Writer) int {
	list, err := loadPhrases(phrases)
	if err != nil {
		fmt.Fprintln(stderr, "koinon-voice bench:", err)
		return 1
	}
	raw, _ := os.ReadFile(phrases)
	keywords := speech.Keywords(string(raw))
	files, skipped, err := speech.CorpusFiles(speech.Corpus{Dir: dir}, list, sets)
	if err != nil {
		fmt.Fprintln(stderr, "koinon-voice bench:", err)
		return 1
	}
	for _, s := range skipped {
		fmt.Fprintln(stderr, "skipped", s)
	}
	if len(files) == 0 {
		fmt.Fprintln(stderr, "koinon-voice bench: no corpus files to score; record them with koinon-voice corpus record")
		return 1
	}
	apple, _, err := b.apple()
	if err != nil {
		fmt.Fprintln(stderr, "koinon-voice bench:", err)
		return 1
	}
	path := func(p string) any { return map[string]any{"path": p} }
	all := []speech.Recognizer{
		{Name: "apple", Note: "SpeechAnalyzer, en-US", Start: apple, Params: path},
		{Name: "apple-keywords", Note: "SpeechAnalyzer, en-US, contextual strings: the Whistle biasing list", Start: apple,
			Params: func(p string) any { return map[string]any{"path": p, "keywords": keywords} }},
		{Name: "apple-dictation-keywords", Note: "SpeechAnalyzer with DictationTranscriber (short dictation), en-US, contextual strings: the Whistle biasing list", Start: apple,
			Params: func(p string) any { return map[string]any{"path": p, "keywords": keywords, "transcriber": "dictation"} }},
		{Name: "parakeet", Note: "Parakeet TDT 0.6B v2 through mlx-audio", Start: b.starter("parakeet"), Params: path},
		{Name: "whistle", Note: "Cactus Whistle, no biasing", Start: b.starter("whistle"), Params: path},
		{Name: "whistle-keywords", Note: "Cactus Whistle, biased to: " + strings.Join(keywords, ", "), Start: b.starter("whistle"),
			Params: func(p string) any { return map[string]any{"path": p, "keywords": keywords} }},
	}
	var engines []speech.Recognizer
	for _, e := range all {
		if selected(b.only, e.Name) {
			engines = append(engines, e)
		}
	}
	fmt.Fprintf(stdout, "Scoring %d files with %d engines.\n", len(files), len(engines))
	infos, ts := speech.RunSTT(context.Background(), engines, files, speech.BenchOptions{
		Timeout: b.timeout, Warmup: b.warmup,
		Progress: func(t speech.Transcript) {
			text := t.Text
			if t.Err != nil {
				text = "failed: " + t.Err.Error()
			}
			fmt.Fprintf(stdout, "%s %02d %s\t%d/%d\t%d ms\t%s\n", t.Set, t.Phrase.N, t.Engine, t.Score.Errors, t.Score.Words, t.Latency.Milliseconds(), text)
		},
	})
	for _, e := range infos {
		if e.Err != nil {
			fmt.Fprintf(stdout, "%s: not runnable: %v\n", e.Name, e.Err)
		}
	}
	return writeCard(b.out, func(w io.Writer) { speech.WriteSTTScorecard(w, infos, ts, sets) }, stdout, stderr)
}

func writeCard(path string, write func(io.Writer), stdout, stderr io.Writer) int {
	f, err := os.Create(path)
	if err != nil {
		fmt.Fprintln(stderr, "koinon-voice bench:", err)
		return 1
	}
	write(f)
	if err := f.Close(); err != nil {
		fmt.Fprintln(stderr, "koinon-voice bench:", err)
		return 1
	}
	fmt.Fprintln(stdout, "Wrote", path)
	return 0
}

func benchTTS(b *benchFlags, textsPath, dir, kokoroVoice string, rate bool, in *bufio.Scanner, stdout, stderr io.Writer) int {
	f, err := os.Open(textsPath)
	if err != nil {
		fmt.Fprintln(stderr, "koinon-voice bench: synthesis texts:", err)
		return 1
	}
	texts, err := speech.ParseTexts(f)
	f.Close()
	if err != nil {
		fmt.Fprintln(stderr, "koinon-voice bench:", err)
		return 1
	}
	apple, voice, err := b.apple()
	if err != nil {
		fmt.Fprintln(stderr, "koinon-voice bench:", err)
		return 1
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		fmt.Fprintln(stderr, "koinon-voice bench:", err)
		return 1
	}
	appleNote := "AVSpeechSynthesizer, the system's en-US voice"
	if voice != "" {
		appleNote = "AVSpeechSynthesizer, " + voice
	}
	all := []speech.Synthesizer{
		{Name: "apple", Note: appleNote, Ext: ".caf", Start: apple, Speak: speech.AppleSpeak(voice)},
		{Name: "kokoro", Note: "Kokoro-82M through mlx-audio, voice " + kokoroVoice, Ext: ".wav", Start: b.starter("kokoro"), Speak: speech.BenchSpeak(kokoroVoice)},
	}
	var synths []speech.Synthesizer
	for _, s := range all {
		if selected(b.only, s.Name) {
			synths = append(synths, s)
		}
	}
	opts := speech.TTSOptions{Timeout: b.timeout, Warmup: b.warmup}
	var key []string
	if rate {
		opts.Order = shuffle
		item, letter, stopped := 0, 0, false
		opts.After = func(r *speech.Rendition) error {
			if r.Text.N != item {
				item, letter = r.Text.N, 0
				fmt.Fprintf(stdout, "\nText %02d\n", r.Text.N)
			}
			label := string(rune('A' + letter))
			letter++
			if r.Err != nil {
				fmt.Fprintf(stdout, "%s: failed; not rated.\n", label)
				return nil
			}
			if stopped {
				return nil
			}
			if err := playAudio(context.Background(), r.Path); err != nil {
				fmt.Fprintf(stdout, "%s: playback failed (%v); not rated.\n", label, err)
				return nil
			}
			n, err := ask(in, stdout, label)
			if errors.Is(err, errStop) {
				stopped = true
				fmt.Fprintln(stdout, "Rating stopped; the run goes on.")
				return nil
			}
			if err != nil {
				return err
			}
			r.Rating = n
			key = append(key, fmt.Sprintf("Text %02d %s = %s", r.Text.N, label, r.Engine))
			return nil
		}
	}
	infos, rs, err := speech.RunTTS(context.Background(), synths, texts, dir, opts)
	if err != nil {
		fmt.Fprintln(stderr, "koinon-voice bench:", err)
		return 1
	}
	for _, e := range infos {
		if e.Err != nil {
			fmt.Fprintf(stdout, "%s: not runnable: %v\n", e.Name, e.Err)
		}
	}
	if rate {
		fmt.Fprintln(stdout, "\nEngines:")
		for _, k := range key {
			fmt.Fprintln(stdout, k)
		}
	}
	return writeCard(b.out, func(w io.Writer) { speech.WriteTTSScorecard(w, infos, rs) }, stdout, stderr)
}

func benchWake(b *benchFlags, phrases, dir, model string, threshold float64, stdout, stderr io.Writer) int {
	raw, err := os.ReadFile(phrases)
	if err != nil {
		fmt.Fprintln(stderr, "koinon-voice bench: phrase list:", err)
		return 1
	}
	keywords := speech.Keywords(string(raw))
	files, skipped, err := speech.WakeFiles(speech.Corpus{Dir: dir})
	if err != nil {
		fmt.Fprintln(stderr, "koinon-voice bench:", err)
		return 1
	}
	for _, s := range skipped {
		fmt.Fprintln(stderr, "skipped", s)
	}
	if len(files) == 0 {
		fmt.Fprintln(stderr, "koinon-voice bench: no wake takes or background recordings; record them with koinon-voice corpus wake and corpus background")
		return 1
	}
	shown := model // the scorecard is committed: no home directory in it
	if home, err := os.UserHomeDir(); err == nil && strings.HasPrefix(model, home+string(filepath.Separator)) {
		shown = "~" + model[len(home):]
	}
	modelNote := "openWakeWord, model " + shown
	if sum, err := speech.Hash(model); err == nil {
		modelNote += " (SHA-256 prefix " + sum[:16]
		// Current exporters keep the weights in an external data file beside the model.
		if data, err := speech.Hash(model + ".data"); err == nil {
			modelNote += "; weights " + filepath.Base(model) + ".data " + data[:16]
		}
		modelNote += ")"
	}
	all := []speech.WakeDetector{
		{Name: "openwakeword", Note: fmt.Sprintf("%s, threshold %.2f", modelNote, threshold), Start: b.starter("openwakeword"),
			Params: func(p string) any { return map[string]any{"path": p, "model": model, "threshold": threshold} }},
		{Name: "phrase", Note: "Silero VAD, then Cactus Whistle biased to: " + strings.Join(keywords, ", ") + "; wakes on the word koinon", Start: b.starter("phrase"),
			Params: func(p string) any { return map[string]any{"path": p, "keywords": keywords} }},
	}
	var detectors []speech.WakeDetector
	for _, d := range all {
		if selected(b.only, d.Name) {
			detectors = append(detectors, d)
		}
	}
	fmt.Fprintf(stdout, "Scoring %d files with %d detectors.\n", len(files), len(detectors))
	infos, runs := speech.RunWake(context.Background(), detectors, files, speech.BenchOptions{Timeout: b.timeout, Warmup: b.warmup})
	for _, e := range infos {
		if e.Err != nil {
			fmt.Fprintf(stdout, "%s: not runnable: %v\n", e.Name, e.Err)
		}
	}
	return writeCard(b.out, func(w io.Writer) { speech.WriteWakeScorecard(w, infos, runs) }, stdout, stderr)
}

// interruptPhrase is what the maintainer says over the bridge: no Koinon term, so a miss is
// the echo canceller's or the transcriber's, not the vocabulary's.
const interruptPhrase = "Stop, wait, I have a question."

func benchInterrupt(b *benchFlags, textsPath string, n int, say string, control bool, seconds float64, dir string, in *bufio.Scanner, stdout, stderr io.Writer) int {
	f, err := os.Open(textsPath)
	if err != nil {
		fmt.Fprintln(stderr, "koinon-voice bench: synthesis texts:", err)
		return 1
	}
	texts, err := speech.ParseTexts(f)
	f.Close()
	if err != nil {
		fmt.Fprintln(stderr, "koinon-voice bench:", err)
		return 1
	}
	var text string
	for _, t := range texts {
		if t.N == n {
			text = t.Text
		}
	}
	if text == "" {
		fmt.Fprintf(stderr, "koinon-voice bench: no synthesis text %d\n", n)
		return 1
	}
	apple, voice, err := b.apple()
	if err != nil {
		fmt.Fprintln(stderr, "koinon-voice bench:", err)
		return 1
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		fmt.Fprintln(stderr, "koinon-voice bench:", err)
		return 1
	}
	kind := "interrupt"
	if control {
		kind, say = "control", ""
	}
	capture := filepath.Join(dir, fmt.Sprintf("%s-%s.wav", kind, time.Now().Format("20060102-150405")))
	if control {
		fmt.Fprintf(stdout, "Control run: the bridge speaks text %02d for about %.0f s and nobody speaks. Press Enter to start.\n", n, seconds)
	} else {
		fmt.Fprintf(stdout, "The bridge will speak text %02d. A few seconds in, interrupt it by saying:\n\n    %s\n\nPress Enter to start.\n", n, say)
	}
	if !in.Scan() {
		return 1
	}
	ctx := context.Background()
	h, err := apple(ctx)
	if err != nil {
		fmt.Fprintln(stderr, "koinon-voice bench:", err)
		return 1
	}
	defer h.Close()
	if !h.Hello().Has("duplex") {
		fmt.Fprintln(stderr, "koinon-voice bench: the Apple helper does not offer duplex; install the current helper")
		return 1
	}
	params := map[string]any{"text": text, "seconds": seconds, "capture": capture, "stop_on_speech": !control}
	if voice != "" {
		params["voice"] = voice
	}
	rctx, cancel := context.WithTimeout(ctx, time.Duration(seconds*float64(time.Second))+b.timeout)
	defer cancel()
	data, err := h.Call(rctx, "duplex", params, func(e sidecar.Event) {
		switch e.Name {
		case "speaking":
			fmt.Fprintln(stdout, "Speaking…")
		case "barge_in":
			fmt.Fprintf(stdout, "Barge-in: %s\n", e.Data)
		case "final":
			fmt.Fprintf(stdout, "Heard: %s\n", e.Data)
		}
	})
	if err != nil {
		fmt.Fprintln(stderr, "koinon-voice bench: duplex:", err)
		return 1
	}
	var r struct {
		Transcript      string  `json:"transcript"`
		Voice           string  `json:"voice"`
		SpokenSeconds   float64 `json:"spoken_seconds"`
		BargeInMS       *int    `json:"barge_in_ms"`
		ListenedSeconds float64 `json:"listened_seconds"`
		VoiceProcessing bool    `json:"voice_processing"`
	}
	if err := json.Unmarshal(data, &r); err != nil {
		fmt.Fprintln(stderr, "koinon-voice bench: duplex result:", err)
		return 1
	}
	score := speech.ScoreInterruption(text, say, r.Transcript)
	sum, _ := speech.Hash(capture)
	if len(sum) > 16 {
		sum = sum[:16]
	}
	barge := "none"
	if r.BargeInMS != nil {
		barge = fmt.Sprintf("%d ms after the start", *r.BargeInMS)
	}
	verdict := "passed"
	if why := score.Verdict(control, r.BargeInMS != nil, r.VoiceProcessing); len(why) > 0 {
		verdict = "failed: " + strings.Join(why, "; ")
	}
	report := fmt.Sprintf(`## %s run, %s

- Bridge text %02d, voice %s, %.1f s of speech; listened %.1f s; voice processing %v.
- Interruption said: %q
- Barge-in: %s.
- Transcript: %q
- Interruption words heard: %v; missed: %v.
- Bridge words in the transcript: %v.
- Result: %s.
- Captured input: SHA-256 prefix %s (not in Git).
`, kind, time.Now().Format("2006-01-02 15:04"), n, r.Voice, r.SpokenSeconds, r.ListenedSeconds, r.VoiceProcessing,
		say, barge, r.Transcript, score.Heard, score.Missed, score.Leaked, verdict, sum)
	fmt.Fprint(stdout, "\n"+report)
	out, err := os.OpenFile(b.out, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		fmt.Fprintln(stderr, "koinon-voice bench:", err)
		return 1
	}
	fmt.Fprint(out, report+"\n")
	out.Close()
	fmt.Fprintln(stdout, "Appended to", b.out)
	return 0
}
