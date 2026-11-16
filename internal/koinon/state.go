package koinon

import (
	"strings"
	"sync"
	"time"
)

// Status is a host's connection state.
type Status string

const (
	Connecting   Status = "connecting"
	Connected    Status = "connected"
	Disconnected Status = "disconnected"
)

// State is what a signed source knows of one computer. A source replaces its slices and
// never changes them in place, so a State is safe to keep and read.
type State struct {
	Host string
	// Pin is the source's stable key for the computer (R12); Host shows its name, which can
	// change and need not be unique. Node is the hosts.toml label of the row that reaches it.
	Pin    string
	Node   string
	Status Status
	// Code and Reason say why the host is disconnected; Code is Koinon's code when there
	// is one.
	Code   string
	Reason string
	Since  time.Time
	// Attempts counts the starts of the connection.
	Attempts int
	Counts   Counts
	// Items are the board's active items, decision and fyi, in board order, each read in
	// full.
	Items []Item
	// Inbox holds the maintainer's messages that are not acknowledged.
	Inbox []Message
	Rooms []RoomState
	// Pending are the source's verdicts whose outcome is not known yet.
	Pending []PendingWrite
	// Loaded: the source has read this computer's topics in full once, so its items, inbox
	// and rooms are complete.
	Loaded bool
}

// RoomState is a room with every post that Koinon retains.
type RoomState struct {
	Room
	Posts []Post
}

type roomCursor struct {
	room  Room
	after int64
	posts []Post
}

// tail keeps the last bytes a process wrote to standard error.
type tail struct {
	mu  sync.Mutex
	max int
	buf []byte
}

func (t *tail) Write(p []byte) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.buf = append(t.buf, p...)
	if len(t.buf) > t.max {
		t.buf = t.buf[len(t.buf)-t.max:]
	}
	return len(p), nil
}

func (t *tail) lastLine() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	lines := strings.Split(strings.TrimSpace(string(t.buf)), "\n")
	line := strings.TrimSpace(lines[len(lines)-1])
	if len(line) > 300 {
		line = line[:300]
	}
	return line
}
