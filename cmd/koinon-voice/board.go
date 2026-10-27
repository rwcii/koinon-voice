package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"text/tabwriter"
	"time"

	"github.com/rwcii/koinon-voice/internal/keychain"
	"github.com/rwcii/koinon-voice/internal/koinon"
)

// hostSecrets reads each host's maintainer secret; tests replace it so no test reads the
// Keychain (WDR 006).
var hostSecrets koinon.Secrets = keychain.Keychain{}

// connectorOptions are the connectors' options before the command sets its own; tests
// shorten the waits.
var connectorOptions = koinon.Options{}

// watchContext ends a --watch run; tests replace it.
var watchContext = func() (context.Context, context.CancelFunc) {
	return signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
}

const boardUsage = `Usage: koinon-voice board [flags]

Connect to each Koinon daemon in the hosts file through its maintainer mode and print each
host's state and the items that wait for the maintainer. The exit status is 1 when a host
is not connected.

`

func runBoard(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("board", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprint(stderr, boardUsage)
		fs.PrintDefaults()
	}
	hostsFile := fs.String("hosts", defaultHosts(), "hosts file")
	watch := fs.Bool("watch", false, "print again on each change until interrupted")
	timeout := fs.Duration("timeout", 30*time.Second, "without --watch, how long to wait for every host")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 0 {
		fs.Usage()
		return 2
	}
	hosts, err := koinon.LoadHosts(*hostsFile)
	if err != nil {
		fmt.Fprintln(stderr, "koinon-voice board:", err)
		return 1
	}

	var ctx context.Context
	var cancel context.CancelFunc
	if *watch {
		ctx, cancel = watchContext()
	} else {
		ctx, cancel = context.WithCancel(context.Background())
	}
	changed := make(chan struct{}, 1)
	opts := connectorOptions
	opts.Secrets, opts.Version = hostSecrets, version
	opts.OnChange = func(koinon.State) {
		select {
		case changed <- struct{}{}:
		default:
		}
	}
	conns := make([]*koinon.Connector, len(hosts))
	var wg sync.WaitGroup
	for i, h := range hosts {
		conns[i] = koinon.New(h, opts)
		wg.Add(1)
		go func(c *koinon.Connector) {
			defer wg.Done()
			c.Run(ctx)
		}(conns[i])
	}
	defer wg.Wait()
	defer cancel()

	if *watch {
		for {
			select {
			case <-ctx.Done():
				return 0
			case <-changed:
				// Let a burst of changes settle into one print.
				time.Sleep(100 * time.Millisecond)
				printBoard(stdout, conns)
				fmt.Fprintln(stdout)
			}
		}
	}

	deadline := time.After(*timeout)
	for !settled(conns) {
		select {
		case <-changed:
		case <-deadline:
			printBoard(stdout, conns)
			return 1
		}
	}
	printBoard(stdout, conns)
	for _, c := range conns {
		if c.State().Status != koinon.Connected {
			return 1
		}
	}
	return 0
}

// settled reports whether every host has connected or failed its first start.
func settled(conns []*koinon.Connector) bool {
	for _, c := range conns {
		if s := c.State(); s.Status == koinon.Connecting {
			return false
		}
	}
	return true
}

func printBoard(w io.Writer, conns []*koinon.Connector) {
	for _, c := range conns {
		s := c.State()
		label := printable(s.Host)
		if s.Status != koinon.Connected {
			if s.Code != "" && !strings.HasPrefix(s.Reason, s.Code) {
				fmt.Fprintf(w, "%s: %s: %s: %s\n", label, s.Status, printable(s.Code), printable(s.Reason))
			} else {
				fmt.Fprintf(w, "%s: %s: %s\n", label, s.Status, printable(s.Reason))
			}
			continue
		}
		fmt.Fprintf(w, "%s: connected, %d waiting, %d open fyi, %d in progress, %d inbox messages, %d rooms\n",
			label, s.Counts.Waiting, s.Counts.OpenFYI, s.Counts.InProgress, len(s.Inbox), len(s.Rooms))
		tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
		for _, it := range s.Items {
			fmt.Fprintf(tw, "  %s\t%s\tr%d\t%s\t(%s)\n", printable(it.Reference), printable(it.Kind), it.Revision, printable(it.Title), printable(it.AuthorName))
		}
		tw.Flush()
	}
}

// printable replaces control characters, so text from agents cannot move the cursor or
// change the terminal.
func printable(s string) string {
	return strings.Map(func(r rune) rune {
		if r < 0x20 || (r >= 0x7f && r < 0xa0) {
			return '?'
		}
		return r
	}, s)
}

func defaultHosts() string {
	// ~/.config on every system, beside engines.toml.
	home, err := os.UserHomeDir()
	if err != nil {
		return "hosts.toml"
	}
	return filepath.Join(home, ".config", "koinon-voice", "hosts.toml")
}
