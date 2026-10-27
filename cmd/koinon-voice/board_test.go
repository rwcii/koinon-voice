package main

import (
	"bytes"
	"context"
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

func TestBoard(t *testing.T) {
	hosts := boardHosts(t)
	var stdout, stderr bytes.Buffer
	code := run([]string{"board", "--hosts", hosts, "--timeout", "20s"}, nil, &stdout, &stderr)
	out := stdout.String()
	if code != 1 {
		t.Errorf("exit %d with a host disconnected", code)
	}
	for _, want := range []string{
		"local: connected, 1 waiting, 1 open fyi, 0 in progress, 0 inbox messages, 0 rooms",
		"p-1  decision  r2  Approve the plan",
		"Clear?[2J the screen",
		"other: disconnected: maintainer_secret_invalid",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
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
	if !strings.Contains(stdout.String(), "local: connected") {
		t.Errorf("watch output:\n%s", stdout.String())
	}
}

func TestBoardHostsFile(t *testing.T) {
	var stdout, stderr bytes.Buffer
	missing := filepath.Join(t.TempDir(), "hosts.toml")
	if code := run([]string{"board", "--hosts", missing}, nil, &stdout, &stderr); code != 1 || !strings.Contains(stderr.String(), missing) {
		t.Errorf("exit %d stderr %q", code, stderr.String())
	}
	if code := run([]string{"board", "extra"}, nil, &stdout, &stderr); code != 2 {
		t.Errorf("extra argument: exit %d", code)
	}
}
