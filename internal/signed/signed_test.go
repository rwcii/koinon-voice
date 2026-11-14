package signed

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/hex"
	"strings"
	"testing"
)

// Koinon's test vectors (rwcii/koinon docs/sprints/2026-11-14-signed-maintainer/decision.md at
// ea8b97a, Decision 3), checked there with Go and Python.
const (
	vecSeed       = "0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20"
	vecPublic     = "79b5562e8fe654f94078b112e8a98ba7901f853ae695bed7e0e3910bad049664"
	vecPublic64   = "ebVWLo/mVPlAeLES6KmLp5AfhTrmlb7X4OORC60ElmQ="
	vecKeyID      = "SHA256:ZbYGc9btiEvwHCwiLYKtoHQPKawzVdapJcgfF/R6J7g"
	vecTarget     = "a0a1a2a3a4a5a6a7a8a9aaabacadaeafb0b1b2b3b4b5b6b7b8b9babbbcbdbebf"
	vecSnapshot   = "350f1e8dd2e447f6bf2594e780e0fc16544c0114cdfdf9cccbb1346a4a9860dc"
	vecSnap2      = "b6c12882582116eae7810fd6cb251045973eea8411b8d800c2c7e0b780d2b5ba"
	vecEnroll     = `{"args":{"client":"koinon-voice","public_key":"ebVWLo/mVPlAeLES6KmLp5AfhTrmlb7X4OORC60ElmQ=","user":"maintainer"},"created":1794699000,"key":"SHA256:ZbYGc9btiEvwHCwiLYKtoHQPKawzVdapJcgfF/R6J7g","op":"enroll","target":"a0a1a2a3a4a5a6a7a8a9aaabacadaeafb0b1b2b3b4b5b6b7b8b9babbbcbdbebf","v":"koinon-maintainer-1"}`
	vecEnrollSig  = "3mzi0yvfgymYoV3Wp6xVOwjOKuUQTP/9+hXAd23WytD8ykv/4vTF1Tpu3t4H8ZBkr7OtLJfr9NoNvVkhTKOqBQ=="
	vecVerdict    = `{"args":{"note":"ok <&> \"quoted\"","reference":"VEC-1","revision":3,"snapshot":"350f1e8dd2e447f6bf2594e780e0fc16544c0114cdfdf9cccbb1346a4a9860dc","verdict":"approved"},"created":1794699000,"key":"SHA256:ZbYGc9btiEvwHCwiLYKtoHQPKawzVdapJcgfF/R6J7g","op":"decision_verdict","request":"vec-request-000001","target":"a0a1a2a3a4a5a6a7a8a9aaabacadaeafb0b1b2b3b4b5b6b7b8b9babbbcbdbebf","v":"koinon-maintainer-1"}`
	vecVerdictSig = "5RIpFZ7G+PMlGqvRQizOOmOaPK637OhJX9dCifNKOjIOXjvG4drxmGRLZVaViYYundattt/zTlJOHDiq6zeKCQ=="
)

func vecKey(t *testing.T) ed25519.PrivateKey {
	t.Helper()
	seed, _ := hex.DecodeString(vecSeed)
	return ed25519.NewKeyFromSeed(seed)
}

func vecItem() Snapshot {
	return Snapshot{Reference: "VEC-1", Revision: 3, Kind: "decision", Title: "Approve the test vector",
		Where: "docs/test", Context: "- Line one\n- Line two", Recommendation: "Approve."}
}

