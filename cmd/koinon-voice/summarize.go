package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/rwcii/koinon-voice/internal/kokoro"
	"github.com/rwcii/koinon-voice/internal/speech"
	"github.com/rwcii/koinon-voice/internal/summarize"
)

// newSpeaker checks the speaker and returns it as the voice that speaks summaries; tests
// replace it so no test plays audio (WDR 006).
var newSpeaker = func(s *speech.Speaker) (summarize.Speaker, error) {
	return s, s.Check()
}

// Where the Kokoro helper and its model files are; tests replace them (WDR 006).
var (
	kokoroHelper = speech.DefaultKokoroHelper
	kokoroModels = kokoro.DefaultDir
)

// speakerFor is the speaker of the engines file's [speech]; a non-empty engine or voice, or
// a speed above 0, replaces the file's.
func speakerFor(config summarize.Config, launcher *summarize.Launcher, engine, voice string, speed float64) (*speech.Speaker, error) {
	if engine != "" && engine != summarize.SpeechKokoro && engine != summarize.SpeechApple {
		return nil, fmt.Errorf("speech engine %q is not kokoro or apple", engine)
	}
	if engine != "" && engine != config.Speaking().Engine {
		// The file's voice and speed are its own engine's.
		config.Speech = summarize.SpeechConfig{Engine: engine}
	}
	if voice != "" {
		config.Speech.Voice = voice
	}
	if speed > 0 {
		config.Speech.Speed = speed
	}
	return &speech.Speaker{Speech: config.Speaking(), Apple: launcher, KokoroHelper: kokoroHelper(), Models: kokoroModels()}, nil
}

// keys reads API keys; tests replace it so no test reads the Keychain (WDR 006).
var keys summarize.Keys = summarize.Keychain{}

// shuffle orders a blind round; tests replace it.
var shuffle = func(n int, swap func(i, j int)) { rand.Shuffle(n, swap) }

