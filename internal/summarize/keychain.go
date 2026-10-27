package summarize

import (
	"context"
	"fmt"

	"github.com/rwcii/koinon-voice/internal/keychain"
)

// Keys returns the API key stored for a provider.
type Keys interface {
	Key(ctx context.Context, provider string) (string, error)
}

// Keychain reads keys from the macOS login Keychain (ADR 003, Decision 2). The generic
// password's service is koinon-voice-<provider>.
type Keychain struct {
	// Command is the security binary; empty is "security" on PATH.
	Command string
	// GOOS overrides runtime.GOOS in tests.
	GOOS string
}

// Key returns the provider's key, never logging it.
func (k Keychain) Key(ctx context.Context, provider string) (string, error) {
	key, err := keychain.Keychain{Command: k.Command, GOOS: k.GOOS}.Get(ctx, provider)
	if err != nil {
		return "", fmt.Errorf("no key for %s: %w", provider, err)
	}
	return key, nil
}
