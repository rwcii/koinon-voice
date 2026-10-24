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
	"strconv"
	"strings"

	"github.com/rwcii/koinon-voice/internal/speech"
	"github.com/rwcii/koinon-voice/internal/summarize"
)

const corpusUsage = `Usage: koinon-voice corpus (record --set clean|noisy | wake --distance near|mid|far |
       background --source music|meeting|podcast|coffee | status) [flags]

record prompts each phrase of the phrase list and records it through the Apple helper as
SET/NN.wav (16 kHz mono) under the corpus directory, with its SHA-256 in SET/manifest.tsv.
wake records short takes of "Koinon", said once each, at one distance; background records
long stretches of a background source with no "Koinon" in it. Takes already in a manifest
are skipped, so a stopped session resumes. status lists what each set has.

`

// wakeSeconds is one wake take: room to say "Koinon" once after pressing Enter.
const wakeSeconds = 3

// phrasesFile is the committed phrase list, relative to the repository root.
const phrasesFile = "docs/sprints/2026-10-23-phase0-spike/corpus-phrases.md"

// recordPhrase records one file; tests replace it so no test opens a microphone (WDR 006).
var recordPhrase = func(ctx context.Context, helper string, seconds float64, path string, channel int, stderr io.Writer) error {
	cmd := exec.CommandContext(ctx, helper, "record", strconv.FormatFloat(seconds, 'f', 1, 64), path, strconv.Itoa(channel))
	cmd.Stdout, cmd.Stderr = stderr, stderr
	return cmd.Run()
}

func defaultCorpus() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "corpus"
	}
	return filepath.Join(home, ".local", "share", "koinon-voice", "corpus")
}

func loadPhrases(path string) ([]speech.Phrase, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("phrase list: %w (run from the repository root or pass --phrases)", err)
	}
	defer f.Close()
	return speech.ParsePhrases(f)
}

func runCorpus(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 || (args[0] != "record" && args[0] != "status" && args[0] != "wake" && args[0] != "background") {
		fmt.Fprint(stderr, corpusUsage)
		return 2
	}
	fs := flag.NewFlagSet("corpus "+args[0], flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprint(stderr, corpusUsage)
		fs.PrintDefaults()
	}
	set := fs.String("set", "", "with record, the set to record: clean or noisy")
	phrases := fs.String("phrases", phrasesFile, "phrase list")
	dir := fs.String("dir", defaultCorpus(), "corpus directory")
	engines := fs.String("engines", defaultEngines(), "engines file, for its helper path")
	helper := fs.String("helper", "", "the Apple helper's path (default: helper in the engines file)")
	only := fs.String("only", "", "with record, comma-separated phrase numbers to record again")
	channel := fs.Int("channel", 1, "with record, wake and background, the input device's channel that carries the microphone")
	distance := fs.String("distance", "", "with wake: near (about 0.5 m), mid (about 1.5 m) or far (about 3 m)")
	source := fs.String("source", "", "with background: music, meeting, podcast or coffee")
	count := fs.Int("count", 0, "with wake, the takes per distance (default 20); with background, the recordings per source (default 3)")
	minutes := fs.Float64("minutes", 20, "with background, the length of each recording")
	if err := fs.Parse(args[1:]); err != nil {
		return 2
	}
	switch {
	case fs.NArg() != 0,
		(args[0] == "record") != (*set != ""),
		(args[0] == "wake") != (*distance != ""),
		(args[0] == "background") != (*source != ""),
		*minutes <= 0:
		fs.Usage()
		return 2
	}
	how := phraseTakes
	switch args[0] {
	case "wake":
		*set = "wake/" + *distance
		n := *count
		if n == 0 {
			n = 20
		}
		var takeList []speech.Phrase
		for i := 1; i <= n; i++ {
			takeList = append(takeList, speech.Phrase{N: i, Text: "Say \"Koinon\" once, as if waking the bridge."})
		}
		how = takes{
			intro:   "Stand at the distance and say \"Koinon\" once in each take, the way you would to wake the bridge;\nvary the tone a little between takes.",
			seconds: func(speech.Phrase) float64 { return wakeSeconds },
			noun:    "takes",
		}
		return corpusTakes(corpusAt(*dir), *set, takeList, how, *engines, *helper, *only, *channel, stdin, stdout, stderr)
	case "background":
		*set = "background/" + *source
		n := *count
		if n == 0 {
			n = 3
		}
		var takeList []speech.Phrase
		for i := 1; i <= n; i++ {
			takeList = append(takeList, speech.Phrase{N: i, Text: fmt.Sprintf("%.0f minutes of %s; nobody says \"Koinon\".", *minutes, *source)})
		}
		secs := *minutes * 60
		how = takes{
			intro:   "Start the source playing at its normal volume, then press Enter; keep quiet and do not say \"Koinon\"\nuntil the recording ends.",
			seconds: func(speech.Phrase) float64 { return secs },
			noun:    "recordings",
		}
		return corpusTakes(corpusAt(*dir), *set, takeList, how, *engines, *helper, *only, *channel, stdin, stdout, stderr)
	}
	list, err := loadPhrases(*phrases)
	if err != nil {
		fmt.Fprintln(stderr, "koinon-voice corpus:", err)
		return 1
	}
	if args[0] == "status" {
		return corpusStatus(corpusAt(*dir), list, stdout, stderr)
	}
	return corpusTakes(corpusAt(*dir), *set, list, how, *engines, *helper, *only, *channel, stdin, stdout, stderr)
}