// runSummarize is koinon-voice summarize. Summarize decision items with each configured engine
// and write a scorecard.
func runSummarize(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flags("summarize")
	engines := fs.String("engines", defaultEngines(), "engines file")
	fixtures := fs.String("fixtures", "", "directory of fixture items with answer keys")
	itemFile := fs.String("item", "", "one decision item exported as JSON")
	only := fs.String("engine", "", "comma-separated engine names to run (default all)")
	out := fs.String("out", "", "write the scorecard to this file (default stdout)")
	timeout := fs.Duration("timeout", 90*time.Second, "per-request timeout")
	doSpeak := fs.Bool("speak", false, "speak each summary with the engines file's [speech] engine as it streams, and time the first spoken word; Ctrl-C stops")
	helper := fs.String("helper", "", "the Apple helper's path (default: helper in the engines file)")
	voice := fs.String("voice", "", "the speech engine's voice: a Kokoro voice, or an AVSpeechSynthesisVoice identifier for engine = \"apple\" (default: from the engines file)")
	blind := fs.Bool("blind", false, "hide engine names, show each rated item first, and ask for a rating per summary, 1 worst to 5 best; q stops rating")
	rateItems := fs.Int("rate", 5, "with --blind, the number of items to rate (0 rates all)")
	rescore := fs.String("rescore", "", "with --fixtures, check this scorecard's recorded summaries again without running an engine, and write it to --out or stdout")
	if err := fs.Parse(args); err != nil {
		return badOptions(stderr)
	}
	if fs.NArg() != 0 || (*fixtures == "") == (*itemFile == "") || (*rescore != "" && *fixtures == "") {
		return badOptions(stderr)
	}

	var items []summarize.Fixture
	var err error
	if *fixtures != "" {
		items, err = summarize.LoadFixtures(*fixtures)
	} else {
		var f summarize.Fixture
		f, err = summarize.LoadItem(*itemFile)
		items = []summarize.Fixture{f}
	}
	if err != nil {
		fmt.Fprintln(stderr, "koinon-voice summarize:", err)
		return 1
	}
	if *rescore != "" {
		return runRescore(*rescore, items, *out, stdout, stderr)
	}
	config, err := summarize.LoadConfig(*engines)
	var rows []summarize.Row
	if err == nil {
		rows, err = selectRows(config.Engine, *only)
	}
	if err != nil {
		fmt.Fprintln(stderr, "koinon-voice summarize:", err)
		return 1
	}

	// Ctrl-C stops the speech and the run; the command exits once the helpers have stopped.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	launcher := &summarize.Launcher{Path: config.Helper}
	if *helper != "" {
		launcher.Path = *helper
	}
	defer launcher.Close()
	var speaker summarize.Speaker
	if *doSpeak {
		s, err := speakerFor(config, launcher, "", *voice, 0)
		if err == nil {
			speaker, err = newSpeaker(s)
		}
		if err != nil {
			fmt.Fprintln(stderr, "koinon-voice summarize:", err)
			return 1
		}
	}
	configured := make([]summarize.Configured, len(rows))
	for i, r := range rows {
		e, err := summarize.Build(ctx, r, summarize.Env{Keys: keys, Helper: launcher})
		configured[i] = summarize.Configured{Row: r, Engine: e, Err: err}
		if err != nil {
			fmt.Fprintf(stderr, "%s: %v\n", r.Name, err)
		}
	}

	opts := summarize.Options{Timeout: *timeout, Label: !*blind, Voice: speaker}
	var r *rater
	if *blind {
		// Engines take turns in a new order for each item, so a listener cannot tell them
		// apart by their place, and no line names them until the ratings are in. The item
		// is shown before its summaries, and each summary is rated right after it is heard.
		opts.Order = shuffle
		r = &rater{in: bufio.NewScanner(stdin), out: stdout, items: *rateItems}
		opts.Before = r.before
		opts.After = r.after
	}
	results, err := summarize.Run(ctx, configured, items, opts)
	if r != nil {
		r.printKey()
	}
	if ctx.Err() != nil {
		fmt.Fprintln(stderr, "koinon-voice summarize: stopped")
		return 1
	}
	if err != nil {
		fmt.Fprintln(stderr, "koinon-voice summarize:", err)
		return 1
	}
	if !*blind {
		for _, r := range results {
			fmt.Fprintln(stdout, line(r))
		}
	}

	w := stdout
	if *out != "" {
		f, err := os.Create(*out)
		if err != nil {
			fmt.Fprintln(stderr, "koinon-voice summarize:", err)
			return 1
		}
		defer f.Close()
		w = f
	}
	if err := summarize.WriteScorecard(w, results, rows); err != nil {
		fmt.Fprintln(stderr, "koinon-voice summarize:", err)
		return 1
	}
	return 0
}

func defaultEngines() string {
	// ~/.config on every system, as chunk 02 names it, not os.UserConfigDir.
	home, err := os.UserHomeDir()
	if err != nil {
		return "engines.toml"
	}
	return filepath.Join(home, ".config", "koinon-voice", "engines.toml")
}

func selectRows(rows []summarize.Row, only string) ([]summarize.Row, error) {
	if only == "" {
		return rows, nil
	}
	byName := map[string]summarize.Row{}
	for _, r := range rows {
		byName[r.Name] = r
	}
	var out []summarize.Row
	for _, name := range strings.Split(only, ",") {
		r, ok := byName[strings.TrimSpace(name)]
		if !ok {
			return nil, fmt.Errorf("no engine named %q", name)
		}
		out = append(out, r)
	}
	return out, nil
}

func line(r summarize.Result) string {
	if r.Err != nil {
		return fmt.Sprintf("%s\t%s\tfailed: %v", r.Item, r.Row.Name, r.Err)
	}
	s := r.Summary
	// An external engine is labeled on screen; in speech, Run says "Summary by PROVIDER."
	// after its summary (ADR 003, Decision 2; PDR 004, Decision 1).
	where := "local"
	if !r.Row.Local {
		where = "external: " + r.Row.Provider
	}
	spoken := ""
	switch {
	case r.Spoken != nil:
		spoken = fmt.Sprintf("\tfirst spoken word %d ms", r.Spoken.FirstWord.Milliseconds())
	case r.SpeechErr != nil:
		spoken = fmt.Sprintf("\tspeech failed: %v", r.SpeechErr)
	}
	return fmt.Sprintf("%s\t%s\t%s%s\tfirst token %d ms\ttotal %d ms\t%s", r.Item, r.Row.Name, where, spoken, s.FirstToken.Milliseconds(), s.Total.Milliseconds(), s.Text)
}

