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
	hosts, err := LoadHosts(writeHosts(t, `
[[host]]
label = "local"
secret = "maintainer-local"

[[host]]
label = "devbox"
ssh = "maintainer@devbox"
koinon = "/home/maintainer/.local/bin/koinon"
state_dir = "/home/maintainer/.local/state/koinon/go"
secret = "maintainer-devbox"
`))
	if err != nil {
		t.Fatal(err)
	}
	if len(hosts) != 2 || hosts[0].Remote() || !hosts[1].Remote() || hosts[1].Secret != "maintainer-devbox" {
		t.Errorf("hosts %+v", hosts)
	}

	for name, tc := range map[string]struct{ content, want string }{
		"unknown field": {"[[host]]\nlabel = \"a\"\nsecret = \"s\"\nport = 1\n", "unknown field host.port"},
		"duplicate":     {"[[host]]\nlabel = \"a\"\nsecret = \"s\"\n[[host]]\nlabel = \"a\"\nsecret = \"s\"\n", "host a is listed twice"},
		"no hosts":      {"", "no [[host]] rows"},
		"no label":      {"[[host]]\nsecret = \"s\"\n", "host 1 has no label"},
		"no secret":     {"[[host]]\nlabel = \"a\"\n", "host a has no secret"},
		"remote path":   {"[[host]]\nlabel = \"a\"\nssh = \"devbox\"\nkoinon = \"koinon\"\nsecret = \"s\"\n", "host a: koinon must be the full path"},
		"ssh option":    {"[[host]]\nlabel = \"a\"\nssh = \"-oProxyCommand=x\"\nkoinon = \"/k\"\nsecret = \"s\"\n", "begins with -"},
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

func TestHostCommand(t *testing.T) {
	name, args := Host{Label: "local", StateDir: "/tmp/s", Address: "127.0.0.1:47671"}.Command("")
	if name != "koinon" || strings.Join(args, " ") != "mcp --maintainer --client koinon-voice --secret-stdin --state-dir /tmp/s --address 127.0.0.1:47671" {
		t.Errorf("local %s %q", name, args)
	}
	name, args = Host{Label: "r", SSH: "devbox", Koinon: "/opt/it's here/koinon"}.Command("")
	if name != "ssh" || strings.Join(args, " ") != `-o BatchMode=yes -T -- devbox '/opt/it'\''s here/koinon' 'mcp' '--maintainer' '--client' 'koinon-voice' '--secret-stdin'` {
		t.Errorf("remote %s %q", name, args)
	}
}
