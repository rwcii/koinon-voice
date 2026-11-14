package signed

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// Snapshot is the text of a decision item at one revision, as the maintainer sees it. Its
// digest goes into a verdict, so the target refuses a verdict on other text than it holds
// (Koinon draft ea8b97a, Decision 2.2).
type Snapshot struct {
	Reference      string
	Revision       int64
	Kind           string
	Title          string
	Where          string
	Context        string
	Recommendation string
	AgentText      string
	Commands       []string
}

// Digest is the hex SHA-256 of the RFC 8785 bytes of the snapshot object: revision an
// integer, commands an array of strings in their order ([] when there is none), and each
// other field a string ("" when it is empty). The client computes it from the fields that it
// shows, never from a digest in an answer.
func (s Snapshot) Digest() (string, error) {
	commands := s.Commands
	if commands == nil {
		commands = []string{}
	}
	var b strings.Builder
	err := writeValue(&b, map[string]any{
		"reference": s.Reference, "revision": s.Revision, "kind": s.Kind, "title": s.Title,
		"where": s.Where, "context": s.Context, "recommendation": s.Recommendation,
		"agent_text": s.AgentText, "commands": commands,
	}, true)
	if err != nil {
		return "", err
	}
	h := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(h[:]), nil
}
