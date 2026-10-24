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
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/rwcii/koinon-voice/internal/summarize"
)

// speak plays a summary; tests replace it.
var speak = func(ctx context.Context, text string) error {
	return exec.CommandContext(ctx, "say", text).Run()
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
	doSpeak := fs.Bool("speak", false, "play each summary with macOS say")
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
	rows, err := summarize.LoadEngines(*engines)
	if err == nil {
		rows, err = selectRows(rows, *only)
	}
	if err != nil {
		fmt.Fprintln(stderr, "koinon-voice summarize:", err)
		return 1
	}

	ctx := context.Background()
	configured := make([]summarize.Configured, len(rows))
	for i, r := range rows {
		e, err := summarize.Build(ctx, r, keys, nil)
		configured[i] = summarize.Configured{Row: r, Engine: e, Err: err}
		if err != nil {
			fmt.Fprintf(stderr, "%s: %v\n", r.Name, err)
		}
	}

	results := summarize.Run(ctx, configured, items, *timeout)
	if *blind {
		if err := rate(ctx, results, stdin, stdout, *doSpeak, *rateItems); err != nil {
			fmt.Fprintln(stderr, "koinon-voice summarize:", err)
			return 1
		}
	} else {
		for _, r := range results {
			fmt.Fprintln(stdout, line(r))
			if *doSpeak && r.Err == nil {
				if err := speak(ctx, spoken(r)); err != nil {
					fmt.Fprintln(stderr, "say:", err)
				}
			}
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
	where := "local"
	if !r.Row.Local {
		where = "external: " + r.Row.Provider
	}
	return fmt.Sprintf("%s\t%s\t%s\tfirst token %d ms\ttotal %d ms\t%s", r.Item, r.Row.Name, where, s.FirstToken.Milliseconds(), s.Total.Milliseconds(), s.Text)
}

// spoken is the summary as played outside the blind rating: an external engine's summary
// names its provider first (ADR 003, Decision 2; PDR 004, Decision 1).
func spoken(r summarize.Result) string {
	if r.Row.Local {
		return r.Summary.Text
	}
	return "From " + r.Row.Provider + ". " + r.Summary.Text
}

// errStop is a q at a rating prompt: rating ends and the ratings so far are kept.
var errStop = errors.New("stop")

// rate runs the blind listenability rating: per item, the summaries in shuffled order under
// letters, each rated 1 to 5 before the engine names are shown. It rates the first items
// items (all when 0) and stops early at a q.
func rate(ctx context.Context, results []summarize.Result, stdin io.Reader, stdout io.Writer, doSpeak bool, items int) error {
	in := bufio.NewScanner(stdin)
	var key []string
	defer func() {
		fmt.Fprintln(stdout, "Engines:")
		for _, k := range key {
			fmt.Fprintln(stdout, k)
		}
	}()
	for start, rated := 0, 0; start < len(results) && (items == 0 || rated < items); rated++ {
		end := start
		for end < len(results) && results[end].Item == results[start].Item {
			end++
		}
		var round []int
		for i := start; i < end; i++ {
			if results[i].Err == nil {
				round = append(round, i)
			}
		}
		shuffle(len(round), func(i, j int) { round[i], round[j] = round[j], round[i] })
		fmt.Fprintf(stdout, "Item %s\n", results[start].Item)
		for n, i := range round {
			label := string(rune('A' + n))
			fmt.Fprintf(stdout, "%s: %s\n", label, results[i].Summary.Text)
			if doSpeak {
				if err := speak(ctx, results[i].Summary.Text); err != nil {
					fmt.Fprintln(stdout, "say:", err)
				}
			}
			r, err := ask(in, stdout, label)
			if errors.Is(err, errStop) {
				return nil
			}
			if err != nil {
				return err
			}
			results[i].Rating = r
			key = append(key, fmt.Sprintf("%s %s = %s", results[start].Item, label, results[i].Row.Name))
		}
		start = end
	}
	return nil
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
