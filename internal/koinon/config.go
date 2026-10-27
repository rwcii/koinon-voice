package koinon

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/BurntSushi/toml"
)

// ClientName is the client that Koinon records with each action, as mcp:koinon-voice
// (ADR 001, Decision 3).
const ClientName = "koinon-voice"

// Host is one Koinon daemon in hosts.toml.
type Host struct {
	// Label names the host in every view.
	Label string `toml:"label"`
	// SSH is the ssh destination of a remote host; empty is this machine.
	SSH string `toml:"ssh"`
	// Koinon is the koinon binary: a full path on a remote host, where a non-interactive
	// shell may not have it on PATH; empty is "koinon" on this machine's PATH.
	Koinon string `toml:"koinon"`
	// StateDir and Address select the daemon when it is not the default one.
	StateDir string `toml:"state_dir"`
	Address  string `toml:"address"`
	// Secret is the Keychain name of the daemon's maintainer secret: the service is
	// koinon-voice-<secret>.
	Secret string `toml:"secret"`
}

// Remote reports whether the host is reached through ssh.
func (h Host) Remote() bool { return h.SSH != "" }

// Command returns the program and arguments that start the host's maintainer mode. ssh is
// the ssh binary for a remote host. The secret is never an argument: it is the first line
// of standard input (Koinon PDR 009, Decision 3).
func (h Host) Command(ssh string) (string, []string) {
	koinon := h.Koinon
	if koinon == "" {
		koinon = "koinon"
	}
	args := []string{"mcp", "--maintainer", "--client", ClientName, "--secret-stdin"}
	if h.StateDir != "" {
		args = append(args, "--state-dir", h.StateDir)
	}
	if h.Address != "" {
		args = append(args, "--address", h.Address)
	}
	if !h.Remote() {
		return koinon, args
	}
	if ssh == "" {
		ssh = "ssh"
	}
	// ssh joins the words after the destination with spaces for the remote shell, so each
	// word is quoted for it. BatchMode stops ssh from asking for a password or a host key.
	remote := []string{"-o", "BatchMode=yes", "-T", "--", h.SSH, shellQuote(koinon)}
	for _, a := range args {
		remote = append(remote, shellQuote(a))
	}
	return ssh, remote
}

// shellQuote quotes a word for a POSIX shell.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// maxHostsFile bounds hosts.toml.
const maxHostsFile = 64 << 10

// LoadHosts reads and checks a hosts file.
func LoadHosts(path string) ([]Host, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if info.Size() > maxHostsFile {
		return nil, errors.New(path + ": larger than 64 KiB")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var f struct {
		Host []Host `toml:"host"`
	}
	meta, err := toml.Decode(string(data), &f)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if undecoded := meta.Undecoded(); len(undecoded) > 0 {
		return nil, fmt.Errorf("%s: unknown field %s", path, undecoded[0])
	}
	if len(f.Host) == 0 {
		return nil, fmt.Errorf("%s: no [[host]] rows", path)
	}
	seen := map[string]bool{}
	for i, h := range f.Host {
		switch {
		case h.Label == "":
			return nil, fmt.Errorf("%s: host %d has no label", path, i+1)
		case seen[h.Label]:
			return nil, fmt.Errorf("%s: host %s is listed twice", path, h.Label)
		case h.Secret == "":
			return nil, fmt.Errorf("%s: host %s has no secret (the Keychain name of its maintainer secret)", path, h.Label)
		case strings.HasPrefix(h.SSH, "-"):
			return nil, fmt.Errorf("%s: host %s: ssh %q begins with -", path, h.Label, h.SSH)
		case h.Remote() && !strings.HasPrefix(h.Koinon, "/"):
			return nil, fmt.Errorf("%s: host %s: koinon must be the full path of the remote koinon binary", path, h.Label)
		}
		seen[h.Label] = true
	}
	return f.Host, nil
}
