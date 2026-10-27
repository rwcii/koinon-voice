package main

import (
	"bytes"
	"encoding/json"
	"runtime"
	"strings"
	"testing"
)

// isCommandError reports whether the last line of stderr is koinon's error object for a
// command line, pointing at koinon-voice --help.
func isCommandError(stderr string) bool {
	lines := strings.Split(strings.TrimSpace(stderr), "\n")
	var e map[string]any
	if json.Unmarshal([]byte(lines[len(lines)-1]), &e) != nil || len(e) != 3 {
		return false
	}
	detail, _ := e["detail"].(string)
	return e["code"] == "daemon_error" && e["ok"] == false && strings.HasSuffix(detail, "; use koinon-voice --help")
}

func TestVersion(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run([]string{"version"}, nil, &stdout, &stderr); code != 0 {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	var v map[string]string
	if err := json.Unmarshal(stdout.Bytes(), &v); err != nil {
		t.Fatalf("%v: %q", err, stdout.String())
	}
	_, hasCommit := v["commit"]
	if len(v) != 5 || v["version"] != "dev" || v["os"] != runtime.GOOS || v["arch"] != runtime.GOARCH || v["go"] != runtime.Version() || !hasCommit {
		t.Errorf("version %v", v)
	}
	if !strings.HasPrefix(stdout.String(), "{\n  \"arch\"") {
		t.Errorf("not indented as koinon version: %q", stdout.String())
	}
}

func TestUsage(t *testing.T) {
	for _, args := range [][]string{nil, {"--help"}, {"help"}} {
		var stdout, stderr bytes.Buffer
		if code := run(args, nil, &stdout, &stderr); code != 0 || stderr.Len() != 0 || stdout.String() != usage {
			t.Errorf("%q: exit %d stdout %q stderr %q", args, code, stdout.String(), stderr.String())
		}
	}
	for _, cmd := range []string{"version", "board", "dashboard", "summarize", "helper", "corpus record", "corpus wake", "corpus background", "corpus status", "bench stt", "bench tts", "bench wake", "bench interrupt", "bench names"} {
		if !strings.Contains(usage, "koinon-voice "+cmd) {
			t.Errorf("usage lacks %s", cmd)
		}
	}
	for args, detail := range map[string]string{
		"bogus":         "unknown command; use koinon-voice --help",
		"-h":            "unknown command; use koinon-voice --help",
		"version extra": "invalid command options; use koinon-voice --help",
		"board --help":  "invalid command options; use koinon-voice --help",
		"board --nope":  "invalid command options; use koinon-voice --help",
	} {
		var stdout, stderr bytes.Buffer
		code := run(strings.Fields(args), nil, &stdout, &stderr)
		want := `{"code":"daemon_error","detail":"` + detail + `","ok":false}` + "\n"
		if code != 1 || stdout.Len() != 0 || stderr.String() != want {
			t.Errorf("%q: exit %d stdout %q stderr %q", args, code, stdout.String(), stderr.String())
		}
	}
}
