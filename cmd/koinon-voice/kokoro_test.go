package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rwcii/koinon-voice/internal/kokoro"
)

func TestKokoroInstallAndStatus(t *testing.T) {
	sum := sha256.Sum256([]byte("weights"))
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/kokoro-v1_0.safetensors" {
			http.NotFound(w, r)
			return
		}
		w.Write([]byte("weights"))
	}))
	defer srv.Close()
	defer func(b string, c *http.Client, f []kokoro.File) { kokoroBase, kokoroClient, kokoroFiles = b, c, f }(kokoroBase, kokoroClient, kokoroFiles)
	kokoroBase, kokoroClient = srv.URL+"/", srv.Client()
	kokoroFiles = []kokoro.File{{Path: "kokoro-v1_0.safetensors", Size: 7, SHA256: hex.EncodeToString(sum[:])}}
	dir := filepath.Join(t.TempDir(), "kokoro")

	var stdout, stderr bytes.Buffer
	if code := run([]string{"kokoro", "status", "--dir", dir}, nil, &stdout, &stderr); code != 1 || !strings.Contains(stdout.String(), "missing  kokoro-v1_0.safetensors") || !strings.Contains(stdout.String(), "run koinon-voice kokoro install") {
		t.Fatalf("status before install: exit %d stdout %q", code, stdout.String())
	}
	stdout.Reset()
	if code := run([]string{"kokoro", "install", "--dir", dir}, nil, &stdout, &stderr); code != 0 || !strings.Contains(stdout.String(), "all present") {
		t.Fatalf("install: exit %d stdout %q stderr %q", code, stdout.String(), stderr.String())
	}
	stdout.Reset()
	if code := run([]string{"kokoro", "status", "--dir", dir}, nil, &stdout, &stderr); code != 0 || !strings.Contains(stdout.String(), "present  kokoro-v1_0.safetensors") {
		t.Fatalf("status after install: exit %d stdout %q", code, stdout.String())
	}
}

func TestKokoroBadOptions(t *testing.T) {
	for _, args := range [][]string{{"kokoro"}, {"kokoro", "remove"}, {"kokoro", "status", "extra"}, {"kokoro", "install", "--nope"}} {
		var stdout, stderr bytes.Buffer
		if code := run(args, nil, &stdout, &stderr); code != 1 || !isCommandError(stderr.String()) {
			t.Errorf("%q: exit %d stderr %q", args, code, stderr.String())
		}
	}
}
