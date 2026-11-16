package main

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestBoardWatch(t *testing.T) {
	hosts, _ := signedHosts(t, true)
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
	// One JSON object per line, the last with both computers connected.
	lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	var last signedBoard
	for _, l := range lines {
		if err := json.Unmarshal([]byte(l), &last); err != nil {
			t.Fatalf("%v: %q", err, l)
		}
	}
	if len(last.Hosts) != 2 || last.Hosts[0].Status != "connected" || last.Hosts[1].Status != "connected" {
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
	// A row of the removed secret connection is refused before anything starts.
	old := signedOnly(t, "[[host]]\nlabel = \"local\"\nsecret = \"maintainer-local\"\n")
	stderr.Reset()
	if code := run([]string{"board", "--hosts", old}, nil, &stdout, &stderr); code != 1 ||
		!strings.Contains(stderr.String(), "ssh and secret rows were removed") {
		t.Errorf("secret row: exit %d stderr %q", code, stderr.String())
	}
}
