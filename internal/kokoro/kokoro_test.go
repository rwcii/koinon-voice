package kokoro

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func file(path, body string) File {
	sum := sha256.Sum256([]byte(body))
	return File{Path: path, Size: int64(len(body)), SHA256: hex.EncodeToString(sum[:])}
}

// server serves bodies by path over loopback HTTPS. A body "CUT" sends a Content-Length of
// 100 and closes the connection after 3 bytes.
func server(t *testing.T, bodies map[string]string) *httptest.Server {
	t.Helper()
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, ok := bodies[strings.TrimPrefix(r.URL.Path, "/")]
		if !ok {
			http.NotFound(w, r)
			return
		}
		if body == "CUT" {
			w.Header().Set("Content-Length", "100")
			w.Write([]byte("abc"))
			conn, _, _ := w.(http.Hijacker).Hijack()
			conn.Close()
			return
		}
		w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func mode(t *testing.T, path string) os.FileMode {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	return info.Mode().Perm()
}

func entries(t *testing.T, dir string) []string {
	t.Helper()
	var names []string
	filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			rel, _ := filepath.Rel(dir, path)
			names = append(names, rel)
		}
		return nil
	})
	return names
}

func TestInstallKeepsACorrectFile(t *testing.T) {
	files := []File{file("model.safetensors", "weights"), file("voices/af_heart.safetensors", "voice")}
	srv := server(t, map[string]string{"model.safetensors": "weights", "voices/af_heart.safetensors": "voice"})
	dir := filepath.Join(t.TempDir(), "kokoro")
	var lines []string
	if err := Install(context.Background(), srv.Client(), srv.URL+"/", dir, files, func(s string) { lines = append(lines, s) }); err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		path := filepath.Join(dir, filepath.FromSlash(f.Path))
		if m := mode(t, path); m != 0o600 {
			t.Errorf("%s mode %o, want 600", f.Path, m)
		}
	}
	for _, d := range []string{dir, filepath.Join(dir, "voices")} {
		if m := mode(t, d); m != 0o700 {
			t.Errorf("%s mode %o, want 700", d, m)
		}
	}
	if got := strings.Join(entries(t, dir), " "); got != "model.safetensors voices/af_heart.safetensors" {
		t.Errorf("files %q", got)
	}
	// A second install keeps what is there.
	lines = nil
	if err := Install(context.Background(), srv.Client(), srv.URL+"/", dir, files, func(s string) { lines = append(lines, s) }); err != nil {
		t.Fatal(err)
	}
	if len(lines) != 2 || !strings.HasSuffix(lines[0], "already installed") {
		t.Errorf("second install %q", lines)
	}
}

func TestInstallRefusesAWrongSHA256(t *testing.T) {
	want := file("model.safetensors", "weights")
	srv := server(t, map[string]string{"model.safetensors": "WEIGHTS"}) // same size, other bytes
	dir := t.TempDir()
	err := Install(context.Background(), srv.Client(), srv.URL+"/", dir, []File{want}, func(string) {})
	if err == nil || !strings.Contains(err.Error(), "SHA-256") || !strings.Contains(err.Error(), "not kept") {
		t.Fatalf("err %v, want a SHA-256 mismatch", err)
	}
	if got := entries(t, dir); len(got) != 0 {
		t.Errorf("left %q", got)
	}
}

func TestInstallRefusesAWrongSize(t *testing.T) {
	srv := server(t, map[string]string{"model.safetensors": "weights and more"})
	dir := t.TempDir()
	err := Install(context.Background(), srv.Client(), srv.URL+"/", dir, []File{file("model.safetensors", "weights")}, func(string) {})
	if err == nil || !strings.Contains(err.Error(), "bytes") {
		t.Fatalf("err %v, want a size mismatch", err)
	}
	if got := entries(t, dir); len(got) != 0 {
		t.Errorf("left %q", got)
	}
}

func TestInstallInterruptedLeavesNoFile(t *testing.T) {
	srv := server(t, map[string]string{"model.safetensors": "CUT"})
	dir := t.TempDir()
	err := Install(context.Background(), srv.Client(), srv.URL+"/", dir, []File{{Path: "model.safetensors", Size: 100, SHA256: strings.Repeat("0", 64)}}, func(string) {})
	if err == nil {
		t.Fatal("an interrupted download installed")
	}
	if got := entries(t, dir); len(got) != 0 {
		t.Errorf("left %q", got)
	}
}

func TestInstallRemovesAnEarlierPart(t *testing.T) {
	srv := server(t, map[string]string{"model.safetensors": "weights"})
	dir := t.TempDir()
	// What a killed install leaves: a temporary file, never the final name.
	os.WriteFile(filepath.Join(dir, ".model.safetensors-123"+partSuffix), []byte("wei"), 0o600)
	if err := Install(context.Background(), srv.Client(), srv.URL+"/", dir, []File{file("model.safetensors", "weights")}, func(string) {}); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(entries(t, dir), " "); got != "model.safetensors" {
		t.Errorf("files %q", got)
	}
}

func TestInstallHTTPError(t *testing.T) {
	srv := server(t, nil)
	err := Install(context.Background(), srv.Client(), srv.URL+"/", t.TempDir(), []File{file("model.safetensors", "weights")}, func(string) {})
	if err == nil || !strings.Contains(err.Error(), "404") {
		t.Fatalf("err %v, want 404", err)
	}
}

func TestStatus(t *testing.T) {
	dir := t.TempDir()
	files := []File{file("a", "one"), file("b", "two"), file("c", "three")}
	os.WriteFile(filepath.Join(dir, "a"), []byte("one"), 0o600)
	os.WriteFile(filepath.Join(dir, "b"), []byte("TWO"), 0o600)
	got := Status(dir, files)
	want := []State{Present, Changed, Missing}
	for i, s := range got {
		if s.State != want[i] {
			t.Errorf("%s: %s, want %s", s.File.Path, s.State, want[i])
		}
	}
	if !strings.Contains(got[1].Reason, "SHA-256") {
		t.Errorf("changed reason %q", got[1].Reason)
	}
	if s := Summary(dir, got); !strings.Contains(s, "b changed, c missing; run koinon-voice kokoro install") {
		t.Errorf("summary %q", s)
	}
	if s := Summary(dir, got[:1]); !strings.HasSuffix(s, "all present") {
		t.Errorf("summary %q", s)
	}
}

// The committed values are the ones Phase 0 rated (definition of done, Model files).
func TestFilesArePhase0s(t *testing.T) {
	if Revision != "a71e4d38b236d968966a2002c4c895dbd12b1c3c" || Files[0].SHA256 != "4e9ecdf03b8b6cf906070390237feda473dc13327cb8d56a43deaa374c02acd8" || Files[0].Size != 327115152 {
		t.Errorf("model file %+v at %s", Files[0], Revision)
	}
}
