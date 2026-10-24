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

const corpusUsage = `Usage: koinon-voice corpus (record --set clean|noisy | status) [flags]

record prompts each phrase of the phrase list and records it through the Apple helper as
SET/NN.wav (16 kHz mono) under the corpus directory, with its SHA-256 in SET/manifest.tsv.
Phrases already in the manifest are skipped, so a stopped session resumes. status lists what
each set has.

`

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
	if len(args) == 0 || (args[0] != "record" && args[0] != "status") {
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
	channel := fs.Int("channel", 1, "with record, the input device's channel that carries the microphone")
	if err := fs.Parse(args[1:]); err != nil {
		return 2
	}
	if fs.NArg() != 0 || (args[0] == "record") != (*set != "") {
		fs.Usage()
		return 2
	}
	list, err := loadPhrases(*phrases)
	if err != nil {
		fmt.Fprintln(stderr, "koinon-voice corpus:", err)
		return 1
	}
	c := speech.Corpus{Dir: *dir}
	if args[0] == "status" {
		return corpusStatus(c, list, stdout, stderr)
	}

	if *helper == "" {
		config, err := summarize.LoadConfig(*engines)
		if err != nil {
			fmt.Fprintln(stderr, "koinon-voice corpus:", err)
			return 1
		}
		*helper = config.Helper
	}
	if *helper == "" {
		fmt.Fprintln(stderr, "koinon-voice corpus: no Apple helper: set helper in the engines file or pass --helper")
		return 1
	}
	again := map[int]bool{}
	for _, f := range strings.Split(*only, ",") {
		if f = strings.TrimSpace(f); f == "" {
			continue
		}
		n, err := strconv.Atoi(f)
		if err != nil {
			fmt.Fprintf(stderr, "koinon-voice corpus: --only: %q is not a phrase number\n", f)
			return 2
		}
		again[n] = true
	}
	if *channel < 1 {
		fmt.Fprintln(stderr, "koinon-voice corpus: --channel counts from 1")
		return 2
	}
	if err := recordSet(c, *set, list, again, *helper, *channel, bufio.NewScanner(stdin), stdout, stderr); err != nil {
		fmt.Fprintln(stderr, "koinon-voice corpus:", err)
		return 1
	}
	return 0
}

func corpusStatus(c speech.Corpus, list []speech.Phrase, stdout, stderr io.Writer) int {
	code := 0
	for _, set := range speech.Sets {
		m, err := c.Manifest(set)
		if err != nil {
			fmt.Fprintln(stderr, "koinon-voice corpus:", err)
			return 1
		}
		var missing []string
		for _, p := range list {
			if _, ok := m[p.N]; !ok {
				missing = append(missing, strconv.Itoa(p.N))
			}
		}
		bad := 0
		for _, e := range m {
			if _, err := c.Verify(set, e); err != nil {
				fmt.Fprintf(stdout, "%s: %v\n", set, err)
				bad++
				code = 1
			}
		}
		line := fmt.Sprintf("%s: %d of %d recorded", set, len(m), len(list))
		if bad > 0 {
			line += fmt.Sprintf(", %d unusable", bad)
		}
		if len(missing) > 0 && len(missing) < len(list) {
			line += "; missing " + strings.Join(missing, ", ")
		}
		fmt.Fprintln(stdout, line)
	}
	return code
}

var errQuit = errors.New("quit")

// recordSet prompts each phrase not yet in the set's manifest (or named in again), records
// it and keeps it in the manifest once the maintainer accepts it.
func recordSet(c speech.Corpus, set string, list []speech.Phrase, again map[int]bool, helper string, channel int, in *bufio.Scanner, stdout, stderr io.Writer) error {
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
		fmt.Fprintf(stdout, "The %s set has every phrase; --only N records one again.\n", set)
		return nil
	}
	fmt.Fprintf(stdout, "Recording the %s set: %d phrases. Read each one at a normal pace after pressing Enter;\nletters in names are spelled out (\"codex koinon voice A F\").\n", set, len(todo))
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
				fmt.Fprintf(stdout, "Stopped; %d of %d phrases are in the %s set.\n", len(manifest), len(list), set)
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
			if err := recordPhrase(context.Background(), helper, speech.RecordSeconds(p), tmp, channel, stderr); err != nil {
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
	fmt.Fprintf(stdout, "\nDone: %d of %d phrases are in the %s set.\n", len(manifest), len(list), set)
	return nil
}
