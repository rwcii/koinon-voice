package koinon

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rwcii/koinon-voice/internal/keychain"
	"github.com/rwcii/koinon-voice/internal/signed"
)

const (
	pinA = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	pinB = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
)

func TestLoadHostsSignedRows(t *testing.T) {
	hosts, err := LoadHosts(writeHosts(t, `
[[host]]
label = "studio"
node_pin = "`+pinA+`"
state_dir = "/state"

[[host]]
label = "blackhole"
node_pin = "`+pinB+`"
serve = "https://blackhole.example.ts.net"
`))
	if err != nil {
		t.Fatal(err)
	}
	if len(hosts) != 2 || hosts[0].NodePin != pinA || hosts[0].Serve != "" || hosts[1].NodePin != pinB || hosts[1].Serve == "" {
		t.Fatalf("hosts %+v", hosts)
	}
	if k, args := hosts[0].SignedLocal(); k != "koinon" || strings.Join(args, " ") != "--state-dir /state" {
		t.Errorf("local form %s %v", k, args)
	}

	row := func(fields string) string { return "[[host]]\nlabel = \"a\"\n" + fields }
	for name, tc := range map[string]struct{ content, want string }{
		"empty pin":       {row("node_pin = \"\"\n"), "needs node_pin"},
		"serve no pin":    {row("serve = \"https://h.example\"\n"), "needs node_pin"},
		"empty serve":     {row("node_pin = \"" + pinA + "\"\nserve = \"\"\n"), "serve: "},
		"short pin":       {row("node_pin = \"abc\"\n"), "node_pin: "},
		"upper pin":       {row("node_pin = \"" + strings.ToUpper(pinA) + "\"\n"), "node_pin: "},
		"http":            {row("node_pin = \"" + pinA + "\"\nserve = \"http://h.example\"\n"), "must be https"},
		"path":            {row("node_pin = \"" + pinA + "\"\nserve = \"https://h.example/x\"\n"), "no path"},
		"user":            {row("node_pin = \"" + pinA + "\"\nserve = \"https://u@h.example\"\n"), "no user"},
		"serve and local": {row("node_pin = \"" + pinA + "\"\nserve = \"https://h.example\"\nkoinon = \"/k\"\n"), "no koinon, state_dir or address"},
		"serve and state": {row("node_pin = \"" + pinA + "\"\nserve = \"https://h.example\"\nstate_dir = \"\"\n"), "no koinon, state_dir or address"},
		"same pin":        {row("node_pin = \""+pinA+"\"\n") + "[[host]]\nlabel = \"b\"\nnode_pin = \"" + pinA + "\"\n", "hosts a and b name the same node_pin"},
	} {
		p := writeHosts(t, tc.content)
		_, err := LoadHosts(p)
		if err == nil || !strings.Contains(err.Error(), tc.want) || !strings.Contains(err.Error(), p) {
			t.Errorf("%s: %v", name, err)
		}
	}
}

// keyStore stands in for the Keychain, with its errors.
type keyStore struct {
	items map[string]string
	fail  error
}

func (m *keyStore) Find(_ context.Context, service, account string) (string, error) {
	if m.fail != nil {
		return "", m.fail
	}
	v, ok := m.items[service+"/"+account]
	if !ok {
		return "", fmt.Errorf("%w: %s", keychain.ErrNotFound, service)
	}
	return v, nil
}

func (m *keyStore) Add(_ context.Context, service, account string, data []byte) error {
	if _, ok := m.items[service+"/"+account]; ok {
		return keychain.ErrExists
	}
	m.items[service+"/"+account] = string(data)
	return nil
}

func TestSignedAccess(t *testing.T) {
	ctx := context.Background()
	store := &keyStore{items: map[string]string{}}
	enr := signed.Enrollments{Path: filepath.Join(t.TempDir(), "enrollments.json")}
	a := SignedAccess{Store: store, Enrollments: enr, Client: ClientName}

	if _, err := a.Load(ctx); !errors.Is(err, signed.ErrNoKey) {
		t.Fatalf("no key: %v", err)
	}
	if len(store.items) != 0 {
		t.Fatal("Load created a key")
	}
	k, err := signed.Create(ctx, store, "robert")
	if err != nil {
		t.Fatal(err)
	}
	other, _ := signed.Create(ctx, &keyStore{items: map[string]string{}}, "robert")

	enroll := func(k signed.Key, pin, user, client string) {
		e, err := signed.Enrollment(k.Public(), pin, user, client, 1)
		if err != nil {
			t.Fatal(err)
		}
		if e, err = signed.Sign(k.Private, e); err != nil {
			t.Fatal(err)
		}
		if _, err := enr.Record(e, user, client); err != nil {
			t.Fatal(err)
		}
	}
	pinC := strings.Repeat("c", 64)
	pinD := strings.Repeat("d", 64)
	enroll(k, pinA, "robert", ClientName)
	enroll(other, pinB, "robert", ClientName)
	enroll(k, pinC, "someone", ClientName)
	enroll(k, pinD, "robert", "other-client")

	got, err := a.Load(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Auths) != 1 || got.Auths[pinA] == "" || got.NodeReason(pinA) != "" {
		t.Errorf("auths %v", got.Auths)
	}
	for pin, want := range map[string]string{
		pinB:                    "another key",
		pinC:                    `user "someone"`,
		pinD:                    `client "other-client"`,
		strings.Repeat("e", 64): "not enrolled from this install",
	} {
		if r := got.NodeReason(pin); !strings.Contains(r, want) {
			t.Errorf("%s: %q", pin[:1], r)
		}
	}

	// A malformed record is named, not used.
	b, _ := os.ReadFile(enr.Path)
	bad := strings.Replace(string(b), got.Auths[pinA], "zz", 1)
	os.WriteFile(enr.Path, []byte(bad), 0o600)
	got, _ = a.Load(ctx)
	if !strings.Contains(got.NodeReason(pinA), "authorization ID is malformed") {
		t.Errorf("malformed auth: %q", got.NodeReason(pinA))
	}

	os.WriteFile(enr.Path, []byte("{"), 0o600)
	if _, err := a.Load(ctx); err == nil {
		t.Error("unreadable enrollments loaded")
	}
	store.fail = errors.New("the Keychain is locked")
	if _, err := a.Load(ctx); err == nil || !strings.Contains(err.Error(), "locked") {
		t.Errorf("locked key: %v", err)
	}
}
