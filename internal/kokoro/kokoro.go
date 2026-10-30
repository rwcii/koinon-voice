// Package kokoro installs and checks the Kokoro model files that koinon-voice-kokoro reads:
// the files Phase 0 rated, from one revision of mlx-community/Kokoro-82M-bf16, each checked
// against its SHA-256 (ADR 002, Decisions 2 and 4).
package kokoro

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// Revision is the snapshot of mlx-community/Kokoro-82M-bf16 that Phase 0 rated
// (docs/sprints/2026-10-23-phase0-spike/scorecard-speech.md, Engines).
const Revision = "a71e4d38b236d968966a2002c4c895dbd12b1c3c"

// BaseURL is where the files of Revision are downloaded from.
const BaseURL = "https://huggingface.co/mlx-community/Kokoro-82M-bf16/resolve/" + Revision + "/"

// File is one model file: its path in the model directory and in the repository at
// Revision, its size and its SHA-256.
type File struct {
	Path   string
	Size   int64
	SHA256 string
}

// Files are the files koinon-voice kokoro install writes.
var Files = []File{
	{Path: "kokoro-v1_0.safetensors", Size: 327115152, SHA256: "4e9ecdf03b8b6cf906070390237feda473dc13327cb8d56a43deaa374c02acd8"},
	{Path: "config.json", Size: 2351, SHA256: "5abb01e2403b072bf03d04fde160443e209d7a0dad49a423be15196b9b43c17f"},
	{Path: "voices/af_heart.safetensors", Size: 522320, SHA256: "2c1c733b0e6576c810e268d3e440c21dea4e0f0131a3ba4cfc98d7fe6136d094"},
}

// InstallCommand is the command that installs the files, for error messages.
const InstallCommand = "koinon-voice kokoro install"

// DefaultDir is the model directory koinon-voice-kokoro reads by default.
func DefaultDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".local", "share", "koinon-voice", "kokoro")
	}
	return filepath.Join(home, ".local", "share", "koinon-voice", "kokoro")
}

// partSuffix marks a download in progress; a file under its final name is always checked.
const partSuffix = ".part"

// Install downloads each file that is missing or changed from base into dir, mode 0600 in
// directories of mode 0700. Each download goes to a temporary name and is renamed only after
// its size and SHA-256 match; a file that does not match is removed. report receives one
// line for each file.
func Install(ctx context.Context, client *http.Client, base, dir string, files []File, report func(string)) error {
	for _, f := range files {
		target := filepath.Join(dir, filepath.FromSlash(f.Path))
		if err := mkdirPrivate(dir, filepath.Dir(target)); err != nil {
			return err
		}
		removeParts(filepath.Dir(target), filepath.Base(target))
		if check(target, f) == nil {
			report(f.Path + ": already installed")
			continue
		}
		if err := download(ctx, client, base+f.Path, target, f); err != nil {
			return fmt.Errorf("%s: %w", f.Path, err)
		}
		report(fmt.Sprintf("%s: installed, %d bytes, SHA-256 %s", f.Path, f.Size, f.SHA256))
	}
	return nil
}

// mkdirPrivate makes dir and every directory from it down to sub with mode 0700.
func mkdirPrivate(dir, sub string) error {
	if err := os.MkdirAll(sub, 0o700); err != nil {
		return err
	}
	for d := sub; ; d = filepath.Dir(d) {
		if err := os.Chmod(d, 0o700); err != nil {
			return err
		}
		if d == filepath.Clean(dir) || d == filepath.Dir(d) {
			return nil
		}
	}
}

// removeParts removes downloads of name that an earlier install left unfinished.
func removeParts(dir, name string) {
	matches, _ := filepath.Glob(filepath.Join(dir, "."+name+"-*"+partSuffix))
	for _, m := range matches {
		os.Remove(m)
	}
}

func download(ctx context.Context, client *http.Client, url, target string, f File) (err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s: %s", url, resp.Status)
	}
	tmp, err := os.CreateTemp(filepath.Dir(target), "."+filepath.Base(target)+"-*"+partSuffix)
	if err != nil {
		return err
	}
	defer func() {
		tmp.Close()
		if err != nil {
			os.Remove(tmp.Name())
		}
	}()
	if err := tmp.Chmod(0o600); err != nil {
		return err
	}
	h := sha256.New()
	// One byte past the size is enough to tell a longer file.
	n, err := io.Copy(io.MultiWriter(tmp, h), io.LimitReader(resp.Body, f.Size+1))
	if err != nil {
		return fmt.Errorf("download %s: %w", url, err)
	}
	if n != f.Size {
		return fmt.Errorf("download %s: %d bytes, want %d; the file was not kept", url, n, f.Size)
	}
	if got := hex.EncodeToString(h.Sum(nil)); got != f.SHA256 {
		return fmt.Errorf("download %s: SHA-256 %s, want %s; the file was not kept", url, got, f.SHA256)
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), target)
}

// State is what Status found for one file.
type State string

const (
	Present State = "present"
	Missing State = "missing"
	Changed State = "changed"
)

// FileStatus is one file's state, with the reason for Changed.
type FileStatus struct {
	File   File
	State  State
	Reason string
}

// Status checks each file in dir again: present with its size and SHA-256, missing, or
// changed.
func Status(dir string, files []File) []FileStatus {
	var out []FileStatus
	for _, f := range files {
		err := check(filepath.Join(dir, filepath.FromSlash(f.Path)), f)
		switch {
		case err == nil:
			out = append(out, FileStatus{File: f, State: Present})
		case errors.Is(err, os.ErrNotExist):
			out = append(out, FileStatus{File: f, State: Missing})
		default:
			out = append(out, FileStatus{File: f, State: Changed, Reason: err.Error()})
		}
	}
	return out
}

func check(path string, f File) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return err
	}
	if info.Size() != f.Size {
		return fmt.Errorf("%d bytes, want %d", info.Size(), f.Size)
	}
	h := sha256.New()
	if _, err := io.Copy(h, file); err != nil {
		return err
	}
	if got := hex.EncodeToString(h.Sum(nil)); got != f.SHA256 {
		return fmt.Errorf("SHA-256 %s, want %s", got, f.SHA256)
	}
	return nil
}

// Summary is one line for a list of states: every file present, or what is not and the
// install command.
func Summary(dir string, states []FileStatus) string {
	var bad []string
	for _, s := range states {
		if s.State != Present {
			bad = append(bad, s.File.Path+" "+string(s.State))
		}
	}
	if len(bad) == 0 {
		return "Kokoro model files in " + dir + ": all present"
	}
	return "Kokoro model files in " + dir + ": " + strings.Join(bad, ", ") + "; run " + InstallCommand
}