// errStop is a q at a rating prompt: rating ends and the ratings so far are kept.
var errStop = errors.New("stop")

// rater runs the blind rating during the run: for the first items items (all when 0), the
// item is shown as the engines receive it, so the summaries can be judged against it for
// accuracy as well as listening, and then each summary is shown under a letter in the run's
// shuffled order and rated 1 to 5 after it has been heard. A q stops rating; the run goes on
// and keeps the ratings so far.
type rater struct {
	in      *bufio.Scanner
	out     io.Writer
	items   int
	started int
	letter  int
	stopped bool
	key     []string
}

func (r *rater) before(item summarize.Fixture) error {
	if r.stopped {
		return nil
	}
	if r.items > 0 && r.started == r.items {
		r.stopped = true
		fmt.Fprintln(r.out, "Rating done; the run goes on.")
		return nil
	}
	r.letter = 0
	r.started++
	fmt.Fprintf(r.out, "Item %s\n%s\n", item.Name, summarize.Prompt(item.Item))
	return nil
}

func (r *rater) after(res *summarize.Result) error {
	if r.stopped {
		return nil
	}
	if res.Err != nil {
		return nil
	}
	label := string(rune('A' + r.letter))
	r.letter++
	if res.SpeechErr != nil {
		// A summary that was not heard is not rated for listening; its text stays hidden
		// so it is not judged by reading. The speech error names no engine.
		fmt.Fprintf(r.out, "%s: playback failed (%v); not rated.\n", label, res.SpeechErr)
		return nil
	}
	fmt.Fprintf(r.out, "%s: %s\n", label, res.Summary.Text)
	n, err := ask(r.in, r.out, label)
	if errors.Is(err, errStop) {
		r.stopped = true
		fmt.Fprintln(r.out, "Rating stopped; the run goes on.")
		return nil
	}
	if err != nil {
		return err
	}
	res.Rating = n
	r.key = append(r.key, fmt.Sprintf("%s %s = %s", res.Item, label, res.Row.Name))
	return nil
}

func (r *rater) printKey() {
	fmt.Fprintln(r.out, "Engines:")
	for _, k := range r.key {
		fmt.Fprintln(r.out, k)
	}
}

func ask(in *bufio.Scanner, stdout io.Writer, label string) (int, error) {
	for {
		fmt.Fprintf(stdout, "Rating for %s (1 worst to 5 best, q to stop): ", label)
		if !in.Scan() {
			if err := in.Err(); err != nil {
				return 0, err
			}
			return 0, errors.New("ratings ended before every summary was rated")
		}
		answer := strings.TrimSpace(in.Text())
		if answer == "q" {
			return 0, errStop
		}
		n, err := strconv.Atoi(answer)
		if err == nil && n >= 1 && n <= 5 {
			return n, nil
		}
	}
}

// runRescore checks a scorecard's recorded summaries again and lists each changed verdict on
// stderr.
func runRescore(path string, items []summarize.Fixture, out string, stdout, stderr io.Writer) int {
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintln(stderr, "koinon-voice summarize:", err)
		return 1
	}
	text, changes, err := summarize.Rescore(string(data), items)
	if err != nil {
		fmt.Fprintf(stderr, "koinon-voice summarize: %s: %v\n", path, err)
		return 1
	}
	if out == "" {
		_, err = io.WriteString(stdout, text)
	} else {
		err = os.WriteFile(out, []byte(text), 0o644)
	}
	if err != nil {
		fmt.Fprintln(stderr, "koinon-voice summarize:", err)
		return 1
	}
	for _, c := range changes {
		fmt.Fprintln(stderr, c)
	}
	fmt.Fprintf(stderr, "%d verdicts changed\n", len(changes))
	return 0
}
