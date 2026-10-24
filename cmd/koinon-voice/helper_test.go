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

// The test binary is also a synthetic helper: with fakeHelperMode set it answers transcribe,
// speak and synthesize instead of running the tests (WDR 006: no audio, no recognition). It
// stands in for both the Apple helper and the bench helper, whose arguments it ignores.
const fakeHelperMode = "KOINON_VOICE_FAKE_CMD_HELPER"

// fakeDuplexAfter makes duplex hear the interruption only after playback, with no barge-in.
const fakeDuplexAfter = "KOINON_VOICE_FAKE_DUPLEX_AFTER"

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
		Capabilities: []string{"duplex", "speak", "synthesize", "transcribe", "wake"}, Info: json.RawMessage(`{"speech":"1.2.3"}`)})
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
			switch {
			case strings.HasSuffix(p.Path, ".aiff"):
				send(sidecar.Message{Type: "error", ID: m.ID, Code: "bad_audio", Message: "want 16 kHz mono"})
			case strings.HasSuffix(p.Path, "/02.wav"):
				send(sidecar.Message{Type: "result", ID: m.ID, Data: json.RawMessage(`{"text":"Approved pull request 12."}`)})
			case strings.HasSuffix(p.Path, "/03.wav"):
				send(sidecar.Message{Type: "error", ID: m.ID, Code: "engine_error", Message: "decoder failed"})
			default:
				send(sidecar.Message{Type: "result", ID: m.ID, Data: json.RawMessage(`{"text":"Koinon, read my inbox."}`)})
			}
		case m.Type == "request" && m.Method == "duplex":
			// The interruption is heard, with one of the bridge's words leaking through.
			var p struct {
				Capture      string
				StopOnSpeech bool `json:"stop_on_speech"`
			}
			json.Unmarshal(m.Params, &p)
			os.WriteFile(p.Capture, []byte("RIFF"), 0o600)
			send(sidecar.Message{Type: "event", ID: m.ID, Event: "speaking"})
			if os.Getenv(fakeDuplexAfter) != "" {
				// The whole interruption is heard, but only after the bridge finished speaking.
				send(sidecar.Message{Type: "result", ID: m.ID, Data: json.RawMessage(`{"transcript":"Stop, wait, I have a question.","voice":"fake-voice","spoken_seconds":15.7,"barge_in_ms":null,"listened_seconds":25,"voice_processing":true}`)})
				continue
			}
			send(sidecar.Message{Type: "event", ID: m.ID, Event: "barge_in", Data: json.RawMessage(`{"since_start_ms":3100}`)})
			send(sidecar.Message{Type: "event", ID: m.ID, Event: "final", Data: json.RawMessage(`{"text":"Stop, wait. I have a question about the hotfix."}`)})
			send(sidecar.Message{Type: "result", ID: m.ID, Data: json.RawMessage(`{"transcript":"Stop, wait. I have a question about the hotfix.","voice":"fake-voice","spoken_seconds":15.7,"barge_in_ms":3100,"listened_seconds":25,"voice_processing":true}`)})
		case m.Type == "request" && m.Method == "wake":
			// A take wakes once (two firings), music wakes once, and the podcast never.
			var p struct{ Path string }
			json.Unmarshal(m.Params, &p)
			events := `[]`
			switch {
			case strings.Contains(p.Path, "/wake/near/02.wav"):
			case strings.Contains(p.Path, "/wake/"):
				events = `[0.88, 0.96]`
			case strings.Contains(p.Path, "/background/music/"):
				events = `[12.5]`
			}
			send(sidecar.Message{Type: "result", ID: m.ID, Data: json.RawMessage(`{"events":` + events + `,"max_score":0.9}`)})
		case m.Type == "request" && m.Method == "synthesize":
			var p struct{ Text, Output string }
			json.Unmarshal(m.Params, &p)
			os.WriteFile(p.Output, []byte(p.Text), 0o600)
			send(sidecar.Message{Type: "event", ID: m.ID, Event: "first_audio", Data: json.RawMessage(`{"since_request_ms":7}`)})
			send(sidecar.Message{Type: "result", ID: m.ID, Data: json.RawMessage(`{"audio_seconds":1.5}`)})
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
	for _, want := range []string{"hello: fake 0.0.1, capabilities [duplex speak synthesize transcribe wake]", `{"speech":"1.2.3"}`,
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
