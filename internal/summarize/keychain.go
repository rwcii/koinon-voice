package summarize

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

// Keys returns the API key stored for a provider.
type Keys interface {
	Key(ctx context.Context, provider string) (string, error)
}

// Keychain reads keys from the macOS login Keychain with the security command, run as a
// subprocess so the module stays CGO-free (ADR 003, Decision 2). The generic password's
// service is koinon-voice-<provider>.
type Keychain struct {
	// Command is the security binary; empty is "security" on PATH.
	Command string
	// GOOS overrides runtime.GOOS in tests.
	GOOS string
}

// Key runs security find-generic-password and returns the password, never logging it.
func (k Keychain) Key(ctx context.Context, provider string) (string, error) {
	goos := k.GOOS
	if goos == "" {
		goos = runtime.GOOS
	}
	if goos != "darwin" {
		return "", fmt.Errorf("no key for %s: the Keychain is available on macOS only", provider)
	}
	command := k.Command
	if command == "" {
		command = "security"
	}
	service := "koinon-voice-" + provider
	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, command, "find-generic-password", "-s", service, "-w")
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			return "", fmt.Errorf("no key for %s: Keychain item %s not found (security exit %d)", provider, service, exit.ExitCode())
		}
		return "", fmt.Errorf("no key for %s: %w", provider, err)
	}
	key := strings.TrimSpace(stdout.String())
	if key == "" {
		return "", fmt.Errorf("no key for %s: Keychain item %s is empty", provider, service)
	}
	return key, nil
}
