package koinon

import (
	"errors"
	"fmt"
	"os"

	"github.com/BurntSushi/toml"

	"github.com/rwcii/koinon-voice/internal/signed"
)

// ClientName is the client that this install's key is enrolled for and that each signed
// envelope names; Koinon records a verdict with it as key:koinon-voice (ADR 004, Decision 3).
const ClientName = "koinon-voice"

// Host is one signed node in hosts.toml (ADR 004): Koinon Voice reaches Koinon only as a
// signed client of it.
type Host struct {
	// Label names the host in every view.
	Label string `toml:"label"`
	// Koinon is the koinon binary of the local form; empty is "koinon" on this machine's PATH.
	Koinon string `toml:"koinon"`
	// StateDir and Address select the node's daemon in the local form when it is not the
	// default one.
	StateDir string `toml:"state_dir"`
	Address  string `toml:"address"`
	// NodePin is the node's pin, read on the node itself with `koinon link status`. Every
	// action to it and through it is a signed envelope.
	NodePin string `toml:"node_pin"`
	// Serve is the node's Remote (Tailscale Serve) address, https://HOST (R24). Empty is the
	// local form: the koinon binary on this machine, with Koinon, StateDir and Address.
	Serve string `toml:"serve"`
}

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
	// empty node_pin or serve is a malformed row, and a row that names ssh or secret at all
	// is refused.
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
		h := Host{Label: r.Label, Koinon: str(r.Koinon), StateDir: str(r.StateDir),
			Address: str(r.Address), NodePin: str(r.NodePin), Serve: str(r.Serve)}
		switch {
		case h.Label == "":
			return nil, fmt.Errorf("%s: host %d has no label", path, i+1)
		case seen[h.Label]:
			return nil, fmt.Errorf("%s: host %s is listed twice", path, h.Label)
		case r.SSH != nil || r.Secret != nil:
			return nil, fmt.Errorf("%s: host %s: %w", path, h.Label, errRemovedRow)
		}
		seen[h.Label] = true
		if err := checkSignedRow(h, r.Serve != nil, r.Koinon != nil || r.StateDir != nil || r.Address != nil); err != nil {
			return nil, fmt.Errorf("%s: host %s: %w", path, h.Label, err)
		}
		if other, ok := pins[h.NodePin]; ok {
			return nil, fmt.Errorf("%s: hosts %s and %s name the same node_pin", path, other, h.Label)
		}
		pins[h.NodePin] = h.Label
		hosts[i] = h
	}
	return hosts, nil
}

// errRemovedRow is a row with ssh or secret, the fields of the maintainer-secret connection
// that Koinon Voice no longer has (ADR 004, Decision 1).
var errRemovedRow = errors.New("ssh and secret rows were removed; use a signed node row: node_pin, " +
	"read on the node with `koinon link status`, and serve = \"https://HOST\" to reach another computer " +
	"(README \"Connect to Koinon\")")

// checkSignedRow checks a signed node row: a valid pin, and either a Serve address or the
// local form's fields, not both.
func checkSignedRow(h Host, hasServe, hasLocal bool) error {
	if h.NodePin == "" {
		return errors.New("a signed node needs node_pin, read on the node with `koinon link status` " +
			"(README \"Connect to Koinon\")")
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
