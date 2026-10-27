// Command koinon-voice is a voice interface to Koinon.
package main

import (
	"fmt"
	"io"
	"os"
)

// version is set at build time with -ldflags "-X main.version=...".
var version = "dev"

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	switch {
	case len(args) == 1 && args[0] == "version":
		fmt.Fprintf(stdout, "koinon-voice %s\n", version)
		return 0
	case len(args) > 0 && args[0] == "summarize":
		return runSummarize(args[1:], stdin, stdout, stderr)
	case len(args) > 0 && args[0] == "helper":
		return runHelper(args[1:], stdout, stderr)
	case len(args) > 0 && args[0] == "corpus":
		return runCorpus(args[1:], stdin, stdout, stderr)
	case len(args) > 0 && args[0] == "bench":
		return runBench(args[1:], stdin, stdout, stderr)
	case len(args) > 0 && args[0] == "board":
		return runBoard(args[1:], stdout, stderr)
	case len(args) > 0 && args[0] == "serve":
		return runServe(args[1:], stdout, stderr)
	}
	fmt.Fprintln(stderr, "Usage: koinon-voice version | summarize | helper | corpus | bench | board | serve")
	return 2
}
