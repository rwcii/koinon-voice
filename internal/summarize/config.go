package summarize

import (
	"context"
	"fmt"
	"net/http"

	"github.com/BurntSushi/toml"
)

// Row is one engine in the engines file (ADR 003, Decision 1).
type Row struct {
	Name    string `toml:"name"`
	Adapter string `toml:"adapter"` // anthropic or openai
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

type enginesFile struct {
	Engine []Row `toml:"engine"`
}

// LoadEngines reads and checks an engines file.
func LoadEngines(path string) ([]Row, error) {
	data, err := readLimited(path)
	if err != nil {
		return nil, err
	}
	var f enginesFile
	meta, err := toml.Decode(string(data), &f)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if undecoded := meta.Undecoded(); len(undecoded) > 0 {
		return nil, fmt.Errorf("%s: unknown field %s", path, undecoded[0])
	}
	if len(f.Engine) == 0 {
		return nil, fmt.Errorf("%s: no [[engine]] rows", path)
	}
	seen := map[string]bool{}
	for i, r := range f.Engine {
		switch {
		case r.Name == "":
			return nil, fmt.Errorf("%s: engine %d has no name", path, i+1)
		case seen[r.Name]:
			return nil, fmt.Errorf("%s: engine %s is listed twice", path, r.Name)
		case r.Model == "":
			return nil, fmt.Errorf("%s: engine %s has no model", path, r.Name)
		case r.Adapter != "anthropic" && r.Adapter != "openai":
			return nil, fmt.Errorf("%s: engine %s: adapter %q is not anthropic or openai", path, r.Name, r.Adapter)
		case r.Adapter == "openai" && r.BaseURL == "":
			return nil, fmt.Errorf("%s: engine %s: the openai adapter needs base_url", path, r.Name)
		case !r.Local && r.Provider == "":
			return nil, fmt.Errorf("%s: engine %s is external and names no provider", path, r.Name)
		}
		seen[r.Name] = true
	}
	return f.Engine, nil
}

// Build makes the row's engine, reading its key. A missing key fails only this engine.
func Build(ctx context.Context, r Row, keys Keys, client *http.Client) (Engine, error) {
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