func corpusAt(dir string) speech.Corpus { return speech.Corpus{Dir: dir} }

// corpusTakes finds the helper and records the set's takes.
func corpusTakes(c speech.Corpus, set string, list []speech.Phrase, how takes, engines, helper, only string, channel int, stdin io.Reader, stdout, stderr io.Writer) int {
	if helper == "" {
		config, err := summarize.LoadConfig(engines)
		if err != nil {
			fmt.Fprintln(stderr, "koinon-voice corpus:", err)
			return 1
		}
		helper = config.Helper
	}
	if helper == "" {
		fmt.Fprintln(stderr, "koinon-voice corpus: no Apple helper: set helper in the engines file or pass --helper")
		return 1
	}
	again := map[int]bool{}
	for _, f := range strings.Split(only, ",") {
		if f = strings.TrimSpace(f); f == "" {
			continue
		}
		n, err := strconv.Atoi(f)
		if err != nil {
			fmt.Fprintf(stderr, "koinon-voice corpus: --only: %q is not a take number\n", f)
			return 2
		}
		again[n] = true
	}
	if channel < 1 {
		fmt.Fprintln(stderr, "koinon-voice corpus: --channel counts from 1")
		return 2
	}
	if err := recordTakes(c, set, list, again, helper, channel, how, bufio.NewScanner(stdin), stdout, stderr); err != nil {
		fmt.Fprintln(stderr, "koinon-voice corpus:", err)
		return 1
	}
	return 0
}

func corpusStatus(c speech.Corpus, list []speech.Phrase, stdout, stderr io.Writer) int {
	code := 0
	for _, set := range append(append(append([]string(nil), speech.Sets...), speech.WakeSets...), speech.BackgroundSets...) {
		m, err := c.Manifest(set)
		if err != nil {
			fmt.Fprintln(stderr, "koinon-voice corpus:", err)
			return 1
		}
		bad := 0
		seconds := 0.0
		for _, e := range m {
			seconds += e.Seconds
			if _, err := c.Verify(set, e); err != nil {
				fmt.Fprintf(stdout, "%s: %v\n", set, err)
				bad++
				code = 1
			}
		}
		var line string
		switch {
		case strings.HasPrefix(set, "wake/"):
			line = fmt.Sprintf("%s: %d takes", set, len(m))
		case strings.HasPrefix(set, "background/"):
			line = fmt.Sprintf("%s: %d recordings, %.0f minutes", set, len(m), seconds/60)
		default:
			var missing []string
			for _, p := range list {
				if _, ok := m[p.N]; !ok {
					missing = append(missing, strconv.Itoa(p.N))
				}
			}
			line = fmt.Sprintf("%s: %d of %d recorded", set, len(m), len(list))
			if len(missing) > 0 && len(missing) < len(list) {
				line += "; missing " + strings.Join(missing, ", ")
			}
		}
		if bad > 0 {
			line += fmt.Sprintf(", %d unusable", bad)
		}
		fmt.Fprintln(stdout, line)
	}
	return code
}

