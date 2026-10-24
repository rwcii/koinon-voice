package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/rwcii/koinon-voice/internal/summarize"
)

// newSpeaker makes the voice that speaks summaries; tests replace it so no test plays audio
// (WDR 006).
var newSpeaker = func(helper *summarize.Launcher, voice string) summarize.Speaker {
	return &summarize.Voice{Helper: helper, Voice: voice}
}

// keys reads API keys; tests replace it so no test reads the Keychain (WDR 006).
var keys summarize.Keys = summarize.Keychain{}

// shuffle orders a blind round; tests replace it.
var shuffle = func(n int, swap func(i, j int)) { rand.Shuffle(n, swap) }

const summarizeUsage = `Usage: koinon-voice summarize (--fixtures DIR | --item FILE) [flags]

Summarize decision items with each configured engine and write a scorecard.

`

func runSummarize(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("summarize", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprint(stderr, summarizeUsage)
		fs.PrintDefaults()
	}
	engines := fs.String("engines", defaultEngines(), "engines file")
	fixtures := fs.String("fixtures", "", "directory of fixture items with answer keys")
	itemFile := fs.String("item", "", "one decision item exported as JSON")
	only := fs.String("engine", "", "comma-separated engine names to run (default all)")
	out := fs.String("out", "", "write the scorecard to this file (default stdout)")
	timeout := fs.Duration("timeout", 90*time.Second, "per-request timeout")
	doSpeak := fs.Bool("speak", false, "speak each summary through the Apple helper as it streams, and time the first spoken word")
	helper := fs.String("helper", "", "the Apple helper's path (default: helper in the engines file)")
	voice := fs.String("voice", "", "an AVSpeechSynthesisVoice identifier (default: voice in the engines file, else the system's en-US voice)")
	blind := fs.Bool("blind", false, "hide engine names and ask for a 1 to 5 listenability rating per summary; q stops rating")
	rateItems := fs.Int("rate", 5, "with --blind, the number of items to rate (0 rates all)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 0 || (*fixtures == "") == (*itemFile == "") {
		fs.Usage()
		return 2
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
	config, err := summarize.LoadConfig(*engines)
	var rows []summarize.Row
	if err == nil {
		rows, err = selectRows(config.Engine, *only)
	}
	if err != nil {
		fmt.Fprintln(stderr, "koinon-voice summarize:", err)
		return 1
	}

	ctx := context.Background()
	launcher := &summarize.Launcher{Path: config.Helper}
	if *helper != "" {
		launcher.Path = *helper
	}
	if *voice == "" {
		*voice = config.Voice
	}
	defer launcher.Close()
	configured := make([]summarize.Configured, len(rows))
	for i, r := range rows {
		e, err := summarize.Build(ctx, r, summarize.Env{Keys: keys, Helper: launcher})
		configured[i] = summarize.Configured{Row: r, Engine: e, Err: err}
		if err != nil {
			fmt.Fprintf(stderr, "%s: %v\n", r.Name, err)
		}
	}

	opts := summarize.Options{Timeout: *timeout, Label: !*blind}
	var speaker summarize.Speaker
	if *doSpeak {
		speaker = newSpeaker(launcher, *voice)
		opts.Voice = speaker
	}
	var r *rater
	if *blind {
		// Engines take turns in a new order for each item, so a listener cannot tell them
		// apart by their place, and no line names them until the ratings are in. Each
		// summary is rated right after it is heard.
		opts.Order = shuffle
		r = &rater{in: bufio.NewScanner(stdin), out: stdout, items: *rateItems}
		opts.After = r.after
	}
	results, err := summarize.Run(ctx, configured, items, opts)
	if r != nil {
		r.printKey()
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

// rater runs the blind listenability rating during the run: for the first items items (all
// when 0), each summary is shown under a letter in the run's shuffled order and rated 1 to
// 5 after it has been heard. A q stops rating; the run goes on and keeps the ratings so far.
type rater struct {
	in      *bufio.Scanner
	out     io.Writer
	items   int
	item    string
	started int
	letter  int
	stopped bool
	key     []string
}

func (r *rater) after(res *summarize.Result) error {
	if r.stopped {
		return nil
	}
	if res.Item != r.item {
		if r.items > 0 && r.started == r.items {
			r.stopped = true
			fmt.Fprintln(r.out, "Rating done; the run goes on.")
			return nil
		}
		r.item, r.letter = res.Item, 0
		r.started++
		fmt.Fprintf(r.out, "Item %s\n", res.Item)
	}
	if res.Err != nil {
		return nil
	}
	label := string(rune('A' + r.letter))
	r.letter++
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
		fmt.Fprintf(stdout, "Rating for %s (1 to 5, q to stop): ", label)
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
