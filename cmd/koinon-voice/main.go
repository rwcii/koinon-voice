// Command koinon-voice is a voice interface to Koinon.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"runtime"
	"runtime/debug"
)

// version is set at build time with -ldflags "-X main.version=...".
var version = "dev"

// usage lists every command with its full synopsis, in the layout of koinon --help.
const usage = `Usage: koinon-voice version
       koinon-voice board [--hosts FILE] [--watch] [--timeout DURATION]
       koinon-voice dashboard [--hosts FILE] [--listen 127.0.0.1:PORT] [--no-open]
       koinon-voice summarize (--fixtures DIR | --item FILE) [--engines FILE] [--engine NAME,...] [--out FILE] [--timeout DURATION] [--speak] [--helper PATH] [--voice ID] [--blind] [--rate N] [--rescore FILE]
       koinon-voice helper [--engines FILE] [--helper PATH] [--transcribe FILE] [--say TEXT] [--out FILE] [--voice ID] [--timeout DURATION]
       koinon-voice corpus record --set clean|noisy [--only N,...] [--channel N] [--phrases FILE] [--dir DIR] [--engines FILE] [--helper PATH]
       koinon-voice corpus wake --distance near|mid|far [--train] [--count N] [--channel N] [--phrases FILE] [--dir DIR] [--engines FILE] [--helper PATH]
       koinon-voice corpus background --source music|meeting|podcast|coffee [--count N] [--minutes N] [--channel N] [--phrases FILE] [--dir DIR] [--engines FILE] [--helper PATH]
       koinon-voice corpus status [--phrases FILE] [--dir DIR] [--engines FILE] [--helper PATH]
       koinon-voice bench stt [--phrases FILE] [--dir DIR] [--sets SET,...] [--out FILE]
       koinon-voice bench tts [--texts FILE] [--dir DIR] [--kokoro-voice VOICE] [--rate=false] [--out FILE]
       koinon-voice bench wake [--phrases FILE] [--dir DIR] [--model FILE] [--threshold N] [--out FILE]
       koinon-voice bench interrupt [--texts FILE] [--text N] [--say TEXT] [--control] [--seconds N] [--dir DIR] [--out FILE]
       koinon-voice bench names [--phrases FILE] [--scorecard FILE] [--ordinary FILE,...] [--out FILE]
Each bench command also takes [--engines FILE] [--helper PATH] [--python PATH] [--bench-helper FILE] [--engine NAME,...] [--timeout DURATION] [--warmup DURATION].
`

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] == "--help" || args[0] == "help" {
		fmt.Fprint(stdout, usage)
		return 0
	}
	switch args[0] {
	case "version":
		return runVersion(args[1:], stdout, stderr)
	case "summarize":
		return runSummarize(args[1:], stdin, stdout, stderr)
	case "helper":
		return runHelper(args[1:], stdout, stderr)
	case "corpus":
		return runCorpus(args[1:], stdin, stdout, stderr)
	case "bench":
		return runBench(args[1:], stdin, stdout, stderr)
	case "board":
		return runBoard(args[1:], stdout, stderr)
	case "dashboard":
		return runDashboard(args[1:], stdout, stderr)
	}
	return commandError(stderr, "unknown command; use koinon-voice --help")
}

// commandError prints a command-line error as koinon does, one JSON object on standard
// error with Koinon's code for it, and returns the exit status 1.
func commandError(stderr io.Writer, detail string) int {
	b, _ := json.Marshal(struct {
		Code   string `json:"code"`
		Detail string `json:"detail"`
		OK     bool   `json:"ok"`
	}{"daemon_error", detail, false})
	fmt.Fprintf(stderr, "%s\n", b)
	return 1
}

// badOptions is the error for options a command cannot use, including --help after a
// command: the help is koinon-voice --help.
func badOptions(stderr io.Writer) int {
	return commandError(stderr, "invalid command options; use koinon-voice --help")
}

// flags returns a flag set that prints nothing; a command reports a parse error with
// badOptions.
func flags(name string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.Usage = func() {}
	return fs
}

// runVersion prints the build as JSON, with the field names of koinon version.
func runVersion(args []string, stdout, stderr io.Writer) int {
	if len(args) != 0 {
		return badOptions(stderr)
	}
	v := struct {
		Arch    string `json:"arch"`
		Commit  string `json:"commit"`
		Go      string `json:"go"`
		OS      string `json:"os"`
		Version string `json:"version"`
	}{Arch: runtime.GOARCH, Go: runtime.Version(), OS: runtime.GOOS, Version: version}
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, s := range info.Settings {
			if s.Key == "vcs.revision" {
				v.Commit = s.Value
			}
		}
	}
	printJSON(stdout, v, "  ")
	return 0
}

// printJSON prints v as one line of JSON, or indented when indent is not empty.
func printJSON(w io.Writer, v any, indent string) {
	b, _ := json.Marshal(v)
	if indent != "" {
		b, _ = json.MarshalIndent(v, "", indent)
	}
	fmt.Fprintf(w, "%s\n", b)
}
