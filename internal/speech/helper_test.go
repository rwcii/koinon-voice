package speech

import (
	"context"
	"errors"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/rwcii/koinon-voice/internal/sidecar"
)

// The Python bench helper speaks the sidecar protocol: its hello comes before any model
// loads, and an unknown method is that request's error. No model or audio is used.
func TestBenchHelperProtocol(t *testing.T) {
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("no python3: the bench helper's protocol is not checked")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	h, err := sidecar.Start(ctx, python, "../../helpers/bench/bench_helper.py", "whistle")
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	hello := h.Hello()
	if hello.Helper != "koinon-voice-bench-whistle" || !hello.Has("transcribe") || hello.Has("synthesize") ||
		!strings.Contains(string(hello.Info), `"model": "Cactus-Compute/whistle"`) {
		t.Errorf("hello %+v %s", hello, hello.Info)
	}
	_, err = h.Call(ctx, "synthesize", map[string]string{"text": "x"}, nil)
	var he *sidecar.Error
	if !errors.As(err, &he) || he.Code != "unknown_method" {
		t.Errorf("synthesize on whistle: %v", err)
	}
	// The helper survives the error and takes the next request.
	if _, err := h.Call(ctx, "speak", nil, nil); !errors.As(err, &he) || he.Code != "unknown_method" {
		t.Errorf("speak: %v", err)
	}

	cmd := exec.Command(python, "../../helpers/bench/bench_helper.py", "nope")
	if err := cmd.Run(); err == nil || cmd.ProcessState.ExitCode() != 2 {
		t.Errorf("unknown engine: %v", err)
	}
}
