package koinon

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/BurntSushi/toml"

	"github.com/rwcii/koinon-voice/internal/signed"
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
	// NodePin makes the row a signed node (ADR 004): the node's pin, read on the node itself
	// with `koinon link status`. Every action to it and through it is a signed envelope.
	NodePin string `toml:"node_pin"`
	// Serve is a signed node's Remote (Tailscale Serve) address, https://HOST (R24). Empty is
	// the local form: the koinon binary on this machine, with Koinon, StateDir and Address.
	Serve string `toml:"serve"`
}

// Signed reports whether the host is a signed node.
func (h Host) Signed() bool { return h.NodePin != "" }

// SignedLocal returns the koinon binary and the extra words after
// `mcp --maintainer --signed` for a signed node in the local form (SignedOptions).
func (h Host) SignedLocal() (string, []string) {
	koinon := h.Koinon
	if koinon == "" {
		koinon = "koinon"
	}
	var args []string
	if h.StateDir != "" {
		args = append(args, "--state-dir", h.StateDir)
	}
	if h.Address != "" {
		args = append(args, "--address", h.Address)
	}
	return koinon, args
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
	// Pointers tell a field that is set to "" from one that is absent, so a row with an
	// empty node_pin or serve is a malformed signed row, never a secret row.
	var f struct {
		Host []struct {
			Label    string  `toml:"label"`
			SSH      *string `toml:"ssh"`
			Koinon   *string `toml:"koinon"`
			StateDir *string `toml:"state_dir"`
			Address  *string `toml:"address"`
			Secret   *string `toml:"secret"`
			NodePin  *string `toml:"node_pin"`
			Serve    *string `toml:"serve"`
		} `toml:"host"`
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
	str := func(p *string) string {
		if p == nil {
			return ""
		}
		return *p
	}
	hosts := make([]Host, len(f.Host))
	seen := map[string]bool{}
	pins := map[string]string{}
	for i, r := range f.Host {
		h := Host{Label: r.Label, SSH: str(r.SSH), Koinon: str(r.Koinon), StateDir: str(r.StateDir),
			Address: str(r.Address), Secret: str(r.Secret), NodePin: str(r.NodePin), Serve: str(r.Serve)}
		switch {
		case h.Label == "":
			return nil, fmt.Errorf("%s: host %d has no label", path, i+1)
		case seen[h.Label]:
			return nil, fmt.Errorf("%s: host %s is listed twice", path, h.Label)
		}
		seen[h.Label] = true
		if r.NodePin != nil || r.Serve != nil {
			if err := checkSignedRow(h, r.Serve != nil, r.SSH != nil, r.Secret != nil, r.Koinon != nil || r.StateDir != nil || r.Address != nil); err != nil {
				return nil, fmt.Errorf("%s: host %s: %w", path, h.Label, err)
			}
			if other, ok := pins[h.NodePin]; ok {
				return nil, fmt.Errorf("%s: hosts %s and %s name the same node_pin", path, other, h.Label)
			}
			pins[h.NodePin] = h.Label
			hosts[i] = h
			continue
		}
		switch {
		case h.Secret == "":
			return nil, fmt.Errorf("%s: host %s has no secret (the Keychain name of its maintainer secret) and no node_pin", path, h.Label)
		case strings.HasPrefix(h.SSH, "-"):
			return nil, fmt.Errorf("%s: host %s: ssh %q begins with -", path, h.Label, h.SSH)
		case h.Remote() && !strings.HasPrefix(h.Koinon, "/"):
			return nil, fmt.Errorf("%s: host %s: koinon must be the full path of the remote koinon binary", path, h.Label)
		}
		hosts[i] = h
	}
	return hosts, nil
}

// checkSignedRow checks a row with node_pin or serve: a signed node has a valid pin, no ssh
// and no secret, and either a Serve address or the local form's fields, not both.
func checkSignedRow(h Host, hasServe, hasSSH, hasSecret, hasLocal bool) error {
	switch {
	case h.NodePin == "":
		return errors.New("a signed node needs node_pin, read on the node with `koinon link status`")
	case hasSSH:
		return errors.New("a signed node has no ssh; set serve to reach it remotely")
	case hasSecret:
		return errors.New("a signed node has no secret; it signs with this install's maintainer key")
	}
	if err := signed.CheckPin(h.NodePin); err != nil {
		return fmt.Errorf("node_pin: %w", err)
	}
	if !hasServe {
		return nil
	}
	if hasLocal {
		return errors.New("a signed node with serve has no koinon, state_dir or address")
	}
	if _, err := checkBase(h.Serve); err != nil {
		return fmt.Errorf("serve: %w", err)
	}
	return nil
}
