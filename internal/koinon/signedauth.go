package koinon

import (
	"context"
	"encoding/hex"
	"fmt"
	"sort"

	"github.com/rwcii/koinon-voice/internal/signed"
)

// SignedAccess reads what a signed node needs from this install: the maintainer key that
// already exists (it is never created here) and the enrollments that this key signed.
// Tests give it a fake store and a temporary enrollments file.
type SignedAccess struct {
	Store       signed.Store
	Enrollments signed.Enrollments
	// Client is the client label of the sessions, which each enrollment must name.
	Client string
}

// Access is the loaded key with its usable authorizations, by target pin. An authorization
// here makes a pin one Voice may try; only the target's answer confirms it (ADR 004).
type Access struct {
	Key   signed.Key
	Auths map[string]string
	// Refused names, by pin, each recorded enrollment that does not match this key, user
	// or client, or is malformed, with the reason; Voice does not act on these pins.
	Refused map[string]string
}

// Load reads the key and the enrollments. A missing or locked key and an unreadable
// enrollments file are errors: no signed node can be tried, and nothing falls back to
// another way of connecting.
func (a SignedAccess) Load(ctx context.Context) (Access, error) {
	k, err := signed.Load(ctx, a.Store)
	if err != nil {
		return Access{}, err
	}
	m, err := a.Enrollments.Load()
	if err != nil {
		return Access{}, err
	}
	auths, refused := Authorizations(k, a.Client, m)
	return Access{Key: k, Auths: auths, Refused: refused}, nil
}

// Authorizations keeps each enrollment whose pin and authorization ID are well formed and
// whose key ID, user and client are this key's and this client's.
func Authorizations(k signed.Key, client string, enrolled map[string]signed.Enrolled) (map[string]string, map[string]string) {
	auths, refused := map[string]string{}, map[string]string{}
	id := k.ID()
	pins := make([]string, 0, len(enrolled))
	for pin := range enrolled {
		pins = append(pins, pin)
	}
	sort.Strings(pins)
	for _, pin := range pins {
		e := enrolled[pin]
		var reason string
		switch {
		case signed.CheckPin(pin) != nil:
			reason = "the enrollment's pin is malformed"
		case !isAuthID(e.Auth):
			reason = "the enrollment's authorization ID is malformed"
		case e.KeyID != id:
			reason = fmt.Sprintf("enrolled with another key (%s); run koinon-voice key enroll --target %s", e.KeyID, pin)
		case e.User != k.User:
			reason = fmt.Sprintf("enrolled for user %q, not this key's %q; run koinon-voice key enroll --target %s", e.User, k.User, pin)
		case e.Client != client:
			reason = fmt.Sprintf("enrolled for client %q, not %q; run koinon-voice key enroll --target %s --client %s", e.Client, client, pin, client)
		default:
			auths[pin] = e.Auth
			continue
		}
		refused[pin] = reason
	}
	return auths, refused
}

// NodeReason is empty when Voice may try the node with this pin, and otherwise says why not.
// A node that is not tried gets no process and no network call.
func (a Access) NodeReason(pin string) string {
	if _, ok := a.Auths[pin]; ok {
		return ""
	}
	if r, ok := a.Refused[pin]; ok {
		return r
	}
	return fmt.Sprintf("not enrolled from this install; run koinon-voice key enroll --target %s and add the statement on that computer", pin)
}

// isAuthID reports whether s is a hex SHA-256, as signed.AuthID makes it.
func isAuthID(s string) bool {
	if len(s) != 64 {
		return false
	}
	b, err := hex.DecodeString(s)
	return err == nil && hex.EncodeToString(b) == s
}
