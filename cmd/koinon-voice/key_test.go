package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rwcii/koinon-voice/internal/keychain"
	"github.com/rwcii/koinon-voice/internal/signed"
)

// memoryStore stands in for the Keychain, with the same errors.
type memoryStore struct {
	items map[string]string
	fail  error
}

func (m *memoryStore) Find(_ context.Context, service, account string) (string, error) {
	if m.fail != nil {
		return "", m.fail
	}
	v, ok := m.items[service+"/"+account]
	if !ok {
		return "", fmt.Errorf("%w: %s", keychain.ErrNotFound, service)
	}
	return v, nil
}

func (m *memoryStore) Add(_ context.Context, service, account string, data []byte) error {
	if m.fail != nil {
		return m.fail
	}
	if _, ok := m.items[service+"/"+account]; ok {
		return fmt.Errorf("%w: %s", keychain.ErrExists, service)
	}
	m.items[service+"/"+account] = string(data)
	return nil
}

func useStore(t *testing.T) *memoryStore {
	t.Helper()
	m := &memoryStore{items: map[string]string{}}
	oldStore, oldNow := keyStore, keyNow
	keyStore, keyNow = m, func() time.Time { return time.Unix(1794699000, 0) }
	t.Cleanup(func() { keyStore, keyNow = oldStore, oldNow })
	return m
}

func runOut(args ...string) (int, string, string) {
	var stdout, stderr bytes.Buffer
	code := run(args, strings.NewReader(""), &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

const testPin = "a0a1a2a3a4a5a6a7a8a9aaabacadaeafb0b1b2b3b4b5b6b7b8b9babbbcbdbebf"

func TestKeyCreateShowEnroll(t *testing.T) {
	m := useStore(t)
	if code, _, stderr := runOut("key", "show"); code != 1 || !strings.Contains(stderr, "key create") {
		t.Fatalf("show before create: %d %s", code, stderr)
	}
	code, created, stderr := runOut("key", "create", "--user", "robert")
	if code != 0 {
		t.Fatalf("create: %s", stderr)
	}
	var shown struct{ KeyID, PublicKey, User string }
	_ = json.Unmarshal([]byte(strings.NewReplacer("key_id", "KeyID", "public_key", "PublicKey").Replace(created)), &shown)
	if shown.User != "robert" || !strings.HasPrefix(shown.KeyID, "SHA256:") {
		t.Errorf("created %s", created)
	}
	seed := seedOf(t, m)
	for _, out := range []string{created, stderr} {
		if strings.Contains(out, seed) {
			t.Error("the private seed was printed")
		}
	}

	if code, again, _ := runOut("key", "show"); code != 0 || again != created {
		t.Errorf("show %d %s, want %s", code, again, created)
	}
	if code, _, stderr := runOut("key", "create", "--user", "other"); code != 1 || !strings.Contains(stderr, "already has") {
		t.Errorf("second create: %d %s", code, stderr)
	}
	if seedOf(t, m) != seed {
		t.Error("a second create replaced the key")
	}

	code, statement, stderr := runOut("key", "enroll", "--target", testPin)
	if code != 0 {
		t.Fatalf("enroll: %s", stderr)
	}
	if strings.Contains(statement, seed) {
		t.Error("the statement holds the private seed")
	}
	var e signed.Envelope
	if err := json.Unmarshal([]byte(statement), &e); err != nil {
		t.Fatal(err)
	}
	pub, _ := base64.StdEncoding.DecodeString(e.Args["public_key"].(string))
	if err := signed.Verify(ed25519.PublicKey(pub), e); err != nil {
		t.Errorf("statement: %v", err)
	}
	if e.Op != "enroll" || e.Target != testPin || e.Args["user"] != "robert" || e.Args["client"] != "koinon-voice" || e.Created != 1794699000 || e.Request != "" {
		t.Errorf("statement %s", statement)
	}

	out := filepath.Join(t.TempDir(), "statement.json")
	if code, msg, stderr := runOut("key", "enroll", "--target", testPin, "--out", out); code != 0 || !strings.Contains(msg, "koinon maintainer key add --statement") {
		t.Fatalf("enroll --out: %d %s %s", code, msg, stderr)
	}
	if b, _ := os.ReadFile(out); string(b) != statement {
		t.Errorf("file %s", b)
	}
	if info, _ := os.Stat(out); info.Mode().Perm() != 0o600 {
		t.Errorf("mode %v", info.Mode())
	}
	if code, _, stderr := runOut("key", "enroll", "--target", testPin, "--out", out); code != 1 || !strings.Contains(stderr, "not written") {
		t.Errorf("enroll over a file: %d %s", code, stderr)
	}
}

func TestKeyErrors(t *testing.T) {
	m := useStore(t)
	for _, args := range [][]string{{"key"}, {"key", "create"}, {"key", "rotate"}, {"key", "show", "x"}, {"key", "enroll"}} {
		if code, _, _ := runOut(args...); code != 1 {
			t.Errorf("%v: %d", args, code)
		}
	}
	if code, _, stderr := runOut("key", "create", "--user", "Robert"); code != 1 || !strings.Contains(stderr, "user name") {
		t.Errorf("bad user: %s", stderr)
	}
	if code, _, _ := runOut("key", "create", "--user", "robert"); code != 0 {
		t.Fatal("create")
	}
	if code, _, stderr := runOut("key", "enroll", "--target", "ABC"); code != 1 || !strings.Contains(stderr, "pin") {
		t.Errorf("bad pin: %s", stderr)
	}
	if code, _, stderr := runOut("key", "enroll", "--target", testPin, "--client", "Bad"); code != 1 || !strings.Contains(stderr, "client label") {
		t.Errorf("bad client: %s", stderr)
	}
	for name, payload := range map[string]string{
		"not json":    "SENTINEL",
		"version":     `{"v":2,"user":"robert","seed":"AAAA"}`,
		"short seed":  `{"v":1,"user":"robert","seed":"U0VOVElORUw="}`,
		"bad user":    `{"v":1,"user":"","seed":"` + strings.Repeat("A", 43) + `="}`,
		"extra field": `{"v":1,"user":"robert","seed":"` + strings.Repeat("A", 43) + `=","x":1}`,
	} {
		m.items[signed.KeychainService+"/"+signed.KeychainAccount] = payload
		code, stdout, stderr := runOut("key", "show")
		if code != 1 || strings.Contains(stdout+stderr, "SENTINEL") || strings.Contains(stdout+stderr, "U0VOVElORUw") {
			t.Errorf("%s: %d %s %s", name, code, stdout, stderr)
		}
	}
	m.fail = fmt.Errorf("Keychain item %s could not be read (security exit 36)", signed.KeychainService)
	if code, _, stderr := runOut("key", "show"); code != 1 || !strings.Contains(stderr, "exit 36") || strings.Contains(stderr, "key create") {
		t.Errorf("locked: %s", stderr)
	}
}

func seedOf(t *testing.T, m *memoryStore) string {
	t.Helper()
	var p struct{ Seed string }
	if err := json.Unmarshal([]byte(m.items[signed.KeychainService+"/"+signed.KeychainAccount]), &p); err != nil || p.Seed == "" {
		t.Fatalf("stored item: %v", err)
	}
	return p.Seed
}
