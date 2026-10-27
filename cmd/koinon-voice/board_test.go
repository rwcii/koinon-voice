package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rwcii/koinon-voice/internal/koinon"
	"github.com/rwcii/koinon-voice/internal/koinon/koinontest"
)

const boardSecret = "synthetic-board-secret-0c7e"

type boardSecrets map[string]string

func (s boardSecrets) Get(ctx context.Context, name string) (string, error) {
	if v, ok := s[name]; ok {
		return v, nil
	}
	return "", fmt.Errorf("Keychain item koinon-voice-%s not found (security exit 44)", name)
}

// boardHosts writes a hosts file with a fake host "local" holding items, and a fake host
// "other" whose secret is wrong.
func boardHosts(t *testing.T) string {
	t.Helper()
	hostSecrets = boardSecrets{"maintainer-local": boardSecret, "maintainer-other": "wrong-secret-6d"}
	connectorOptions = koinon.Options{MinDelay: 20 * time.Millisecond, MaxDelay: time.Hour, Grace: 300 * time.Millisecond}
	t.Cleanup(func() {
		hostSecrets = boardSecrets{}
		connectorOptions = koinon.Options{}
	})
	dir := t.TempDir()
	var rows strings.Builder
	for _, label := range []string{"local", "other"} {
		d := filepath.Join(dir, label)
		if err := os.Mkdir(d, 0o700); err != nil {
			t.Fatal(err)
		}
		c := koinontest.NewControl(d, boardSecret)
		c.SetBoard(koinontest.Board{Items: []map[string]any{
			koinontest.Item("p-1", "decision", 2, "Approve the plan", "ctx", ""),
			koinontest.Item("f-1", "fyi", 1, "Clear\x1b[2J the screen", "ctx", ""),
		}})
		path := koinontest.WriteKoinon(filepath.Join(d, "koinon"), c)
		fmt.Fprintf(&rows, "[[host]]\nlabel = %q\nkoinon = %q\nsecret = \"maintainer-%s\"\n\n", label, path, label)
	}
	p := filepath.Join(dir, "hosts.toml")
	if err := os.WriteFile(p, []byte(rows.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

// boardOutput is board's JSON, as a test reads it.
type boardOutput struct {
	Hosts []struct {
		Host, Status, Code, Reason string
		Counts                     koinon.Counts
		Items                      []koinon.Item
		Inbox                      []koinon.Message
		Rooms                      []json.RawMessage
	}
	OK bool
}

func TestBoard(t *testing.T) {
	hosts := boardHosts(t)
	var stdout, stderr bytes.Buffer
	code := run([]string{"board", "--hosts", hosts, "--timeout", "20s"}, nil, &stdout, &stderr)
	out := stdout.String()
	if code != 1 {
		t.Errorf("exit %d with a host disconnected", code)
	}
	if strings.Count(out, "\n") != 1 {
		t.Errorf("not one line of JSON:\n%s", out)
	}
	var got boardOutput
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("%v:\n%s", err, out)
	}
	if got.OK || len(got.Hosts) != 2 {
		t.Fatalf("output:\n%s", out)
	}
	local, other := got.Hosts[0], got.Hosts[1]
	if local.Host != "local" || local.Status != "connected" || local.Counts.Waiting != 1 || local.Counts.OpenFYI != 1 ||
		len(local.Items) != 2 || local.Items[0].Reference != "p-1" || local.Items[0].Revision != 2 ||
		local.Items[1].Title != "Clear\x1b[2J the screen" || local.Inbox == nil || local.Rooms == nil {
		t.Errorf("local: %+v", local)
	}
	if other.Host != "other" || other.Status != "disconnected" || other.Code != "maintainer_secret_invalid" {
		t.Errorf("other: %+v", other)
	}
	if strings.Contains(out, "\x1b") || strings.Contains(out+stderr.String(), boardSecret) || strings.Contains(out, "wrong-secret-6d") {
		t.Errorf("output holds an escape or a secret:\n%q", out)
	}
}

func TestBoardWatch(t *testing.T) {
	hosts := boardHosts(t)
	watchContext = func() (context.Context, context.CancelFunc) {
		return context.WithTimeout(context.Background(), 1500*time.Millisecond)
	}
	t.Cleanup(func() {
		watchContext = func() (context.Context, context.CancelFunc) { return context.WithCancel(context.Background()) }
	})
	var stdout, stderr bytes.Buffer
	if code := run([]string{"board", "--hosts", hosts, "--watch"}, nil, &stdout, &stderr); code != 0 {
		t.Errorf("exit %d: %s", code, stderr.String())
	}
	// One JSON object per line, the last with local connected.
	lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	var last boardOutput
	for _, l := range lines {
		if err := json.Unmarshal([]byte(l), &last); err != nil {
			t.Fatalf("%v: %q", err, l)
		}
	}
	if last.Hosts[0].Status != "connected" {
		t.Errorf("watch output:\n%s", stdout.String())
	}
}

func TestBoardHostsFile(t *testing.T) {
	var stdout, stderr bytes.Buffer
	missing := filepath.Join(t.TempDir(), "hosts.toml")
	if code := run([]string{"board", "--hosts", missing}, nil, &stdout, &stderr); code != 1 || !strings.Contains(stderr.String(), missing) {
		t.Errorf("exit %d stderr %q", code, stderr.String())
	}
	if code := run([]string{"board", "extra"}, nil, &stdout, &stderr); code != 1 || !isCommandError(stderr.String()) {
		t.Errorf("extra argument: exit %d", code)
	}
}
