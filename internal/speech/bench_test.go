package speech

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/rwcii/koinon-voice/internal/sidecar"
)

// The test binary is also a synthetic recognition helper: with fakeRecognizer set it answers
// transcribe, and never answers for a path containing "hang" (WDR 006: no model, no audio).
const fakeRecognizer = "KOINON_VOICE_FAKE_RECOGNIZER"

func TestMain(m *testing.M) {
	if os.Getenv(fakeRecognizer) != "" {
		out := bufio.NewWriter(os.Stdout)
		send := func(m sidecar.Message) {
			sidecar.WriteMessage(out, m)
			out.Flush()
		}
		send(sidecar.Message{Type: "hello", Protocol: sidecar.Version, Helper: "fake", Capabilities: []string{"transcribe"}})
		in := bufio.NewReader(os.Stdin)
		for {
			msg, err := sidecar.ReadMessage(in)
			if err != nil {
				os.Exit(0)
			}
			var p struct{ Path string }
			json.Unmarshal(msg.Params, &p)
			if !strings.Contains(p.Path, "hang") {
				send(sidecar.Message{Type: "result", ID: msg.ID, Data: json.RawMessage(`{"text":"stop"}`)})
			}
		}
	}
	os.Exit(m.Run())
}

// A request past its deadline kills the helper; when starting a new one fails, each later
// file is a failed row and the bench goes on to the next engine instead of panicking.
func TestRunSTTFailedRestart(t *testing.T) {
	t.Setenv(fakeRecognizer, "1")
	starts := 0
	broken := Recognizer{
		Name: "broken",
		Start: func(ctx context.Context) (*sidecar.Helper, error) {
			starts++
			if starts > 1 {
				return nil, errors.New("model gone")
			}
			return sidecar.Start(ctx, os.Args[0])
		},
		Params: func(p string) any { return map[string]string{"path": p} },
	}
	fine := Recognizer{Name: "fine", Start: func(ctx context.Context) (*sidecar.Helper, error) { return sidecar.Start(ctx, os.Args[0]) },
		Params: broken.Params}
	stop := phraseFor(t, "Stop.")
	files := []File{{Set: "clean", Phrase: stop, Path: "/c/01.wav"}, {Set: "clean", Phrase: stop, Path: "/c/hang.wav"},
		{Set: "clean", Phrase: stop, Path: "/c/03.wav"}, {Set: "clean", Phrase: stop, Path: "/c/04.wav"}}
	infos, ts := RunSTT(context.Background(), []Recognizer{broken, fine}, files, BenchOptions{Timeout: 200 * time.Millisecond, Warmup: 5 * time.Second})
	if len(infos) != 2 || infos[0].Err != nil || len(ts) != 8 {
		t.Fatalf("infos %+v, %d transcripts", infos, len(ts))
	}
	if !errors.Is(ts[1].Err, sidecar.ErrDeadline) {
		t.Errorf("hang: %v", ts[1].Err)
	}
	for _, i := range []int{2, 3} {
		if ts[i].Err == nil || !strings.Contains(ts[i].Err.Error(), "restart: model gone") || ts[i].Score.Errors != 1 {
			t.Errorf("file %d: %+v", i, ts[i])
		}
	}
	if ts[4].Err != nil || ts[4].Text != "stop" || ts[7].Err != nil {
		t.Errorf("the next engine: %+v %+v", ts[4], ts[7])
	}
}

func phraseFor(t *testing.T, s string) Phrase {
	t.Helper()
	p, err := parsePhrase(1, s)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestMedians(t *testing.T) {
	ms := func(vs ...int) []time.Duration {
		var out []time.Duration
		for _, v := range vs {
			out = append(out, time.Duration(v)*time.Millisecond)
		}
		return out
	}
	// The six Apple first-audio times of the committed synthesis run: the median is the mean
	// of 442 and 536.
	if got := medianDuration(ms(536, 442, 755, 412, 739, 424)); got != 489*time.Millisecond {
		t.Errorf("even: %v", got)
	}
	if got := medianDuration(ms(30, 10, 20)); got != 20*time.Millisecond {
		t.Errorf("odd: %v", got)
	}
	if medianDuration(nil) != 0 || medianFloat(nil) != 0 || medianFloat([]float64{1, 2}) != 1.5 {
		t.Error("empty or two")
	}
}
