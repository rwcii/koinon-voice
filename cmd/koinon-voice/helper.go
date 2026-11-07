package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"time"

	"github.com/rwcii/koinon-voice/internal/speech"
	"github.com/rwcii/koinon-voice/internal/summarize"
)

// runHelper is koinon-voice helper. Start the Apple helper, print its hello, and optionally
// transcribe a 16 kHz mono file or speak a text with the engines file's speech engine.
func runHelper(args []string, stdout, stderr io.Writer) int {
	fs := flags("helper")
	engines := fs.String("engines", defaultEngines(), "engines file, for its helper path and [speech]")
	path := fs.String("helper", "", "the Apple helper's path (default: helper in the engines file)")
	transcribe := fs.String("transcribe", "", "transcribe this 16 kHz mono audio file")
	say := fs.String("say", "", "speak this text; Ctrl-C stops")
	out := fs.String("out", "", "with --say, write the speech to this audio file instead of the output device")
	engine := fs.String("engine", "", "with --say, the speech engine, kokoro or apple (default: [speech] engine in the engines file, else kokoro)")
	voice := fs.String("voice", "", "with --say, the engine's voice: a Kokoro voice, or an AVSpeechSynthesisVoice identifier (default: from the engines file)")
	speed := fs.Float64("speed", 0, "with --say, Kokoro's speed (default: [speech] speed in the engines file, else 1.1)")
	timeout := fs.Duration("timeout", 2*time.Minute, "timeout for each request")
	if err := fs.Parse(args); err != nil {
		return badOptions(stderr)
	}
	if fs.NArg() != 0 || *speed < 0 {
		return badOptions(stderr)
	}
	config, err := summarize.LoadConfig(*engines)
	if err != nil && *path == "" {
		fmt.Fprintln(stderr, "koinon-voice helper:", err)
		return 1
	}
	if *path == "" {
		*path = config.Helper
	}
	if *path == "" {
		fmt.Fprintln(stderr, "koinon-voice helper: no Apple helper: set helper in the engines file or pass --helper")
		return 1
	}
	launcher := &summarize.Launcher{Path: *path}
	defer launcher.Close()
	var speaker *speech.Speaker
	if *say != "" {
		// A missing Kokoro helper or model fails before any audio (criterion 5).
		speaker, err = speakerFor(config, launcher, *engine, *voice, *speed)
		if err == nil {
			err = speaker.Check()
		}
		if err != nil {
			fmt.Fprintln(stderr, "koinon-voice helper:", err)
			return 1
		}
		speaker.Output = *out
	}

	// Ctrl-C stops the speech; the command exits once the helpers have stopped.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	h, err := launcher.Get(ctx)
	if err != nil {
		fmt.Fprintln(stderr, "koinon-voice helper:", err)
		return 1
	}
	hello := h.Hello()
	fmt.Fprintf(stdout, "hello: %s %s, capabilities %v, info %s\n", hello.Helper, hello.Version, hello.Capabilities, hello.Info)

	code := 0
	if *transcribe != "" {
		rctx, cancel := context.WithTimeout(ctx, *timeout)
		data, err := h.Call(rctx, "transcribe", map[string]string{"path": *transcribe}, nil)
		cancel()
		if err != nil {
			fmt.Fprintln(stderr, "transcribe:", err)
			code = 1
		} else {
			fmt.Fprintf(stdout, "transcribe: %s\n", data)
		}
	}
	if *say != "" {
		if err := helperSay(ctx, speaker, *say, *timeout, stdout); err != nil {
			if ctx.Err() != nil {
				err = errors.New("stopped")
			}
			fmt.Fprintln(stderr, "speak:", err)
			code = 1
		}
	}
	return code
}

// helperSay speaks one text and prints its time to first audio: from the first sentence's
// request to the Apple helper's first audio, the same for both engines.
func helperSay(ctx context.Context, s *speech.Speaker, text string, timeout time.Duration, stdout io.Writer) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	sp, err := s.Start(ctx)
	if err != nil {
		return err
	}
	sp.Write(text)
	spoken, err := sp.Finish()
	if err != nil {
		return err
	}
	if !spoken.FirstAudio.IsZero() {
		fmt.Fprintf(stdout, "speak: first audio %d ms after the first sentence's request\n", spoken.FirstAudio.Sub(spoken.FirstSentence).Milliseconds())
	}
	fmt.Fprintf(stdout, "speak: %s, voice %s, %.1f s of audio\n", s.Speech.Engine, spoken.Voice, spoken.AudioSeconds)
	return nil
}
