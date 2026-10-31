package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"

	"github.com/rwcii/koinon-voice/internal/kokoro"
)

// What kokoro install downloads, and from where; tests replace them with a loopback server.
var (
	kokoroBase   = kokoro.BaseURL
	kokoroClient = http.DefaultClient
	kokoroFiles  = kokoro.Files
)

// runKokoro installs or checks the Kokoro model files that koinon-voice-kokoro reads.
func runKokoro(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || (args[0] != "install" && args[0] != "status") {
		return badOptions(stderr)
	}
	fs := flags("kokoro " + args[0])
	dir := fs.String("dir", kokoro.DefaultDir(), "the model directory")
	if fs.Parse(args[1:]) != nil || fs.NArg() != 0 {
		return badOptions(stderr)
	}
	if args[0] == "install" {
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
		defer stop()
		fmt.Fprintf(stdout, "Installing the Kokoro model files of mlx-community/Kokoro-82M-bf16 at %s into %s\n", kokoro.Revision, *dir)
		err := kokoro.Install(ctx, kokoroClient, kokoroBase, *dir, kokoroFiles, func(line string) { fmt.Fprintln(stdout, line) })
		if err != nil {
			fmt.Fprintln(stderr, "koinon-voice kokoro install:", err)
			return 1
		}
	}
	states := kokoro.Status(*dir, kokoroFiles)
	ok := true
	for _, s := range states {
		line := fmt.Sprintf("%-8s %s", s.State, s.File.Path)
		if s.Reason != "" {
			line += ": " + s.Reason
		}
		fmt.Fprintln(stdout, line)
		ok = ok && s.State == kokoro.Present
	}
	fmt.Fprintln(stdout, kokoro.Summary(*dir, states))
	if !ok {
		return 1
	}
	return 0
}