func TestKoinonVectors(t *testing.T) {
	k := vecKey(t)
	pub := k.Public().(ed25519.PublicKey)
	if hex.EncodeToString(pub) != vecPublic || base64.StdEncoding.EncodeToString(pub) != vecPublic64 {
		t.Fatalf("public key %x", pub)
	}
	if KeyID(pub) != vecKeyID {
		t.Errorf("key ID %s", KeyID(pub))
	}
	snap := vecItem()
	if d, err := snap.Digest(); err != nil || d != vecSnapshot {
		t.Errorf("snapshot, no commands: %s %v", d, err)
	}
	snap.Commands = []string{}
	if d, _ := snap.Digest(); d != vecSnapshot {
		t.Errorf("snapshot, empty commands: %s", d)
	}
	snap.Commands = []string{"koinon version", "git status --short"}
	if d, err := snap.Digest(); err != nil || d != vecSnap2 {
		t.Errorf("snapshot, two commands: %s %v", d, err)
	}

	enroll, err := Enrollment(pub, vecTarget, "maintainer", "koinon-voice", 1794699000)
	if err != nil {
		t.Fatal(err)
	}
	checkVector(t, k, enroll, vecEnroll, vecEnrollSig)

	verdict := Envelope{V: Version, Target: vecTarget, Op: "decision_verdict", Request: "vec-request-000001", Created: 1794699000,
		Args: map[string]any{"note": `ok <&> "quoted"`, "reference": "VEC-1", "revision": 3, "snapshot": vecSnapshot, "verdict": "approved"}}
	checkVector(t, k, verdict, vecVerdict, vecVerdictSig)
}

func checkVector(t *testing.T, k ed25519.PrivateKey, e Envelope, canonical, sig string) {
	t.Helper()
	s, err := Sign(k, e)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := SignedBytes(s)
	if string(b) != Version+"\n"+canonical {
		t.Errorf("%s signed bytes\n got %s\nwant %s", e.Op, b, canonical)
	}
	if s.Sig != sig {
		t.Errorf("%s signature %s", e.Op, s.Sig)
	}
	if err := Verify(k.Public().(ed25519.PublicKey), s); err != nil {
		t.Errorf("%s verify: %v", e.Op, err)
	}
}

// RFC 8785, section 3.2.3: the sorting example, by UTF-16 code units.
func TestCanonicalSortsByUTF16(t *testing.T) {
	m := map[string]any{"\u20ac": "Euro Sign", "\r": "Carriage Return", "\ufb33": "Hebrew Letter Dalet With Dagesh",
		"1": "One", "\U0001F600": "Emoji: Grinning Face", "\u0080": "Control", "\u00f6": "Latin Small Letter O With Diaeresis"}
	b, err := Canonical(m)
	if err != nil {
		t.Fatal(err)
	}
	want := "{\"\\r\":\"Carriage Return\",\"1\":\"One\",\"\u0080\":\"Control\",\"\u00f6\":\"Latin Small Letter O With Diaeresis\",\"\u20ac\":\"Euro Sign\",\"\U0001F600\":\"Emoji: Grinning Face\",\"\ufb33\":\"Hebrew Letter Dalet With Dagesh\"}"
	if string(b) != want {
		t.Errorf("got  %s\nwant %s", b, want)
	}
}

func TestCanonicalStrings(t *testing.T) {
	cases := map[string]string{
		"a\"b\\c":            `"a\"b\\c"`,
		"\b\f\n\r\t":         `"\b\f\n\r\t"`,
		"\x00\x1f\x7f":       "\"\\u0000\\u001f\x7f\"",
		"<&> \u2028\u2029 /": "\"<&> \u2028\u2029 /\"",
		"é \U0001F600":       "\"é \U0001F600\"",
	}
	for in, want := range cases {
		b, err := Canonical(in)
		if err != nil || string(b) != want {
			t.Errorf("%q: got %s %v, want %s", in, b, err, want)
		}
	}
	if _, err := Canonical("\xff"); err == nil {
		t.Error("invalid UTF-8 was accepted")
	}
	if _, err := Canonical(map[string]any{"\xff": "x"}); err == nil {
		t.Error("an invalid UTF-8 name was accepted")
	}
}

