package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/rwcii/koinon-voice/internal/sidecar"
	"github.com/rwcii/koinon-voice/internal/summarize"
)

// runHelper is koinon-voice helper. Start the Apple helper, print its hello, and optionally
// transcribe a 16 kHz mono file or speak a text through it.
func runHelper(args []string, stdout, stderr io.Writer) int {
	fs := flags("helper")
	engines := fs.String("engines", defaultEngines(), "engines file, for its helper path")
	path := fs.String("helper", "", "the Apple helper's path (default: helper in the engines file)")
	transcribe := fs.String("transcribe", "", "transcribe this 16 kHz mono audio file")
	say := fs.String("say", "", "speak this text")
	out := fs.String("out", "", "with --say, write the speech to this audio file instead of the output device")
	voice := fs.String("voice", "", "with --say, an AVSpeechSynthesisVoice identifier (default: voice in the engines file)")
	timeout := fs.Duration("timeout", 2*time.Minute, "timeout for each request")
	if err := fs.Parse(args); err != nil {
		return badOptions(stderr)
	}
	if fs.NArg() != 0 {
		return badOptions(stderr)
	}
	if *path == "" || *voice == "" {
		config, err := summarize.LoadConfig(*engines)
		switch {
		case err == nil:
			if *path == "" {
				*path = config.Helper
			}
			if *voice == "" {
				*voice = config.Voice
			}
		case *path == "":
			fmt.Fprintln(stderr, "koinon-voice helper:", err)
			return 1
		}
	}
	if *path == "" {
		fmt.Fprintln(stderr, "koinon-voice helper: no Apple helper: set helper in the engines file or pass --helper")
		return 1
	}

	ctx := context.Background()
	h, err := sidecar.Start(ctx, *path)
	if err != nil {
		fmt.Fprintln(stderr, "koinon-voice helper:", err)
		return 1
	}
	defer h.Close()
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
		if err := helperSay(ctx, h, *say, *out, *voice, *timeout, stdout); err != nil {
			fmt.Fprintln(stderr, "speak:", err)
			code = 1
		}
	}
	return code
}

func helperSay(ctx context.Context, h *sidecar.Helper, text, out, voice string, timeout time.Duration, stdout io.Writer) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	params := map[string]string{}
	if out != "" {
		params["output"] = out
	}
	if voice != "" {
		params["voice"] = voice
	}
	start := time.Now()
	var first json.RawMessage
	var firstAt time.Time
	s, err := h.Open(ctx, "speak", params, func(e sidecar.Event) {
		if e.Name == "first_audio" && firstAt.IsZero() {
			first, firstAt = e.Data, e.At
		}
	})
	if err != nil {
		return err
	}
	if err := s.Send(map[string]string{"text": text}); err != nil {
		return err
	}
	if err := s.CloseInput(); err != nil {
		return err
	}
	data, err := s.Wait()
	if err != nil {
		return err
	}
	if !firstAt.IsZero() {
		fmt.Fprintf(stdout, "speak: first audio %d ms after the request (helper: %s)\n", firstAt.Sub(start).Milliseconds(), first)
	}
	fmt.Fprintf(stdout, "speak: %s\n", data)
	return nil
}
