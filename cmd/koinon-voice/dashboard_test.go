package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDashboardRefusesNonLoopback(t *testing.T) {
	var stdout, stderr bytes.Buffer
	for _, addr := range []string{"0.0.0.0:47680", "localhost:47680"} {
		if code := run([]string{"dashboard", "--listen", addr, "--hosts", filepath.Join(os.TempDir(), "none.toml")}, nil, &stdout, &stderr); code != 1 || !strings.Contains(stderr.String(), "loopback") {
			t.Errorf("%s: exit %d %q", addr, code, stderr.String())
		}
	}
}
