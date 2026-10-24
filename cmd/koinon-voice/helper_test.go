package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rwcii/koinon-voice/internal/sidecar"
)

// The test binary is also a synthetic helper: with fakeHelperMode set it answers transcribe
// and speak instead of running the tests (WDR 006: no audio, no recognition).
const fakeHelperMode = "KOINON_VOICE_FAKE_CMD_HELPER"

func TestMain(m *testing.M) {
	if os.Getenv(fakeHelperMode) != "" {
		fakeHelper()
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func fakeHelper() {
	out := bufio.NewWriter(os.Stdout)
	send := func(m sidecar.Message) {
		sidecar.WriteMessage(out, m)
		out.Flush()
	}
	send(sidecar.Message{Type: "hello", Protocol: sidecar.Version, Helper: "fake", HelperVer: "0.0.1",
		Capabilities: []string{"speak", "transcribe"}, Info: json.RawMessage(`{"speech":"1.2.3"}`)})
	in := bufio.NewReader(os.Stdin)
	for {
		m, err := sidecar.ReadMessage(in)
		if err != nil {
			return
		}
		switch {
		case m.Type == "request" && m.Method == "transcribe":
			var p struct{ Path string }
			json.Unmarshal(m.Params, &p)
			if strings.HasSuffix(p.Path, ".aiff") {
				send(sidecar.Message{Type: "error", ID: m.ID, Code: "bad_audio", Message: "want 16 kHz mono"})
			} else {
				send(sidecar.Message{Type: "result", ID: m.ID, Data: json.RawMessage(`{"text":"Koinon, read my inbox."}`)})
			}
		case m.Type == "input" && !m.End:
			send(sidecar.Message{Type: "event", ID: m.ID, Event: "first_audio", Data: json.RawMessage(`{"since_request_ms":2}`)})
		case m.Type == "input" && m.End:
			send(sidecar.Message{Type: "result", ID: m.ID, Data: json.RawMessage(`{"voice":"fake-voice"}`)})
		}
	}
}

func TestHelperCommand(t *testing.T) {
	t.Setenv(fakeHelperMode, "1")
	var stdout, stderr bytes.Buffer
	code := run([]string{"helper", "--helper", os.Args[0], "--transcribe", "check.wav", "--say", "Hello."}, nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	for _, want := range []string{"hello: fake 0.0.1, capabilities [speak transcribe]", `{"speech":"1.2.3"}`,
		`transcribe: {"text":"Koinon, read my inbox."}`, "speak: first audio", `speak: {"voice":"fake-voice"}`} {
		if !strings.Contains(stdout.String(), want) {
			t.Errorf("stdout lacks %q:\n%s", want, stdout.String())
		}
	}

	stdout.Reset()
	stderr.Reset()
	if code := run([]string{"helper", "--helper", os.Args[0], "--transcribe", "check.aiff"}, nil, &stdout, &stderr); code != 1 ||
		!strings.Contains(stderr.String(), "transcribe: bad_audio: want 16 kHz mono") {
		t.Errorf("exit %d, stderr %q", code, stderr.String())
	}
}

func TestHelperCommandFailures(t *testing.T) {
	dir := t.TempDir()
	engines := filepath.Join(dir, "engines.toml")
	os.WriteFile(engines, []byte("[[engine]]\nname = \"a\"\nadapter = \"openai\"\nbase_url = \"u\"\nmodel = \"m\"\nlocal = true\n"), 0o600)
	for name, args := range map[string][]string{
		"no helper": {"helper", "--engines", engines},
		"missing":   {"helper", "--helper", filepath.Join(dir, "absent")},
	} {
		var stdout, stderr bytes.Buffer
		if code := run(args, nil, &stdout, &stderr); code != 1 || stderr.Len() == 0 {
			t.Errorf("%s: exit %d, stderr %q", name, code, stderr.String())
		}
	}
	var stdout, stderr bytes.Buffer
	if code := run([]string{"helper", "extra"}, nil, &stdout, &stderr); code != 2 {
		t.Errorf("extra argument: exit %d", code)
	}
}