var errQuit = errors.New("quit")

// takes says how a set is recorded: what is said before it starts, how long each take is and
// what a take is called.
type takes struct {
	intro   string
	seconds func(speech.Phrase) float64
	noun    string // "phrases", "takes", "recordings"
}

var phraseTakes = takes{
	intro:   "Read each one at a normal pace after pressing Enter;\nletters in names are spelled out (\"codex koinon voice A F\").",
	seconds: speech.RecordSeconds,
	noun:    "phrases",
}

// recordTakes prompts each take not yet in the set's manifest (or named in again), records
// it and keeps it in the manifest once the maintainer accepts it.
func recordTakes(c speech.Corpus, set string, list []speech.Phrase, again map[int]bool, helper string, channel int, how takes, in *bufio.Scanner, stdout, stderr io.Writer) error {
	if _, err := c.Prepare(set); err != nil {
		return err
	}
	manifest, err := c.Manifest(set)
	if err != nil {
		return err
	}
	var todo []speech.Phrase
	for _, p := range list {
		if _, done := manifest[p.N]; (!done && len(again) == 0) || again[p.N] {
			todo = append(todo, p)
		}
	}
	if len(todo) == 0 {
		fmt.Fprintf(stdout, "The %s set has every one of its %s; --only N records one again.\n", set, how.noun)
		return nil
	}
	fmt.Fprintf(stdout, "Recording the %s set: %d %s. %s\n", set, len(todo), how.noun, how.intro)
	read := func(prompt string) (string, error) {
		fmt.Fprint(stdout, prompt)
		if !in.Scan() {
			if err := in.Err(); err != nil {
				return "", err
			}
			return "", errQuit
		}
		return strings.TrimSpace(in.Text()), nil
	}
	for i, p := range todo {
		fmt.Fprintf(stdout, "\n[%d/%d] %02d. %s\n", i+1, len(todo), p.N, p.Text)
		for {
			answer, err := read("Enter to record, s to skip, q to stop: ")
			if errors.Is(err, errQuit) || answer == "q" {
				fmt.Fprintf(stdout, "Stopped; %d of %d %s are in the %s set.\n", len(manifest), len(list), how.noun, set)
				return nil
			}
			if err != nil {
				return err
			}
			if answer == "s" {
				break
			}
			path := c.Path(set, p.N)
			tmp := path + ".new.wav"
			if err := recordPhrase(context.Background(), helper, how.seconds(p), tmp, channel, stderr); err != nil {
				os.Remove(tmp)
				fmt.Fprintf(stdout, "Recording failed: %v. Try again, or q to stop.\n", err)
				continue
			}
			a, err := speech.CheckCorpusFile(tmp)
			if err != nil {
				os.Remove(tmp)
				fmt.Fprintf(stdout, "Not usable: %v. Try again.\n", err)
				continue
			}
			if a.Peak < 0.02 {
				fmt.Fprintf(stdout, "Very quiet (peak %.1f%%): speak closer or louder.\n", a.Peak*100)
			}
			keep, keepErr := read("Enter to keep it, r to record it again: ")
			if keepErr != nil && !errors.Is(keepErr, errQuit) {
				return keepErr
			}
			if keep == "r" {
				os.Remove(tmp)
				continue
			}
			if err := os.Rename(tmp, path); err != nil {
				return err
			}
			sum, err := speech.Hash(path)
			if err != nil {
				return err
			}
			manifest[p.N] = speech.Entry{N: p.N, SHA256: sum, Seconds: a.Seconds()}
			if err := c.WriteManifest(set, manifest); err != nil {
				return err
			}
			if errors.Is(keepErr, errQuit) {
				return nil // input ended after the recording, which is kept
			}
			break
		}
	}
	fmt.Fprintf(stdout, "\nDone: %d of %d %s are in the %s set.\n", len(manifest), len(list), how.noun, set)
	return nil
}
