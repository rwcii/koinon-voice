package speech

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/rwcii/koinon-voice/internal/kokoro"
	"github.com/rwcii/koinon-voice/internal/sidecar"
	"github.com/rwcii/koinon-voice/internal/summarize"
)

// The test binary is also a synthetic Kokoro helper and a synthetic Apple helper: with
// fakeSpeakerLog set to a file, it is the Kokoro helper when started with --models and the
// Apple helper otherwise, and both append what they receive to that file, one line each, so
// a test reads the order of events across the two (WDR 006: no model, no audio).
const (
	fakeSpeakerLog  = "KOINON_VOICE_FAKE_SPEAKER_LOG"
	fakeSpeakerMode = "KOINON_VOICE_FAKE_SPEAKER_MODE"
)

func init() {
	if path := os.Getenv(fakeSpeakerLog); path != "" {
		fakeSpeaker(path, os.Getenv(fakeSpeakerMode), slices.Contains(os.Args[1:], "--models"))
		os.Exit(0)
	}
}

func fakeSpeaker(path, mode string, isKokoro bool) {
	logf, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		os.Exit(9)
	}
	role := "apple"
	if isKokoro {
		role = "kokoro"
	}
	log := func(format string, a ...any) { fmt.Fprintf(logf, role+" "+format+"\n", a...) }
	out := bufio.NewWriter(os.Stdout)
	send := func(m sidecar.Message) {
		sidecar.WriteMessage(out, m)
		out.Flush()
	}
	log("start %d", os.Getpid())
	if isKokoro {
		info := `{}`
		if mode == "no-model" {
			info = `{"error":"no Kokoro model: kokoro-v1_0.safetensors is missing; run koinon-voice kokoro install"}`
		}
		send(sidecar.Message{Type: "hello", Protocol: sidecar.Version, Helper: "koinon-voice-kokoro", Capabilities: []string{"synthesize"}, Info: json.RawMessage(info)})
	} else {
		send(sidecar.Message{Type: "hello", Protocol: sidecar.Version, Helper: "koinon-voice-apple", Capabilities: []string{"play", "speak"}})
	}
	in := bufio.NewReader(os.Stdin)
	sentences := 0
	plays := map[uint64]int{} // frames received by each play request
	for {
		m, audio, id, err := readAny(in)
		if err != nil {
			return
		}
		if m == nil {
			// Audio for a play request: its first two bytes name the sentence and the frame.
			log("audio %d %d", audio[0], audio[1])
			if plays[id] == 0 {
				if mode == "apple-crash" {
					fmt.Fprintln(os.Stderr, "fatal: audio unit failed")
					os.Exit(3)
				}
				send(sidecar.Message{Type: "event", ID: id, Event: "first_audio", Data: json.RawMessage(`{"since_request_ms":3}`)})
			}
			plays[id]++
			continue
		}
		switch {
		case m.Type == "request" && m.Method == "synthesize":
			log("synthesize %s", m.Params)
			sentences++
			var p struct{ Text string }
			json.Unmarshal(m.Params, &p)
			if strings.Contains(p.Text, "slow") {
				time.Sleep(time.Minute) // a sentence still being synthesized when the stop comes
			}
			for f := 0; f < 3; f++ {
				if strings.Contains(p.Text, "crash") && f == 1 {
					fmt.Fprintln(os.Stderr, "fatal: synthesis crashed")
					os.Exit(3)
				}
				sidecar.WriteAudio(out, m.ID, []byte{byte(sentences), byte(f), 0x34, 0x12})
				out.Flush()
			}
			send(sidecar.Message{Type: "result", ID: m.ID, Data: json.RawMessage(`{"audio_seconds":0.6}`)})
		case m.Type == "request":
			log("%s %s", m.Method, m.Params)
			plays[m.ID] = 0
		case m.Type == "input" && m.End:
			log("end")
			if mode == "hold-play" {
				continue // the last sentence is still playing
			}
			data, _ := json.Marshal(map[string]any{"voice": "fake-voice", "audio_seconds": float64(plays[m.ID]) * 0.2})
			send(sidecar.Message{Type: "result", ID: m.ID, Data: data})
		case m.Type == "input":
			log("input %s", m.Params)
			send(sidecar.Message{Type: "event", ID: m.ID, Event: "first_audio", Data: json.RawMessage(`{}`)})
		case m.Type == "cancel":
			log("cancel")
		}
	}
}