func TestCanonicalValues(t *testing.T) {
	b, err := Canonical(map[string]any{"b": map[string]any{"z": true, "a": false}, "a": -MaxInt, "c": int64(MaxInt), "d": 0})
	if err != nil || string(b) != `{"a":-9007199254740991,"b":{"a":false,"z":true},"c":9007199254740991,"d":0}` {
		t.Errorf("got %s %v", b, err)
	}
	for name, v := range map[string]any{
		"float": 1.5, "null": nil, "array": []string{"a"}, "any array": []any{1},
		"too big": int64(MaxInt + 1), "too small": int64(-MaxInt - 1), "uint": uint(1),
	} {
		if _, err := Canonical(map[string]any{"x": v}); err == nil {
			t.Errorf("%s was accepted", name)
		}
	}
}

func TestTamperIsRefused(t *testing.T) {
	k := vecKey(t)
	pub := k.Public().(ed25519.PublicKey)
	base := Envelope{V: Version, Target: vecTarget, Op: "decision_verdict", Request: "vec-request-000001", Created: 1794699000,
		Args: map[string]any{"reference": "VEC-1", "revision": 3, "snapshot": vecSnapshot, "verdict": "approved", "note": ""}}
	s, err := Sign(k, base)
	if err != nil {
		t.Fatal(err)
	}
	changes := map[string]func(e *Envelope){
		"target":   func(e *Envelope) { e.Target = strings.Repeat("b", 64) },
		"op":       func(e *Envelope) { e.Op = "decision_get" },
		"request":  func(e *Envelope) { e.Request = "vec-request-000002" },
		"created":  func(e *Envelope) { e.Created++ },
		"revision": func(e *Envelope) { e.Args = copyArgs(e.Args, "revision", 4) },
		"verdict":  func(e *Envelope) { e.Args = copyArgs(e.Args, "verdict", "declined") },
		"note":     func(e *Envelope) { e.Args = copyArgs(e.Args, "note", "x") },
		"extra":    func(e *Envelope) { e.Args = copyArgs(e.Args, "extra", "x") },
		"sig":      func(e *Envelope) { e.Sig = base64.StdEncoding.EncodeToString(make([]byte, 64)) },
		"no sig":   func(e *Envelope) { e.Sig = "" },
	}
	for name, change := range changes {
		e := s
		change(&e)
		if Verify(pub, e) == nil {
			t.Errorf("a changed %s verified", name)
		}
	}
	other := ed25519.NewKeyFromSeed(make([]byte, 32))
	if Verify(other.Public().(ed25519.PublicKey), s) == nil {
		t.Error("another key verified")
	}
	if Verify(pub, s) != nil {
		t.Error("the untouched envelope did not verify")
	}
}

func copyArgs(m map[string]any, k string, v any) map[string]any {
	c := map[string]any{}
	for key, val := range m {
		c[key] = val
	}
	c[k] = v
	return c
}

func TestChecks(t *testing.T) {
	if _, err := SignedBytes(Envelope{V: Version, Request: "short"}); err == nil {
		t.Error("a short request ID was accepted")
	}
	if _, err := SignedBytes(Envelope{V: "koinon-maintainer-2"}); err == nil {
		t.Error("another version was accepted")
	}
	id, err := NewRequestID()
	if err != nil || !requestPattern.MatchString(id) {
		t.Errorf("request ID %q %v", id, err)
	}
	for _, u := range []string{"", "Robert", "1a", "a b", strings.Repeat("a", 33)} {
		if CheckUser(u) == nil {
			t.Errorf("user %q was accepted", u)
		}
	}
	for _, c := range []string{"", "-a", "Koinon", strings.Repeat("a", 33)} {
		if CheckClient(c) == nil {
			t.Errorf("client %q was accepted", c)
		}
	}
	for _, p := range []string{"", "abc", strings.ToUpper(vecTarget), vecTarget[:63] + "g"} {
		if CheckPin(p) == nil {
			t.Errorf("pin %q was accepted", p)
		}
	}
	pub := vecKey(t).Public().(ed25519.PublicKey)
	if _, err := Enrollment(pub, vecTarget, "Bad", "koinon-voice", 1); err == nil {
		t.Error("enrollment took a bad user")
	}
}
