package signed

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
)

// Version is the envelope version of the v1 contract; the signed bytes start with it and a
// newline.
const Version = "koinon-maintainer-1"

// Envelope is one signed maintainer action, or an enrollment statement (op "enroll", no
// request). Args holds only v1 values (see Canonical).
type Envelope struct {
	V       string         `json:"v"`
	Key     string         `json:"key"`
	Target  string         `json:"target"`
	Op      string         `json:"op"`
	Args    map[string]any `json:"args"`
	Request string         `json:"request,omitempty"`
	Created int64          `json:"created"`
	Sig     string         `json:"sig,omitempty"`
}

var requestPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{16,128}$`)

// SignedBytes are the bytes that the signature covers: the version, a newline, and the
// RFC 8785 bytes of the envelope without sig. An envelope with no request leaves the field
// out, as the enrollment statement does.
func SignedBytes(e Envelope) ([]byte, error) {
	if e.V != Version {
		return nil, fmt.Errorf("envelope version %q is not %s", e.V, Version)
	}
	if e.Request != "" && !requestPattern.MatchString(e.Request) {
		return nil, errors.New("the request ID is not 16 to 128 of [A-Za-z0-9_-]")
	}
	args := e.Args
	if args == nil {
		args = map[string]any{}
	}
	m := map[string]any{
		"v": e.V, "key": e.Key, "target": e.Target, "op": e.Op, "args": args, "created": e.Created,
	}
	if e.Request != "" {
		m["request"] = e.Request
	}
	c, err := Canonical(m)
	if err != nil {
		return nil, err
	}
	return append([]byte(Version+"\n"), c...), nil
}

// Sign sets e.Key to the key's ID and e.Sig to the standard base64 (with padding) Ed25519
// signature of the signed bytes.
func Sign(priv ed25519.PrivateKey, e Envelope) (Envelope, error) {
	e.Key = KeyID(priv.Public().(ed25519.PublicKey))
	e.Sig = ""
	b, err := SignedBytes(e)
	if err != nil {
		return Envelope{}, err
	}
	e.Sig = base64.StdEncoding.EncodeToString(ed25519.Sign(priv, b))
	return e, nil
}

// Verify checks that e names pub's key ID and carries pub's signature of its signed bytes.
func Verify(pub ed25519.PublicKey, e Envelope) error {
	if len(pub) != ed25519.PublicKeySize {
		return errors.New("the public key is not 32 bytes")
	}
	if e.Key != KeyID(pub) {
		return errors.New("the envelope names another key")
	}
	sig, err := base64.StdEncoding.Strict().DecodeString(e.Sig)
	if err != nil || len(sig) != ed25519.SignatureSize {
		return errors.New("the signature is not standard base64 of 64 bytes")
	}
	b, err := SignedBytes(Envelope{V: e.V, Key: e.Key, Target: e.Target, Op: e.Op, Args: e.Args, Request: e.Request, Created: e.Created})
	if err != nil {
		return err
	}
	if !ed25519.Verify(pub, b, sig) {
		return errors.New("the signature does not verify")
	}
	return nil
}

// KeyID is "SHA256:" and the standard base64, without padding, of the SHA-256 of the raw
// 32-byte public key, as OpenSSH writes fingerprints.
func KeyID(pub ed25519.PublicKey) string {
	h := sha256.Sum256(pub)
	return "SHA256:" + base64.RawStdEncoding.EncodeToString(h[:])
}

// NewRequestID returns 22 random characters of [A-Za-z0-9_-] (16 bytes in URL-safe base64).
func NewRequestID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b[:]), nil
}

// Enrollment returns the unsigned enrollment statement for target: op "enroll", args
// {user, client, public_key}, no request (Koinon draft 8dc1c25, Decision 3.5).
func Enrollment(pub ed25519.PublicKey, target, user, client string, created int64) (Envelope, error) {
	if err := CheckUser(user); err != nil {
		return Envelope{}, err
	}
	if err := CheckClient(client); err != nil {
		return Envelope{}, err
	}
	if err := CheckPin(target); err != nil {
		return Envelope{}, err
	}
	return Envelope{
		V: Version, Key: KeyID(pub), Target: target, Op: "enroll", Created: created,
		Args: map[string]any{"user": user, "client": client, "public_key": base64.StdEncoding.EncodeToString(pub)},
	}, nil
}

var (
	userPattern   = regexp.MustCompile(`^[a-z][a-z0-9._-]{0,31}$`)
	clientPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,31}$`)
)

// CheckUser checks a user name against the contract's pattern.
func CheckUser(user string) error {
	if !userPattern.MatchString(user) {
		return fmt.Errorf("the user name %q does not match ^[a-z][a-z0-9._-]{0,31}$", user)
	}
	return nil
}

// CheckClient checks a client label against the contract's pattern.
func CheckClient(client string) error {
	if !clientPattern.MatchString(client) {
		return fmt.Errorf("the client label %q does not match ^[a-z0-9][a-z0-9-]{0,31}$", client)
	}
	return nil
}

// CheckPin checks that a pin is the 64 lower-case hex digits of a SHA-256 fingerprint, as
// `koinon link status` prints it.
func CheckPin(pin string) error {
	if len(pin) != 64 {
		return errors.New("the target pin is not 64 hex digits; copy it from `koinon link status` on the target")
	}
	if _, err := hex.DecodeString(pin); err != nil || pin != fmt.Sprintf("%x", mustHex(pin)) {
		return errors.New("the target pin is not lower-case hex; copy it from `koinon link status` on the target")
	}
	return nil
}

func mustHex(s string) []byte {
	b, _ := hex.DecodeString(s)
	return b
}