// readAny reads the next frame: a message, or audio with its request ID.
func readAny(r *bufio.Reader) (*sidecar.Message, []byte, uint64, error) {
	var head [4]byte
	if _, err := io.ReadFull(r, head[:]); err != nil {
		return nil, nil, 0, err
	}
	n := int(head[0])<<24 | int(head[1])<<16 | int(head[2])<<8 | int(head[3])
	body := make([]byte, n)
	if _, err := io.ReadFull(r, body); err != nil {
		return nil, nil, 0, err
	}
	if body[0] == 'A' {
		var id uint64
		for _, b := range body[1:9] {
			id = id<<8 | uint64(b)
		}
		return nil, body[9:], id, nil
	}
	var m sidecar.Message
	err := json.Unmarshal(body[1:], &m)
	return &m, nil, 0, err
}

type speakerTest struct {
	t       *testing.T
	log     string
	speaker *Speaker
}

// newSpeakerTest makes a speaker on the synthetic helpers, with empty model files for the
// configured voice in a temporary model directory.
func newSpeakerTest(t *testing.T, mode string, config summarize.Config) *speakerTest {
	t.Helper()
	dir := t.TempDir()
	log := filepath.Join(dir, "helpers.log")
	t.Setenv(fakeSpeakerLog, log)
	t.Setenv(fakeSpeakerMode, mode)
	models := filepath.Join(dir, "kokoro")
	sc := config.Speaking()
	for _, f := range append(kokoroPaths(), "voices/"+sc.Voice+".safetensors") {
		path := filepath.Join(models, filepath.FromSlash(f))
		os.MkdirAll(filepath.Dir(path), 0o700)
		os.WriteFile(path, nil, 0o600)
	}
	launcher := &summarize.Launcher{Path: os.Args[0]}
	t.Cleanup(launcher.Close)
	return &speakerTest{t: t, log: log, speaker: &Speaker{Speech: sc, Apple: launcher, KokoroHelper: os.Args[0], Models: models}}
}

func kokoroPaths() []string {
	var out []string
	for _, f := range kokoro.Files {
		out = append(out, f.Path)
	}
	return out
}

// lines are the helpers' log lines so far.
func (s *speakerTest) lines() []string {
	data, _ := os.ReadFile(s.log)
	return strings.Split(strings.TrimSpace(string(data)), "\n")
}

