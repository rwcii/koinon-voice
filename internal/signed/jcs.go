// Package signed builds and signs the maintainer envelopes of Koinon's koinon-maintainer-1
// contract (rwcii/koinon#693; ADR 004): the key pair of one user and install, its key ID, the
// RFC 8785 canonical bytes of an envelope and its Ed25519 signature.
package signed

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

// MaxInt is the largest integer magnitude that the v1 contract allows, 2^53-1, so that an
// integer has one exact IEEE-754 form (RFC 8785, section 3.2.2.3).
const MaxInt = 1<<53 - 1

// Canonical returns the RFC 8785 (JSON Canonicalization Scheme) bytes of v for the value
// shapes of the v1 contract: objects (map[string]any), strings, integers (int, int64) within
// ±MaxInt and booleans. It refuses floats, null, arrays and any other type, and strings that
// are not valid UTF-8, instead of guessing at their form.
func Canonical(v any) ([]byte, error) {
	var b strings.Builder
	if err := writeValue(&b, v, false); err != nil {
		return nil, err
	}
	return []byte(b.String()), nil
}

func writeValue(b *strings.Builder, v any, arrays bool) error {
	switch x := v.(type) {
	case map[string]any:
		return writeObject(b, x, arrays)
	case []string:
		// Only the snapshot object has an array (its commands); it is hashed, never sent.
		if !arrays {
			return errors.New("canonical JSON: an array is not a v1 value")
		}
		b.WriteByte('[')
		for i, s := range x {
			if i > 0 {
				b.WriteByte(',')
			}
			if err := writeString(b, s); err != nil {
				return err
			}
		}
		b.WriteByte(']')
		return nil
	case string:
		return writeString(b, x)
	case bool:
		if x {
			b.WriteString("true")
		} else {
			b.WriteString("false")
		}
		return nil
	case int:
		return writeInt(b, int64(x))
	case int64:
		return writeInt(b, x)
	case nil:
		return errors.New("canonical JSON: null is not a v1 value")
	default:
		return fmt.Errorf("canonical JSON: %T is not a v1 value", v)
	}
}

func writeInt(b *strings.Builder, n int64) error {
	if n > MaxInt || n < -MaxInt {
		return fmt.Errorf("canonical JSON: integer %d is outside ±(2^53-1)", n)
	}
	b.WriteString(strconv.FormatInt(n, 10))
	return nil
}

// writeObject sorts the property names by their UTF-16 code units (RFC 8785, section 3.2.3).
func writeObject(b *strings.Builder, m map[string]any, arrays bool) error {
	keys := make([]string, 0, len(m))
	for k := range m {
		if !utf8.ValidString(k) {
			return errors.New("canonical JSON: a property name is not valid UTF-8")
		}
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return lessUTF16(keys[i], keys[j]) })
	b.WriteByte('{')
	for i, k := range keys {
		if i > 0 {
			b.WriteByte(',')
		}
		if err := writeString(b, k); err != nil {
			return err
		}
		b.WriteByte(':')
		if err := writeValue(b, m[k], arrays); err != nil {
			return fmt.Errorf("%s: %w", k, err)
		}
	}
	b.WriteByte('}')
	return nil
}

func lessUTF16(a, b string) bool {
	ua, ub := utf16.Encode([]rune(a)), utf16.Encode([]rune(b))
	for i := 0; i < len(ua) && i < len(ub); i++ {
		if ua[i] != ub[i] {
			return ua[i] < ub[i]
		}
	}
	return len(ua) < len(ub)
}

// writeString escapes as RFC 8785, section 3.2.2.2 says: the quotation mark, the reverse
// solidus, and the controls below U+0020, with the short forms where JSON has them and
// \u00xx in lower-case hex otherwise. Every other character, <, > and & and U+2028 and
// U+2029 included, is written as itself.
func writeString(b *strings.Builder, s string) error {
	if !utf8.ValidString(s) {
		return errors.New("canonical JSON: a string is not valid UTF-8")
	}
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\b':
			b.WriteString(`\b`)
		case '\f':
			b.WriteString(`\f`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			if r < 0x20 {
				fmt.Fprintf(b, `\u%04x`, r)
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
	return nil
}
