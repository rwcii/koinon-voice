package sidecar

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// The test binary is also the synthetic helper: with fakeMode set it runs fakeHelper
// instead of the tests (definition of done, Sidecar protocol).
const fakeMode = "KOINON_VOICE_FAKE_HELPER"

func TestMain(m *testing.M) {
	if mode := os.Getenv(fakeMode); mode != "" {
		fakeHelper(mode)
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func fakeHelper(mode string) {
	out := bufio.NewWriter(os.Stdout)
	send := func(m Message) {
		writeMessage(out, m)
		out.Flush()
	}
	switch mode {
	case "sleeper":
		time.Sleep(time.Minute)
		return
	case "silent":
		time.Sleep(time.Minute)
		return
	case "exit-early":
		fmt.Fprintln(os.Stderr, "no model")
		os.Exit(4)
	case "junk-first":
		os.Stdout.WriteString("Usage: helper [flags]\n")
		time.Sleep(time.Minute)
		return
	case "version":
		send(Message{Type: "hello", Protocol: 99})
		time.Sleep(time.Minute)
		return
	}
	send(Message{Type: "hello", Protocol: Version, Helper: "fake", HelperVer: "0.0.1", Capabilities: []string{"echo", "speak"}})
	in := bufio.NewReader(os.Stdin)
	var spoken []string
	for {
		f, err := readFrame(in)
		if err != nil {
			return // stdin closed: exit cleanly
		}
		m := f.msg
		if m == nil {
			continue
		}
		switch {
		case m.Type == "request" && mode == "error":
			send(Message{Type: "error", ID: m.ID, Code: "unavailable", Message: "appleIntelligenceNotEnabled"})
		case m.Type == "request" && mode == "crash":
			send(Message{Type: "event", ID: m.ID, Event: "delta", Data: json.RawMessage(`{"text":"Codex asks"}`)})
			fmt.Fprintln(os.Stderr, "fatal: model crashed")
			os.Exit(3)
		case m.Type == "request" && mode == "slow":
			child := exec.Command(os.Args[0])
			child.Env = append(os.Environ(), fakeMode+"=sleeper")
			child.Start()
			send(Message{Type: "event", ID: m.ID, Event: "child", Data: json.RawMessage(fmt.Sprintf(`{"pid":%d}`, child.Process.Pid))})
			// No answer: the bridge's deadline ends this.
		case m.Type == "request" && mode == "junk":
			os.Stdout.Write([]byte{0, 0, 0, 3, 'X', 'Y', 'Z'})
		case m.Type == "request" && m.Method == "echo":
			var p struct{ Text string }
			json.Unmarshal(m.Params, &p)
			send(Message{Type: "event", ID: m.ID, Event: "delta", Data: json.RawMessage(`{"text":"partial"}`)})
			data, _ := json.Marshal(map[string]any{"text": p.Text, "deadline_ms": m.DeadlineMS})
			send(Message{Type: "result", ID: m.ID, Data: data})
		case m.Type == "request" && m.Method == "speak":
			spoken = nil
		case m.Type == "input" && m.End:
			data, _ := json.Marshal(map[string]any{"spoken": strings.Join(spoken, "|")})
			send(Message{Type: "result", ID: m.ID, Data: data})
		case m.Type == "input":
			var p struct{ Text string }
			json.Unmarshal(m.Params, &p)
			if len(spoken) == 0 {
				send(Message{Type: "event", ID: m.ID, Event: "first_audio"})
			}
			spoken = append(spoken, p.Text)
		case m.Type == "request":
			send(Message{Type: "error", ID: m.ID, Code: "unknown_method", Message: m.Method})
		}
	}
}

func start(t *testing.T, mode string) *Helper {
	t.Helper()
	t.Setenv(fakeMode, mode)
	h, err := Start(context.Background(), os.Args[0])
	if err != nil {
		t.Fatalf("start %s: %v", mode, err)
	}
	t.Cleanup(func() {
		h.Close()
		gone(t, h.cmd.Process.Pid)
	})
	return h
}

// gone fails unless the process has ended; a killed process may take a moment to be reaped.
func gone(t *testing.T, pid int) {
	t.Helper()
	for i := 0; i < 200; i++ {
		if err := syscall.Kill(pid, 0); errors.Is(err, syscall.ESRCH) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Errorf("process %d is still running", pid)
}

func TestRequestAndResponse(t *testing.T) {
	h := start(t, "ok")
	if h.Hello().Helper != "fake" || !h.Hello().Has("echo") || h.Hello().Has("transcribe") {
		t.Errorf("hello %+v", h.Hello())
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var events []string
	data, err := h.Call(ctx, "echo", map[string]string{"text": "hello"}, func(e Event) { events = append(events, e.Name) })
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		Text       string
		DeadlineMS int64 `json:"deadline_ms"`
	}
	json.Unmarshal(data, &got)
	if got.Text != "hello" || got.DeadlineMS <= 0 || got.DeadlineMS > 5000 {
		t.Errorf("result %s", data)
	}
	if strings.Join(events, ",") != "delta" {
		t.Errorf("events %v", events)
	}
	// Requests are independent: a second one on the same helper works.
	if _, err := h.Call(ctx, "echo", map[string]string{"text": "again"}, nil); err != nil {
		t.Fatal(err)
	}
}

func TestStreamedInput(t *testing.T) {
	h := start(t, "ok")
	s, err := h.Open(context.Background(), "speak", map[string]string{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, text := range []string{"One sentence.", "Two."} {
		if err := s.Send(map[string]string{"text": text}); err != nil {
			t.Fatal(err)
		}
	}
	s.CloseInput()
	data, err := s.Wait()
	if err != nil || !strings.Contains(string(data), "One sentence.|Two.") {
		t.Fatalf("%s, %v", data, err)
	}
}

func TestEngineError(t *testing.T) {
	h := start(t, "error")
	_, err := h.Call(context.Background(), "echo", nil, nil)
	var e *Error
	if !errors.As(err, &e) || e.Code != "unavailable" || !strings.Contains(err.Error(), "appleIntelligenceNotEnabled") {
		t.Fatalf("error %v", err)
	}
	// An engine error is the request's, not the helper's: the helper still answers.
	if _, err := h.Call(context.Background(), "echo", nil, nil); !errors.As(err, &e) {
		t.Fatalf("second call %v", err)
	}
}

func TestUnknownMethod(t *testing.T) {
	h := start(t, "ok")
	_, err := h.Call(context.Background(), "summarize", nil, nil)
	var e *Error
	if !errors.As(err, &e) || e.Code != "unknown_method" {
		t.Fatalf("error %v", err)
	}
}

func TestCrashMidStream(t *testing.T) {
	h := start(t, "crash")
	var events int
	_, err := h.Call(context.Background(), "echo", nil, func(Event) { events++ })
	if !errors.Is(err, ErrExited) || !strings.Contains(err.Error(), "exit status 3") || !strings.Contains(err.Error(), "model crashed") {
		t.Fatalf("error %v", err)
	}
	if events != 1 {
		t.Errorf("events %d", events)
	}
	if _, err := h.Call(context.Background(), "echo", nil, nil); !errors.Is(err, ErrExited) {
		t.Errorf("call after crash: %v", err)
	}
}

func TestDeadlineKillsGroup(t *testing.T) {
	h := start(t, "slow")
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	child := 0
	_, err := h.Call(ctx, "echo", nil, func(e Event) {
		var p struct{ PID int }
		json.Unmarshal(e.Data, &p)
		child = p.PID
	})
	if !errors.Is(err, ErrDeadline) || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error %v", err)
	}
	if child == 0 {
		t.Fatal("the helper reported no child")
	}
	gone(t, child)
	gone(t, h.cmd.Process.Pid)
	if _, err := h.Call(context.Background(), "echo", nil, nil); !errors.Is(err, ErrDeadline) {
		t.Errorf("call after deadline: %v", err)
	}
}

func TestJunk(t *testing.T) {
	h := start(t, "junk")
	_, err := h.Call(context.Background(), "echo", nil, nil)
	if !errors.Is(err, ErrProtocol) {
		t.Fatalf("error %v", err)
	}
	gone(t, h.cmd.Process.Pid)
}

func TestStartFailures(t *testing.T) {
	old := HelloTimeout
	HelloTimeout = 300 * time.Millisecond
	t.Cleanup(func() { HelloTimeout = old })
	dir := t.TempDir()
	notExec := filepath.Join(dir, "helper")
	os.WriteFile(notExec, []byte("#!/bin/sh\n"), 0o600)
	for name, c := range map[string]struct {
		path, mode string
		is         error
		want       string
	}{
		"missing":        {filepath.Join(dir, "absent"), "", nil, "no such file"},
		"not executable": {notExec, "", nil, "permission denied"},
		"wrong version":  {os.Args[0], "version", ErrVersion, "99, want 1"},
		"junk first":     {os.Args[0], "junk-first", ErrProtocol, ""},
		"exits early":    {os.Args[0], "exit-early", ErrExited, "no model"},
		"no hello":       {os.Args[0], "silent", nil, "no hello"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Setenv(fakeMode, c.mode)
			h, err := Start(context.Background(), c.path)
			if err == nil {
				h.Close()
				t.Fatal("started")
			}
			if c.is != nil && !errors.Is(err, c.is) {
				t.Errorf("error %v, want %v", err, c.is)
			}
			if c.want != "" && !strings.Contains(err.Error(), c.want) {
				t.Errorf("error %v, want %q", err, c.want)
			}
		})
	}
}

func TestFrames(t *testing.T) {
	var b bytes.Buffer
	writeMessage(&b, Message{Type: "event", ID: 7, Event: "delta"})
	writeAudio(&b, 7, []byte{1, 2, 3, 4})
	f, err := readFrame(&b)
	if err != nil || f.msg == nil || f.msg.ID != 7 || f.msg.Event != "delta" {
		t.Fatalf("%+v %v", f, err)
	}
	f, err = readFrame(&b)
	if err != nil || f.msg != nil || f.id != 7 || !bytes.Equal(f.audio, []byte{1, 2, 3, 4}) {
		t.Fatalf("%+v %v", f, err)
	}
	if _, err := readFrame(&b); !errors.Is(err, io.EOF) {
		t.Fatalf("end %v", err)
	}
	for name, raw := range map[string][]byte{
		"zero length": {0, 0, 0, 0},
		"too long":    {0xff, 0xff, 0xff, 0xff},
		"truncated":   {0, 0, 0, 9, 'J', '{'},
		"short head":  {0, 0},
		"bad kind":    {0, 0, 0, 1, 'Q'},
		"bad json":    {0, 0, 0, 2, 'J', '{'},
		"no type":     append([]byte{0, 0, 0, 3, 'J'}, "{}"...),
		"short audio": {0, 0, 0, 2, 'A', 1},
	} {
		if _, err := readFrame(bytes.NewReader(raw)); !errors.Is(err, ErrProtocol) {
			t.Errorf("%s: %v", name, err)
		}
	}
	if err := writeFrame(io.Discard, kindJSON, make([]byte, MaxFrame)); err == nil {
		t.Error("an oversized frame was written")
	}
}