// waitFor waits until a log line starts with prefix and returns its index.
func (s *speakerTest) waitFor(prefix string) int {
	s.t.Helper()
	for i := 0; i < 500; i++ {
		for j, l := range s.lines() {
			if strings.HasPrefix(l, prefix) {
				return j
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	s.t.Fatalf("no log line %q in:\n%s", prefix, strings.Join(s.lines(), "\n"))
	return -1
}

// pids are the Kokoro helpers started, in order.
func (s *speakerTest) pids() []int {
	var out []int
	for _, l := range s.lines() {
		if p, ok := strings.CutPrefix(l, "kokoro start "); ok {
			n, _ := strconv.Atoi(p)
			out = append(out, n)
		}
	}
	return out
}

func (s *speakerTest) say(ctx context.Context, text string) (summarize.Spoken, error) {
	sp, err := s.speaker.Start(ctx)
	if err != nil {
		return summarize.Spoken{}, err
	}
	sp.Write(text)
	return sp.Finish()
}

func gone(t *testing.T, pid int) {
	t.Helper()
	for i := 0; i < 300; i++ {
		if err := syscall.Kill(pid, 0); errors.Is(err, syscall.ESRCH) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Errorf("Kokoro helper %d is still running", pid)
}

// Checks 1, 2 and 3: synthesize gets the defaults, the frames reach play in order and
// unchanged at 24 kHz, and sentence 2 is requested before play has its end.
func TestKokoroSpeaks(t *testing.T) {
	s := newSpeakerTest(t, "", summarize.Config{})
	sp, err := s.say(context.Background(), "Codex asks you to merge. Claude agrees.")
	if err != nil {
		t.Fatal(err)
	}
	if sp.Voice != "af_heart" || sp.AudioSeconds < 1.19 || sp.AudioSeconds > 1.21 || sp.FirstAudio.Before(sp.FirstSentence) || sp.FirstSentence.IsZero() {
		t.Errorf("spoken %+v", sp)
	}
	var got []string
	for _, l := range s.lines() {
		if !strings.HasPrefix(l, "kokoro start") && !strings.HasPrefix(l, "apple start") {
			got = append(got, l)
		}
	}
	want := []string{
		`apple play {"sample_rate":24000}`,
		`kokoro synthesize {"speed":1.1,"text":"Codex asks you to merge.","voice":"af_heart"}`,
		"apple audio 1 0", "apple audio 1 1", "apple audio 1 2",
		`kokoro synthesize {"speed":1.1,"text":"Claude agrees.","voice":"af_heart"}`,
		"apple audio 2 0", "apple audio 2 1", "apple audio 2 2",
		"apple end",
	}
	// The Kokoro helper may log its next request before the Apple helper logs the frames
	// before it; each helper's own lines are in order.
	for _, role := range []string{"apple", "kokoro"} {
		if g, w := only(got, role), only(want, role); !slices.Equal(g, w) {
			t.Errorf("%s received:\n%s\nwant:\n%s", role, strings.Join(g, "\n"), strings.Join(w, "\n"))
		}
	}
	if slices.Index(got, want[5]) > slices.Index(got, "apple end") {
		t.Errorf("sentence 2 was requested after play ended:\n%s", strings.Join(got, "\n"))
	}
	if pids := s.pids(); len(pids) != 1 {
		t.Errorf("Kokoro helpers %v, want one", pids)
	} else {
		gone(t, pids[0])
	}
}

func only(lines []string, role string) []string {
	var out []string
	for _, l := range lines {
		if strings.HasPrefix(l, role+" ") {
			out = append(out, l)
		}
	}
	return out
}

// Check 1: the engines file's voice and speed reach synthesize.
func TestKokoroConfiguredVoice(t *testing.T) {
	s := newSpeakerTest(t, "", summarize.Config{Speech: summarize.SpeechConfig{Voice: "am_michael", Speed: 0.9}})
	if _, err := s.say(context.Background(), "Ready."); err != nil {
		t.Fatal(err)
	}
	s.waitFor(`kokoro synthesize {"speed":0.9,"text":"Ready.","voice":"am_michael"}`)
}

// Check 4: a stop during sentence 1's synthesis ends the Kokoro helper's process and cancels
// play; no frame reaches play after it; the next text starts a new helper and plays.
func TestKokoroStopDuringSynthesis(t *testing.T) {
	s := newSpeakerTest(t, "", summarize.Config{})
	ctx, stop := context.WithCancel(context.Background())
	sp, err := s.speaker.Start(ctx)
	if err != nil {
		t.Fatal(err)
	}
	sp.Write("A slow sentence. Never synthesized.")
	s.waitFor("kokoro synthesize")
	stop()
	if _, err := sp.Finish(); !errors.Is(err, context.Canceled) {
		t.Fatalf("finish: %v, want context.Canceled", err)
	}
	stopped := len(s.lines())
	s.waitFor("apple cancel")
	gone(t, s.pids()[0])
	for _, l := range s.lines() {
		if strings.HasPrefix(l, "apple audio") || strings.Contains(l, "Never synthesized") {
			t.Errorf("after the stop: %s", l)
		}
	}

	if _, err := s.say(context.Background(), "Next text."); err != nil {
		t.Fatal(err)
	}
	if pids := s.pids(); len(pids) != 2 || pids[0] == pids[1] {
		t.Errorf("Kokoro helpers %v, want a new one for the next text", pids)
	}
	if after := strings.Join(s.lines()[stopped:], "\n"); !strings.Contains(after, "apple audio 1 2") || !strings.Contains(after, "apple end") {
		t.Errorf("the next text did not play:\n%s", after)
	}
}

// Check 4: a stop during playback cancels play at once.
func TestKokoroStopDuringPlayback(t *testing.T) {
	s := newSpeakerTest(t, "", summarize.Config{})
	ctx, stop := context.WithCancel(context.Background())
	sp, err := s.speaker.Start(ctx)
	if err != nil {
		t.Fatal(err)
	}
	sp.Write("Codex asks you to merge. ")
	s.waitFor("apple audio 1 2")
	stop()
	s.waitFor("apple cancel")
	if _, err := sp.Finish(); !errors.Is(err, context.Canceled) {
		t.Fatalf("finish: %v, want context.Canceled", err)
	}
	for _, l := range s.lines() {
		if l == "apple end" {
			t.Errorf("play was ended, not cancelled")
		}
	}
}

// Check 5: a Kokoro helper that exits mid-sentence gives an error that names it, and play is
// cancelled.
func TestKokoroHelperExits(t *testing.T) {
	s := newSpeakerTest(t, "", summarize.Config{})
	_, err := s.say(context.Background(), "This will crash.")
	if err == nil || !strings.Contains(err.Error(), "Kokoro helper") || !strings.Contains(err.Error(), "synthesis crashed") {
		t.Fatalf("err %v, want the Kokoro helper's exit", err)
	}
	s.waitFor("apple cancel")
}

// An Apple helper that exits mid-text gives an error that names it, and the Kokoro helper
// is ended.
func TestAppleHelperExitsDuringPlay(t *testing.T) {
	s := newSpeakerTest(t, "apple-crash", summarize.Config{})
	_, err := s.say(context.Background(), "One sentence. A slow second one.")
	if err == nil || !strings.Contains(err.Error(), "Apple helper") || !strings.Contains(err.Error(), "audio unit failed") {
		t.Fatalf("err %v, want the Apple helper's exit", err)
	}
	gone(t, s.pids()[0])
}

// Check 6: with engine = "apple", speak is used as now, with the configured Apple voice, and
// no Kokoro helper starts.
func TestAppleEngine(t *testing.T) {
	s := newSpeakerTest(t, "", summarize.Config{Voice: "com.apple.voice.top", Speech: summarize.SpeechConfig{Engine: "apple"}})
	if s.speaker.Speech.Voice != "com.apple.voice.top" {
		t.Fatalf("voice %q, want the top-level voice", s.speaker.Speech.Voice)
	}
	if _, err := s.say(context.Background(), "Hello."); err != nil {
		t.Fatal(err)
	}
	s.waitFor(`apple speak {"voice":"com.apple.voice.top"}`)
	s.waitFor(`apple input {"text":"Hello."}`)
	if pids := s.pids(); len(pids) != 0 {
		t.Errorf("Kokoro helpers %v, want none", pids)
	}
}

// Check 7: a missing helper or model file is named with the command that installs it, before
// any helper starts.
func TestKokoroMissing(t *testing.T) {
	s := newSpeakerTest(t, "", summarize.Config{})
	s.speaker.KokoroHelper = filepath.Join(t.TempDir(), "koinon-voice-kokoro")
	_, err := s.speaker.Start(context.Background())
	if err == nil || !strings.Contains(err.Error(), s.speaker.KokoroHelper) || !strings.Contains(err.Error(), KokoroBuild) {
		t.Errorf("missing helper: %v", err)
	}

	s.speaker.KokoroHelper = os.Args[0]
	model := filepath.Join(s.speaker.Models, "kokoro-v1_0.safetensors")
	os.Remove(model)
	_, err = s.speaker.Start(context.Background())
	if err == nil || !strings.Contains(err.Error(), model) || !strings.Contains(err.Error(), kokoro.InstallCommand) {
		t.Errorf("missing model: %v", err)
	}
	if _, statErr := os.Stat(s.log); statErr == nil {
		t.Errorf("a helper started:\n%s", strings.Join(s.lines(), "\n"))
	}

	// A model the helper itself cannot read is named from its hello.
	s = newSpeakerTest(t, "no-model", summarize.Config{})
	if _, err := s.speaker.Start(context.Background()); err == nil || !strings.Contains(err.Error(), kokoro.InstallCommand) {
		t.Errorf("hello error: %v", err)
	}
}

// Check 4: a stop while the last sentence plays, after all of it is synthesized, cancels play.
func TestKokoroStopDuringLastSentence(t *testing.T) {
	s := newSpeakerTest(t, "hold-play", summarize.Config{})
	ctx, stop := context.WithCancel(context.Background())
	sp, err := s.speaker.Start(ctx)
	if err != nil {
		t.Fatal(err)
	}
	sp.Write("The only sentence.")
	done := make(chan error, 1)
	go func() {
		_, err := sp.Finish()
		done <- err
	}()
	s.waitFor("apple end")
	stop()
	s.waitFor("apple cancel")
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("finish: %v, want context.Canceled", err)
	}
}
