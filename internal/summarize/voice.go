package summarize

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/rwcii/koinon-voice/internal/sidecar"
)

// Speaker starts spoken summaries; Voice is the real one.
type Speaker interface {
	Start(ctx context.Context) (Speech, error)
}

// Speech is one summary being spoken: Write adds streamed text, Finish waits for playback.
type Speech interface {
	Write(delta string)
	Finish() (Spoken, error)
}

// Say speaks one text and waits until it has played.
func Say(ctx context.Context, s Speaker, text string) error {
	sp, err := s.Start(ctx)
	if err != nil {
		return err
	}
	sp.Write(text)
	_, err = sp.Finish()
	return err
}

// Voice speaks summaries through the Apple helper's speak.
type Voice struct {
	Helper *Launcher
	// Voice is an AVSpeechSynthesisVoice identifier; empty is the system's en-US voice.
	Voice string
}

// Spoken is one utterance's timing.
type Spoken struct {
	// FirstSentence is when the first complete sentence went to the helper, and FirstAudio
	// when the helper reported its first sample at the output; both are zero when absent.
	FirstSentence time.Time
	FirstAudio    time.Time
	Voice         string
	AudioSeconds  float64
}

// Utterance streams one summary into speech: each complete sentence goes to the helper as
// soon as it arrives, so synthesis does not wait for the whole summary.
type Utterance struct {
	s *sidecar.Stream

	mu         sync.Mutex
	buf        strings.Builder
	firstSent  time.Time
	firstAudio time.Time
	err        error
}

// Start opens an utterance. ctx bounds the whole utterance, playback included.
func (v *Voice) Start(ctx context.Context) (Speech, error) {
	h, err := v.Helper.Get(ctx)
	if err != nil {
		return nil, err
	}
	if !h.Hello().Has("speak") {
		return nil, errors.New("the helper does not offer speak")
	}
	u := &Utterance{}
	params := map[string]string{}
	if v.Voice != "" {
		params["voice"] = v.Voice
	}
	u.s, err = h.Open(ctx, "speak", params, func(e sidecar.Event) {
		if e.Name == "first_audio" {
			u.mu.Lock()
			if u.firstAudio.IsZero() {
				u.firstAudio = e.At
			}
			u.mu.Unlock()
		}
	})
	if err != nil {
		return nil, err
	}
	return u, nil
}

// Write adds streamed text and sends each sentence it completes.
func (u *Utterance) Write(delta string) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.buf.WriteString(delta)
	for {
		text := u.buf.String()
		loc := sentenceEnd.FindStringIndex(text)
		if loc == nil {
			return
		}
		u.send(text[:loc[1]])
		u.buf.Reset()
		u.buf.WriteString(text[loc[1]:])
	}
}

func (u *Utterance) send(text string) {
	text = strings.TrimSpace(text)
	if text == "" || u.err != nil {
		return
	}
	if u.firstSent.IsZero() {
		u.firstSent = time.Now()
	}
	u.err = u.s.Send(map[string]string{"text": text})
}

// Finish sends what is left, ends the input and waits until the speech has played.
func (u *Utterance) Finish() (Spoken, error) {
	u.mu.Lock()
	u.send(u.buf.String())
	u.buf.Reset()
	sendErr := u.err
	u.mu.Unlock()
	if err := u.s.CloseInput(); err != nil && sendErr == nil {
		sendErr = err
	}
	data, err := u.s.Wait()
	u.mu.Lock()
	defer u.mu.Unlock()
	sp := Spoken{FirstSentence: u.firstSent, FirstAudio: u.firstAudio}
	if err == nil {
		err = sendErr
	}
	if err != nil {
		return sp, fmt.Errorf("speak: %w", err)
	}
	var r struct {
		Voice        string
		AudioSeconds float64 `json:"audio_seconds"`
	}
	if err := json.Unmarshal(data, &r); err != nil {
		return sp, fmt.Errorf("speak result: %w", err)
	}
	sp.Voice, sp.AudioSeconds = r.Voice, r.AudioSeconds
	return sp, nil
}
