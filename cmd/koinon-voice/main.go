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
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 1 && args[0] == "version" {
		fmt.Fprintf(stdout, "koinon-voice %s\n", version)
		return 0
	}
	fmt.Fprintln(stderr, "Usage: koinon-voice version")
	return 2
}
