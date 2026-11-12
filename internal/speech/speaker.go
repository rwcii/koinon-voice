package speech

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/rwcii/koinon-voice/internal/kokoro"
	"github.com/rwcii/koinon-voice/internal/sidecar"
	"github.com/rwcii/koinon-voice/internal/summarize"
)

// KokoroBuild is the command that builds and installs koinon-voice-kokoro, for error messages.
const KokoroBuild = "scripts/build-kokoro-helper.sh"

// DefaultKokoroHelper is where scripts/build-kokoro-helper.sh installs koinon-voice-kokoro.
func DefaultKokoroHelper() string {
	return filepath.Join(filepath.Dir(kokoro.DefaultDir()), "bin", "kokoro", "koinon-voice-kokoro")
}

// Speaker speaks texts with the engine the engines file chooses (ADR 002, Decision 2):
// Kokoro through koinon-voice-kokoro, played by the Apple helper's play, or Apple's voice
// through the Apple helper's speak. It is a summarize.Speaker. The context given to Start
// is the stop: cancelling it ends playback and synthesis.
type Speaker struct {
	// Speech is the engine, voice and speed, with the defaults filled in.
	Speech summarize.SpeechConfig
	// Apple runs the Apple helper, which plays for both engines.
	Apple *summarize.Launcher
	// KokoroHelper is koinon-voice-kokoro's path, and Models its model directory.
	KokoroHelper string
	Models       string
	// Output, when set, is an audio file the speech is written to in place of the output
	// device.
	Output string
}

// Check reports a Kokoro helper or model file that is missing, with the command that
// installs it, so a command can fail before any audio (criterion 5). Apple's voice needs
// neither.
func (s *Speaker) Check() error {
	if s.Speech.Engine != summarize.SpeechKokoro {
		return nil
	}
	if info, err := os.Stat(s.KokoroHelper); err != nil || info.IsDir() || info.Mode()&0o111 == 0 {
		return fmt.Errorf("no Kokoro helper: %s is missing; build and install it with %s", s.KokoroHelper, KokoroBuild)
	}
	files := []string{}
	for _, f := range kokoro.Files {
		files = append(files, f.Path)
	}
	if voice := "voices/" + s.Speech.Voice + ".safetensors"; !contains(files, voice) {
		files = append(files, voice)
	}
	var missing []string
	for _, f := range files {
		if _, err := os.Stat(filepath.Join(s.Models, filepath.FromSlash(f))); err != nil {
			missing = append(missing, filepath.Join(s.Models, filepath.FromSlash(f)))
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("no Kokoro model: %s missing; run %s", strings.Join(missing, ", "), kokoro.InstallCommand)
	}
	return nil
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// Start opens one text's speech. For Kokoro it starts a new koinon-voice-kokoro, so a text
// after a stop has a helper of its own (definition of done, Stopping).
func (s *Speaker) Start(ctx context.Context) (summarize.Speech, error) {
	if s.Speech.Engine == summarize.SpeechApple {
		return (&summarize.Voice{Helper: s.Apple, Voice: s.Speech.Voice, Output: s.Output}).Start(ctx)
	}
	if err := s.Check(); err != nil {
		return nil, err
	}
	apple, err := s.Apple.Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("Apple helper: %w", err)
	}
	if !apple.Hello().Has("play") {
		return nil, errors.New("Apple helper: it does not offer play; build it again from helpers/apple")
	}
	k, err := sidecar.Start(ctx, s.KokoroHelper, "--models", s.Models)
	if err != nil {
		return nil, fmt.Errorf("Kokoro helper: %w", err)
	}
	var info struct{ Error string }
	if json.Unmarshal(k.Hello().Info, &info) == nil && info.Error != "" {
		k.Close()
		return nil, fmt.Errorf("Kokoro helper: %s", info.Error)
	}

	sp := &kokoroSpeech{kokoro: k, voice: s.Speech.Voice, speed: s.Speech.Speed, wake: make(chan struct{}, 1), done: make(chan struct{}), finished: make(chan struct{}), played: make(chan outcome, 1)}
	sp.kctx, sp.kcancel = context.WithCancel(ctx)
	// play keeps ctx's deadline, past which the bridge kills a wedged helper; a stop cancels
	// play and leaves the Apple helper running.
	pctx := context.WithoutCancel(ctx)
	cancelPlay := context.CancelFunc(func() {})
	if d, ok := ctx.Deadline(); ok {
		pctx, cancelPlay = context.WithDeadline(pctx, d)
	}
	params := map[string]any{"sample_rate": KokoroRate}
	if s.Output != "" {
		params["output"] = s.Output
	}
	sp.play, err = apple.Open(pctx, "play", params, func(e sidecar.Event) {
		if e.Name == "first_audio" {
			sp.mu.Lock()
			if sp.firstAudio.IsZero() {
				sp.firstAudio = e.At
			}
			sp.mu.Unlock()
		}
	})
	if err != nil {
		cancelPlay()
		sp.kcancel()
		k.Close()
		return nil, fmt.Errorf("Apple helper: %w", err)
	}
	go func() {
		data, err := sp.play.Wait()
		cancelPlay()
		sp.played <- outcome{data, err}
		sp.mu.Lock()
		ended := sp.inputClosed
		sp.mu.Unlock()
		if !ended && !errors.Is(err, context.Canceled) {
			// The Apple helper ended play before the text did: an exit or an error.
			sp.fail(fmt.Errorf("Apple helper: %w", err))
		}
	}()
	go func() {
		select {
		case <-ctx.Done():
			if errors.Is(ctx.Err(), context.Canceled) {
				sp.play.Cancel()
			}
		case <-sp.finished:
		}
	}()
	go sp.run()
	return sp, nil
}

type outcome struct {
	data json.RawMessage
	err  error
}

// kokoroSpeech is one text spoken with Kokoro: each sentence is synthesized in turn, and its
// frames go to the Apple helper's play as they arrive, so the next sentence is synthesized
// while the ones before it play (criterion 4).
type kokoroSpeech struct {
	kokoro   *sidecar.Helper
	play     *sidecar.Stream
	played   chan outcome
	voice    string
	speed    float64
	kctx     context.Context // synthesize requests; cancelling it ends the Kokoro helper
	kcancel  context.CancelFunc
	wake     chan struct{}
	done     chan struct{} // closed when run has returned
	finished chan struct{} // closed when Finish has returned

	mu          sync.Mutex
	buf         strings.Builder
	queue       []string
	inputDone   bool
	inputClosed bool // play's input is closed
	firstSent   time.Time
	firstAudio  time.Time
	err         error
}

// Write adds streamed text and queues each sentence it completes.
func (k *kokoroSpeech) Write(delta string) {
	k.mu.Lock()
	k.buf.WriteString(delta)
	for {
		text := k.buf.String()
		loc := sentenceEnd.FindStringIndex(text)
		if loc == nil {
			break
		}
		k.add(text[:loc[1]])
		k.buf.Reset()
		k.buf.WriteString(text[loc[1]:])
	}
	k.mu.Unlock()
	k.signal()
}

// add queues one sentence; k.mu is held.
func (k *kokoroSpeech) add(text string) {
	if text = strings.TrimSpace(text); text != "" && k.err == nil {
		k.queue = append(k.queue, text)
	}
}

func (k *kokoroSpeech) signal() {
	select {
	case k.wake <- struct{}{}:
	default:
	}
}

// fail ends the text with err, the first failure; the other helper's request is cancelled.
func (k *kokoroSpeech) fail(err error) {
	k.mu.Lock()
	if k.err == nil {
		k.err = err
	}
	k.mu.Unlock()
	k.kcancel()
	k.play.Cancel()
	k.signal()
}

// run synthesizes the queued sentences in order until the input is done or the text fails.
func (k *kokoroSpeech) run() {
	defer close(k.done)
	for n := 1; ; {
		k.mu.Lock()
		if k.err != nil || (len(k.queue) == 0 && k.inputDone) {
			k.mu.Unlock()
			return
		}
		if len(k.queue) == 0 {
			k.mu.Unlock()
			select {
			case <-k.wake:
			case <-k.kctx.Done():
				k.fail(k.kctx.Err())
			}
			continue
		}
		sentence := k.queue[0]
		k.queue = k.queue[1:]
		if k.firstSent.IsZero() {
			k.firstSent = time.Now()
		}
		k.mu.Unlock()
		if err := k.say(sentence); err != nil {
			k.fail(fmt.Errorf("sentence %d: %w", n, err))
			return
		}
		n++
	}
}

// say synthesizes one sentence and forwards its frames to play as they arrive.
func (k *kokoroSpeech) say(sentence string) error {
	s, err := k.kokoro.OpenAudio(k.kctx, "synthesize", map[string]any{"text": sentence, "voice": k.voice, "speed": k.speed}, nil)
	if err != nil {
		return kokoroErr(k.kctx, err)
	}
	var sendErr error
	for frame := range s.Audio() {
		if sendErr == nil {
			sendErr = k.play.SendAudio(frame)
		}
	}
	if _, err := s.Wait(); err != nil {
		return kokoroErr(k.kctx, err)
	}
	if sendErr != nil {
		return fmt.Errorf("Apple helper: %w", sendErr)
	}
	return nil
}

// kokoroErr names the Kokoro helper in err, unless the text was stopped.
func kokoroErr(ctx context.Context, err error) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return fmt.Errorf("Kokoro helper: %w", err)
}

