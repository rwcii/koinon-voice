package keychain

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeSecurity writes a stand-in for macOS security that knows two items.
func fakeSecurity(t *testing.T) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "security")
	script := `#!/bin/sh
[ "$1" = find-generic-password ] && [ "$2" = -s ] && [ "$4" = -w ] || exit 2
case "$3" in
koinon-voice-anthropic) echo "sk-test-anthropic" ;;
koinon-voice-blank) echo "" ;;
*) echo "security: SecKeychainSearchCopyNext: The specified item could not be found in the keychain." >&2; exit 44 ;;
esac
`
	if err := os.WriteFile(p, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestGet(t *testing.T) {
	k := Keychain{Command: fakeSecurity(t), GOOS: "darwin"}
	ctx := context.Background()
	if s, err := k.Get(ctx, "anthropic"); err != nil || s != "sk-test-anthropic" {
		t.Errorf("secret %q %v", s, err)
	}
	_, err := k.Get(ctx, "maintainer-local")
	if err == nil || !strings.Contains(err.Error(), "koinon-voice-maintainer-local not found") {
		t.Errorf("missing: %v", err)
	}
	if _, err := k.Get(ctx, "blank"); err == nil || !strings.Contains(err.Error(), "koinon-voice-blank is empty") {
		t.Errorf("blank: %v", err)
	}
	if _, err := (Keychain{GOOS: "linux"}).Get(ctx, "anthropic"); err == nil || !strings.Contains(err.Error(), "macOS only") {
		t.Errorf("linux: %v", err)
	}
	if _, err := (Keychain{Command: filepath.Join(t.TempDir(), "none"), GOOS: "darwin"}).Get(ctx, "anthropic"); err == nil {
		t.Error("a missing security command gave a secret")
	}
}
