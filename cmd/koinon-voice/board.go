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
	src := newSources(hosts, func() {
		select {
		case changed <- struct{}{}:
		default:
		}
	})
	var wg sync.WaitGroup
	src.run(ctx, &wg)
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
				printBoard(stdout, src)
			}
		}
	}

	deadline := time.After(*timeout)
	for !settled(src.states()) {
		select {
		case <-changed:
		case <-deadline:
			printBoard(stdout, src)
			return 1
		}
	}
	if !printBoard(stdout, src) {
		return 1
	}
	return 0
}

// settled reports whether every host has connected or failed its first start, and every
// computer of a signed row has been read in full once or shows why it was not.
func settled(states []koinon.State) bool {
	for _, s := range states {
		if s.Status == koinon.Connecting {
			return false
		}
		if s.Pin != "" && s.Status == koinon.Connected && !s.Loaded && s.Code == "" {
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
	// Pin and Node are a signed row's computer and the row's label; Pending its verdicts
	// whose outcome is not known, or learned after the answer was lost.
	Pin     string                `json:"pin,omitempty"`
	Node    string                `json:"node,omitempty"`
	Pending []koinon.PendingWrite `json:"pending,omitempty"`
}

type boardRoom struct {
	koinon.Room
	Posts []koinon.Post `json:"posts"`
}

// printBoard prints every host's state as one line of JSON, as koinon prints a result; ok
// is false when a host is not connected or the pending verdicts file cannot be read. JSON
// escapes control characters, so text from agents cannot move the cursor or change the
// terminal. unresolved lists the pending verdicts through nodes that no row names.
func printBoard(w io.Writer, src sources) bool {
	out := struct {
		Hosts        []boardHost       `json:"hosts"`
		Unresolved   []koinon.Unrouted `json:"unresolved"`
		JournalError string            `json:"journal_error,omitempty"`
		OK           bool              `json:"ok"`
	}{Hosts: []boardHost{}, Unresolved: nonNil(src.unrouted), OK: src.journalErr == nil}
	if src.journalErr != nil {
		out.JournalError = src.journalErr.Error()
	}
	for _, s := range src.states() {
		h := boardHost{Host: s.Host, Status: s.Status, Code: s.Code, Reason: s.Reason,
			Since: s.Since.UnixMilli(), Attempts: s.Attempts, Counts: s.Counts,
			Items: nonNil(s.Items), Inbox: nonNil(s.Inbox), Rooms: []boardRoom{},
			Pin: s.Pin, Node: s.Node, Pending: s.Pending}
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
