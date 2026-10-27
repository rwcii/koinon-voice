// Package keychain reads secrets from the macOS login Keychain: provider API keys
// (ADR 003, Decision 2) and each Koinon daemon's maintainer secret (ADR 001, Decision 3).
package keychain

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

// Prefix begins the service name of every Koinon Voice Keychain item.
const Prefix = "koinon-voice-"

// Keychain reads generic passwords with the security command, run as a subprocess so the
// module stays CGO-free. The item's service is koinon-voice-<name>.
type Keychain struct {
	// Command is the security binary; empty is "security" on PATH.
	Command string
	// GOOS overrides runtime.GOOS in tests.
	GOOS string
}

// Get runs security find-generic-password and returns the password, never logging it. Its
// errors name the service and never carry the password.
func (k Keychain) Get(ctx context.Context, name string) (string, error) {
	goos := k.GOOS
	if goos == "" {
		goos = runtime.GOOS
	}
	if goos != "darwin" {
		return "", errors.New("the Keychain is available on macOS only")
	}
	command := k.Command
	if command == "" {
		command = "security"
	}
	service := Prefix + name
	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, command, "find-generic-password", "-s", service, "-w")
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			return "", fmt.Errorf("Keychain item %s not found (security exit %d)", service, exit.ExitCode())
		}
		return "", err
	}
	secret := strings.TrimSpace(stdout.String())
	if secret == "" {
		return "", fmt.Errorf("Keychain item %s is empty", service)
	}
	return secret, nil
}
