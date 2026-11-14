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
	// File names a keychain file for Find and Add; empty is the default keychain. Native
	// checks use a scratch keychain so that they never touch the login keychain.
	File string
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

// ErrNotFound is the answer of Find when the Keychain has no such item (security exit 44).
var ErrNotFound = errors.New("Keychain item not found")

// ErrExists is the answer of Add when the item is already in the Keychain (security exit 45).
var ErrExists = errors.New("Keychain item already exists")

// Find returns the password of the item with this service and account. A missing item is
// ErrNotFound; a locked Keychain, a refused access or another failure is an error with
// security's exit status, which is never reported as a missing item. Errors carry neither the
// password nor security's output.
func (k Keychain) Find(ctx context.Context, service, account string) (string, error) {
	command, err := k.command()
	if err != nil {
		return "", err
	}
	var stdout bytes.Buffer
	args := []string{"find-generic-password", "-s", service, "-a", account, "-w"}
	if k.File != "" {
		args = append(args, k.File)
	}
	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			if exit.ExitCode() == 44 {
				return "", fmt.Errorf("%w: %s", ErrNotFound, service)
			}
			return "", fmt.Errorf("Keychain item %s could not be read (security exit %d; the Keychain may be locked or access refused)", service, exit.ExitCode())
		}
		return "", fmt.Errorf("Keychain item %s could not be read: the security command did not run", service)
	}
	return strings.TrimSuffix(stdout.String(), "\n"), nil
}

// Add stores data as the password of a new item with this service and account. It never
// replaces an item: one that exists is ErrExists. The command goes to security -i on standard
// input, with the data in hex (-X), so the data is in no process argument; security's output
// is discarded so that no error can carry it.
func (k Keychain) Add(ctx context.Context, service, account string, data []byte) error {
	command, err := k.command()
	if err != nil {
		return err
	}
	if strings.ContainsAny(service+account+k.File, " \t\n\"'\\") {
		return errors.New("Keychain service, account and file must not contain spaces or quotes")
	}
	line := fmt.Sprintf("add-generic-password -s %s -a %s -X %x", service, account, data)
	if k.File != "" {
		line += " " + k.File
	}
	line += "\n"
	cmd := exec.CommandContext(ctx, command, "-i")
	cmd.Stdin = strings.NewReader(line)
	if err := cmd.Run(); err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			if exit.ExitCode() == 45 {
				return fmt.Errorf("%w: %s", ErrExists, service)
			}
			return fmt.Errorf("Keychain item %s could not be added (security exit %d; the Keychain may be locked or access refused)", service, exit.ExitCode())
		}
		return fmt.Errorf("Keychain item %s could not be added: the security command did not run", service)
	}
	return nil
}

func (k Keychain) command() (string, error) {
	goos := k.GOOS
	if goos == "" {
		goos = runtime.GOOS
	}
	if goos != "darwin" {
		return "", errors.New("the Keychain is available on macOS only")
	}
	if k.Command == "" {
		return "security", nil
	}
	return k.Command, nil
}
