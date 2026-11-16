package koinon

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeHosts(t *testing.T, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "hosts.toml")
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestLoadHosts(t *testing.T) {
	pin := "node_pin = \"" + pinA + "\"\n"
	for name, tc := range map[string]struct{ content, want string }{
		"unknown field": {"[[host]]\nlabel = \"a\"\n" + pin + "port = 1\n", "unknown field host.port"},
		"duplicate":     {"[[host]]\nlabel = \"a\"\n" + pin + "[[host]]\nlabel = \"a\"\nnode_pin = \"" + pinB + "\"\n", "host a is listed twice"},
		"no hosts":      {"", "no [[host]] rows"},
		"no label":      {"[[host]]\n" + pin, "host 1 has no label"},
		"no node_pin":   {"[[host]]\nlabel = \"a\"\n", "host a: a signed node needs node_pin, read on the node with `koinon link status`"},
		"local only":    {"[[host]]\nlabel = \"a\"\nkoinon = \"/k\"\nstate_dir = \"/s\"\n", "host a: a signed node needs node_pin"},
		"syntax":        {"[[host]\n", "hosts.toml"},
	} {
		p := writeHosts(t, tc.content)
		_, err := LoadHosts(p)
		if err == nil || !strings.Contains(err.Error(), tc.want) || !strings.Contains(err.Error(), p) {
			t.Errorf("%s: %v", name, err)
		}
	}
	if _, err := LoadHosts(filepath.Join(t.TempDir(), "absent.toml")); !os.IsNotExist(err) {
		t.Errorf("missing file: %v", err)
	}
}

// A row of the removed maintainer-secret connection is refused, with what to write instead,
// whether or not it also has node_pin.
func TestLoadHostsRefusesSSHAndSecretRows(t *testing.T) {
	for name, fields := range map[string]string{
		"secret":            "secret = \"maintainer-local\"\n",
		"ssh":               "ssh = \"maintainer@devbox\"\nkoinon = \"/home/maintainer/.local/bin/koinon\"\nsecret = \"maintainer-devbox\"\n",
		"empty secret":      "secret = \"\"\n",
		"ssh only":          "ssh = \"devbox\"\n",
		"secret and pin":    "node_pin = \"" + pinA + "\"\nsecret = \"s\"\n",
		"ssh and pin":       "node_pin = \"" + pinA + "\"\nssh = \"devbox\"\n",
		"empty pin, secret": "node_pin = \"\"\nsecret = \"s\"\n",
	} {
		p := writeHosts(t, "[[host]]\nlabel = \"a\"\n"+fields)
		_, err := LoadHosts(p)
		if err == nil || !strings.Contains(err.Error(), p+": host a: ssh and secret rows were removed; use a signed node row") ||
			!strings.Contains(err.Error(), "`koinon link status`") || !strings.Contains(err.Error(), `README "Connect to Koinon"`) {
			t.Errorf("%s: %v", name, err)
		}
	}
}
