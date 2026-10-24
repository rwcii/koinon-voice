package main

import (
	"bufio"
	"context"
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

const benchUsage = `Usage: koinon-voice bench (stt | tts) [flags]

stt runs each recognition engine over the recorded corpus and writes word error rate,
overall and on Koinon terms, latency and real-time factor. tts speaks each synthesis text
with each synthesis engine, in a shuffled order with the engines' names hidden, plays it and
asks for a 1 to 5 listenability rating; it writes time to first audio and the ratings.

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
	if len(args) == 0 || (args[0] != "stt" && args[0] != "tts") {
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