// Finish queues what is left, waits until every sentence has played and ends the Kokoro
// helper. After a stop it returns context.Canceled once both helpers' requests have ended.
func (k *kokoroSpeech) Finish() (summarize.Spoken, error) {
	k.mu.Lock()
	k.add(k.buf.String())
	k.buf.Reset()
	k.inputDone = true
	k.mu.Unlock()
	k.signal()
	<-k.done
	defer close(k.finished)

	k.mu.Lock()
	err := k.err
	if err == nil {
		k.inputClosed = true
	}
	k.mu.Unlock()
	if err == nil {
		if cerr := k.play.CloseInput(); cerr != nil {
			err = fmt.Errorf("Apple helper: %w", cerr)
		}
	}
	var res outcome
	if err == nil {
		res = <-k.played
		if res.err != nil {
			err = fmt.Errorf("Apple helper: %w", res.err)
		}
	} else {
		k.play.Cancel()
	}
	k.kcancel()
	k.kokoro.Close()

	k.mu.Lock()
	defer k.mu.Unlock()
	sp := summarize.Spoken{FirstSentence: k.firstSent, FirstAudio: k.firstAudio, Voice: k.voice}
	if err == nil && k.err != nil {
		err = k.err
	}
	if err != nil {
		return sp, fmt.Errorf("speak: %w", err)
	}
	var r struct {
		AudioSeconds float64 `json:"audio_seconds"`
	}
	if err := json.Unmarshal(res.data, &r); err != nil {
		return sp, fmt.Errorf("play result: %w", err)
	}
	sp.AudioSeconds = r.AudioSeconds
	return sp, nil
}
