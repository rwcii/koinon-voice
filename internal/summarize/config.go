package summarize

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/BurntSushi/toml"
)

// Row is one engine in the engines file (ADR 003, Decision 1).
type Row struct {
	Name    string `toml:"name"`
	Adapter string `toml:"adapter"` // anthropic, openai or apple
	BaseURL string `toml:"base_url"`
	Model   string `toml:"model"`
	// Local is true when no text leaves the machine.
	Local bool `toml:"local"`
	// Provider names who receives the text; an external summary is labeled with it.
	Provider string `toml:"provider"`
	// Key is the Keychain provider name (koinon-voice-<key>); empty sends no key.
	Key         string  `toml:"key"`
	Effort      string  `toml:"effort"`
	MaxTokens   int64   `toml:"max_tokens"`
	InputPrice  float64 `toml:"input_price"`
	OutputPrice float64 `toml:"output_price"`
	// DenyDataCollection sends OpenRouter's provider.data_collection "deny" (openai adapter).
	DenyDataCollection bool `toml:"deny_data_collection"`
	// Note is recorded with the row's results.
	Note string `toml:"note"`
}

// Config is an engines file.
type Config struct {
	// Helper is the Apple helper's path, for the apple adapter and for speech.
	Helper string `toml:"helper"`
	// Voice is the AVSpeechSynthesisVoice identifier that speaks summaries with engine =
	// "apple"; empty is the system's en-US voice.
	Voice  string       `toml:"voice"`
	Speech SpeechConfig `toml:"speech"`
	Engine []Row        `toml:"engine"`
}

// The speech engines (ADR 002, Decision 2).
const (
	SpeechKokoro = "kokoro"
	SpeechApple  = "apple"
)

// Kokoro's defaults: the voice and speed the maintainer rated (ADR 002, Decision 2).
const (
	KokoroVoice = "af_heart"
	KokoroSpeed = 1.1
)

// SpeechConfig is the engines file's [speech] table: the engine that speaks, its voice and,
// for Kokoro, its speed. Empty fields take Speaking's defaults.
type SpeechConfig struct {
	Engine string  `toml:"engine"`
	Voice  string  `toml:"voice"`
	Speed  float64 `toml:"speed"`
}

// Speaking is the speech engine, voice and speed with the defaults filled in: Kokoro, voice
// af_heart at speed 1.1; for engine = "apple", the voice is [speech]'s or else the top-level
// voice.
func (c Config) Speaking() SpeechConfig {
	s := c.Speech
	if s.Engine == "" {
		s.Engine = SpeechKokoro
	}
	switch s.Engine {
	case SpeechKokoro:
		if s.Voice == "" {
			s.Voice = KokoroVoice
		}
		if s.Speed == 0 {
			s.Speed = KokoroSpeed
		}
	case SpeechApple:
		if s.Voice == "" {
			s.Voice = c.Voice
		}
	}
	return s
}

// LoadEngines reads and checks an engines file's rows.
func LoadEngines(path string) ([]Row, error) {
	c, err := LoadConfig(path)
	return c.Engine, err
}

// LoadConfig reads and checks an engines file.
func LoadConfig(path string) (Config, error) {
	data, err := readLimited(path)
	if err != nil {
		return Config{}, err
	}
	var f Config
	meta, err := toml.Decode(string(data), &f)
	if err != nil {
		return Config{}, fmt.Errorf("%s: %w", path, err)
	}
	if undecoded := meta.Undecoded(); len(undecoded) > 0 {
		return Config{}, fmt.Errorf("%s: unknown field %s", path, undecoded[0])
	}
	switch {
	case f.Speech.Engine != "" && f.Speech.Engine != SpeechKokoro && f.Speech.Engine != SpeechApple:
		return Config{}, fmt.Errorf("%s: [speech] engine %q is not kokoro or apple", path, f.Speech.Engine)
	case f.Speech.Speed < 0:
		return Config{}, fmt.Errorf("%s: [speech] speed %g is below 0", path, f.Speech.Speed)
	}
	if len(f.Engine) == 0 {
		return Config{}, fmt.Errorf("%s: no [[engine]] rows", path)
	}
	seen := map[string]bool{}
	for i, r := range f.Engine {
		switch {
		case r.Name == "":
			return Config{}, fmt.Errorf("%s: engine %d has no name", path, i+1)
		case seen[r.Name]:
			return Config{}, fmt.Errorf("%s: engine %s is listed twice", path, r.Name)
		case r.Model == "":
			return Config{}, fmt.Errorf("%s: engine %s has no model", path, r.Name)
		case r.Adapter != "anthropic" && r.Adapter != "openai" && r.Adapter != "apple":
			return Config{}, fmt.Errorf("%s: engine %s: adapter %q is not anthropic, openai or apple", path, r.Name, r.Adapter)
		case r.Adapter == "apple" && !r.Local:
			return Config{}, fmt.Errorf("%s: engine %s: Apple's model runs on this Mac; set local = true", path, r.Name)
		case r.Adapter == "openai" && r.BaseURL == "":
			return Config{}, fmt.Errorf("%s: engine %s: the openai adapter needs base_url", path, r.Name)
		case !r.Local && r.Provider == "":
			return Config{}, fmt.Errorf("%s: engine %s is external and names no provider", path, r.Name)
		}
		seen[r.Name] = true
	}
	return f, nil
}

// Env is what Build needs beyond the row.
type Env struct {
	Keys   Keys
	HTTP   *http.Client
	Helper *Launcher
}

// Build makes the row's engine, reading its key. A missing key fails only this engine.
func Build(ctx context.Context, r Row, env Env) (Engine, error) {
	keys, client := env.Keys, env.HTTP
	if r.Adapter == "apple" {
		if env.Helper == nil || env.Helper.Path == "" {
			return nil, errors.New("no Apple helper: set helper in the engines file or pass --helper")
		}
		return &Apple{Helper: env.Helper}, nil
	}
	key := ""
	if r.Key != "" {
		var err error
		if key, err = keys.Key(ctx, r.Key); err != nil {
			return nil, err
		}
	}
	price := Price{Input: r.InputPrice, Output: r.OutputPrice}
	switch r.Adapter {
	case "anthropic":
		return &Anthropic{Model: r.Model, BaseURL: r.BaseURL, Key: key, Effort: r.Effort, MaxTokens: r.MaxTokens, Price: price, HTTP: client}, nil
	default:
		return &OpenAI{Model: r.Model, BaseURL: r.BaseURL, Key: key, MaxTokens: r.MaxTokens, Price: price, DenyDataCollection: r.DenyDataCollection, HTTP: client}, nil
	}
}
