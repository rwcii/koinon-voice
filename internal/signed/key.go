package signed

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/rwcii/koinon-voice/internal/keychain"
)

// The Keychain item that holds this install's key: one key per install, for one user.
const (
	KeychainService = "koinon-voice-maintainer-key"
	KeychainAccount = "koinon-voice"
)

// Store is the part of the Keychain that the key needs.
type Store interface {
	Find(ctx context.Context, service, account string) (string, error)
	Add(ctx context.Context, service, account string, data []byte) error
}

// Key is this install's key pair and the user it was made for.
type Key struct {
	User    string
	Private ed25519.PrivateKey
}

// Public returns the public key.
func (k Key) Public() ed25519.PublicKey { return k.Private.Public().(ed25519.PublicKey) }

// ID returns the key ID.
func (k Key) ID() string { return KeyID(k.Public()) }

// payload is the Keychain item's password: a versioned JSON object with the user and the
// 32-byte Ed25519 seed. The public key is derived from the seed, so the two cannot disagree.
type payload struct {
	V    int    `json:"v"`
	User string `json:"user"`
	Seed string `json:"seed"`
}

// ErrNoKey is the answer of Load when this install has no key yet.
var ErrNoKey = errors.New("this install has no maintainer key; run koinon-voice key create --user NAME")

// Create makes a new key pair for user and stores it. It refuses when a key exists, and never
// replaces one.
func Create(ctx context.Context, s Store, user string) (Key, error) {
	if err := CheckUser(user); err != nil {
		return Key{}, err
	}
	seed := make([]byte, ed25519.SeedSize)
	if _, err := rand.Read(seed); err != nil {
		return Key{}, err
	}
	data, err := json.Marshal(payload{V: 1, User: user, Seed: base64.StdEncoding.EncodeToString(seed)})
	if err != nil {
		return Key{}, err
	}
	if err := s.Add(ctx, KeychainService, KeychainAccount, data); err != nil {
		return Key{}, err
	}
	return Key{User: user, Private: ed25519.NewKeyFromSeed(seed)}, nil
}

// Load reads this install's key. A missing item is ErrNoKey; a payload that is not version 1,
// names no valid user or holds a seed that is not 32 bytes is an error that carries no key
// material.
func Load(ctx context.Context, s Store) (Key, error) {
	text, err := s.Find(ctx, KeychainService, KeychainAccount)
	if err != nil {
		if isNotFound(err) {
			return Key{}, ErrNoKey
		}
		return Key{}, err
	}
	p, err := decodePayload(text)
	if err != nil {
		return Key{}, fmt.Errorf("the Keychain item %s is not a koinon-voice key", KeychainService)
	}
	if p.V != 1 {
		return Key{}, fmt.Errorf("the Keychain item %s has key version %d; this build reads version 1", KeychainService, p.V)
	}
	if err := CheckUser(p.User); err != nil {
		return Key{}, fmt.Errorf("the Keychain item %s names no valid user", KeychainService)
	}
	seed, err := base64.StdEncoding.Strict().DecodeString(p.Seed)
	if err != nil || len(seed) != ed25519.SeedSize {
		return Key{}, fmt.Errorf("the Keychain item %s does not hold a 32-byte key", KeychainService)
	}
	return Key{User: p.User, Private: ed25519.NewKeyFromSeed(seed)}, nil
}

// decodePayload reads exactly one JSON object with the payload's fields, each once, and
// nothing after it but white space.
func decodePayload(text string) (payload, error) {
	dec := json.NewDecoder(bytes.NewReader([]byte(text)))
	if t, err := dec.Token(); err != nil || t != json.Delim('{') {
		return payload{}, errors.New("not an object")
	}
	seen := map[string]bool{}
	for dec.More() {
		t, err := dec.Token()
		name, ok := t.(string)
		if err != nil || !ok || seen[name] {
			return payload{}, errors.New("a field name is missing or repeated")
		}
		seen[name] = true
		var skip json.RawMessage
		if err := dec.Decode(&skip); err != nil {
			return payload{}, err
		}
	}
	if t, err := dec.Token(); err != nil || t != json.Delim('}') {
		return payload{}, errors.New("the object does not end")
	}
	if _, err := dec.Token(); err != io.EOF {
		return payload{}, errors.New("text after the object")
	}
	var p payload
	strict := json.NewDecoder(bytes.NewReader([]byte(text)))
	strict.DisallowUnknownFields()
	if err := strict.Decode(&p); err != nil {
		return payload{}, err
	}
	return p, nil
}

func isNotFound(err error) bool { return errors.Is(err, keychain.ErrNotFound) }
