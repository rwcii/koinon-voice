package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
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

// runBoard is koinon-voice board. Connect to each Koinon daemon in the hosts file through its
// maintainer mode and print each host's state and the items that wait for the maintainer. The
// exit status is 1 when a host is not connected.
func runBoard(args []string, stdout, stderr io.Writer) int {
	fs := flags("board")
	hostsFile := fs.String("hosts", defaultHosts(), "hosts file")
	watch := fs.Bool("watch", false, "print again on each change until interrupted")
	timeout := fs.Duration("timeout", 30*time.Second, "without --watch, how long to wait for every host")
	if err := fs.Parse(args); err != nil {
		return badOptions(stderr)
	}
	if fs.NArg() != 0 {
		return badOptions(stderr)
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
	if !printBoard(stdout, conns) {
		return 1
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

// boardHost is one host as board prints it.
type boardHost struct {
	Host     string           `json:"host"`
	Status   koinon.Status    `json:"status"`
	Code     string           `json:"code,omitempty"`
	Reason   string           `json:"reason,omitempty"`
	Since    int64            `json:"since"`
	Attempts int              `json:"attempts"`
	Counts   koinon.Counts    `json:"counts"`
	Items    []koinon.Item    `json:"items"`
	Inbox    []koinon.Message `json:"inbox"`
	Rooms    []boardRoom      `json:"rooms"`
}

type boardRoom struct {
	koinon.Room
	Posts []koinon.Post `json:"posts"`
}

// printBoard prints every host's state as one line of JSON, as koinon prints a result; ok
// is false when a host is not connected. JSON escapes control characters, so text from
// agents cannot move the cursor or change the terminal.
func printBoard(w io.Writer, conns []*koinon.Connector) bool {
	out := struct {
		Hosts []boardHost `json:"hosts"`
		OK    bool        `json:"ok"`
	}{Hosts: []boardHost{}, OK: true}
	for _, c := range conns {
		s := c.State()
		h := boardHost{Host: s.Host, Status: s.Status, Code: s.Code, Reason: s.Reason,
			Since: s.Since.UnixMilli(), Attempts: s.Attempts, Counts: s.Counts,
			Items: nonNil(s.Items), Inbox: nonNil(s.Inbox), Rooms: []boardRoom{}}
		for _, r := range s.Rooms {
			h.Rooms = append(h.Rooms, boardRoom{r.Room, nonNil(r.Posts)})
		}
		if s.Status != koinon.Connected {
			out.OK = false
		}
		out.Hosts = append(out.Hosts, h)
	}
	printJSON(w, out, "")
	return out.OK
}

// nonNil makes an empty list print as [] rather than null.
func nonNil[T any](v []T) []T {
	if v == nil {
		return []T{}
	}
	return v
}

func defaultHosts() string {
	// ~/.config on every system, beside engines.toml.
	home, err := os.UserHomeDir()
	if err != nil {
		return "hosts.toml"
	}
	return filepath.Join(home, ".config", "koinon-voice", "hosts.toml")
}
