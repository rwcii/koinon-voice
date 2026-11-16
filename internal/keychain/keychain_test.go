package keychain

import (
	"context"
	"errors"
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

// fakeStore writes a stand-in for security that keeps items in a directory, takes
// add-generic-password through -i on standard input (exit 45 for an existing item), and logs
// its arguments. A file named "locked" makes every call exit 36.
func fakeStore(t *testing.T) (command, dir string) {
	t.Helper()
	dir = t.TempDir()
	command = filepath.Join(dir, "security")
	script := `#!/bin/sh
d=$(dirname "$0")
echo "$@" >> "$d/args"
[ -f "$d/locked" ] && { echo "locked" >&2; exit 36; }
if [ "$1" = -i ]; then
	read -r cmd s svc a acct x hex
	[ "$cmd $s $a $x" = "add-generic-password -s -a -X" ] || exit 2
	[ -f "$d/item-$svc-$acct" ] && { echo "already exists $hex" >&2; exit 45; }
	printf '%s' "$hex" | xxd -r -p > "$d/item-$svc-$acct"
	exit 0
fi
[ "$1" = find-generic-password ] && [ "$2" = -s ] && [ "$4" = -a ] && [ "$6" = -w ] || exit 2
[ -f "$d/item-$3-$5" ] || { echo "could not be found" >&2; exit 44; }
cat "$d/item-$3-$5"; echo
`
	if err := os.WriteFile(command, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	return command, dir
}

func TestAddAndFind(t *testing.T) {
	command, dir := fakeStore(t)
	k := Keychain{Command: command, GOOS: "darwin"}
	ctx := context.Background()
	const sentinel = `{"seed":"SENTINEL-PRIVATE"}`
	if _, err := k.Find(ctx, "koinon-voice-x", "a"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing: %v", err)
	}
	if err := k.Add(ctx, "koinon-voice-x", "a", []byte(sentinel)); err != nil {
		t.Fatal(err)
	}
	if s, err := k.Find(ctx, "koinon-voice-x", "a"); err != nil || s != sentinel {
		t.Errorf("found %q %v", s, err)
	}
	err := k.Add(ctx, "koinon-voice-x", "a", []byte("other"))
	if !errors.Is(err, ErrExists) || strings.Contains(err.Error(), "SENTINEL") {
		t.Errorf("duplicate: %v", err)
	}
	if s, _ := k.Find(ctx, "koinon-voice-x", "a"); s != sentinel {
		t.Errorf("a duplicate add replaced the item: %q", s)
	}
	args, _ := os.ReadFile(filepath.Join(dir, "args"))
	if strings.Contains(string(args), "SENTINEL") || strings.Contains(string(args), "53454e54494e454c") {
		t.Errorf("the data was in an argument: %s", args)
	}
	if err := os.WriteFile(filepath.Join(dir, "locked"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	_, err = k.Find(ctx, "koinon-voice-x", "a")
	if err == nil || errors.Is(err, ErrNotFound) || !strings.Contains(err.Error(), "exit 36") {
		t.Errorf("locked find: %v", err)
	}
	err = k.Add(ctx, "koinon-voice-y", "a", []byte(sentinel))
	if err == nil || errors.Is(err, ErrExists) || strings.Contains(err.Error(), "SENTINEL") {
		t.Errorf("locked add: %v", err)
	}
	if err := k.Add(ctx, "bad service", "a", nil); err == nil {
		t.Error("a service with a space was accepted")
	}
	if _, err := (Keychain{GOOS: "linux"}).Find(ctx, "x", "a"); err == nil || !strings.Contains(err.Error(), "macOS only") {
		t.Errorf("linux: %v", err)
	}
	if err := (Keychain{Command: filepath.Join(dir, "none"), GOOS: "darwin"}).Add(ctx, "x", "a", nil); err == nil || errors.Is(err, ErrExists) {
		t.Errorf("missing command: %v", err)
	}
}
