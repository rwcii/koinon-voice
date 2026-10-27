package summarize

import (
	"fmt"
	"strings"
)

// Instructions is the system prompt every engine receives. The item follows in the user turn.
const Instructions = `You write the spoken summary of one decision item for the maintainer, who hears it through a speaker and does not see the item.

Write two or three short sentences of plain speech. Say who asks, what they ask the maintainer to decide, the options, the asker's recommendation and any deadline.

Rules:
- Say only what the item says. If the item has no recommendation, say that there is none or leave it out; never invent one. Do the same for a deadline.
- Never read out hashes, file paths, URLs, command lines or code. Name what they are for instead.
- Say the asker as the item gives it. Say any other agent session's name the same way: the agent family, then the project, without its letters or numbers.
- Keep only the figures that the decision needs.
- Write numbers as they are spoken, for example "pull request four sixty-six".
- Use no markdown, lists, headings or symbols.
- Reply with the summary only.`

// Prompt renders the item as the user turn.
func Prompt(item Item) string {
	var b strings.Builder
	field := func(name, value string) {
		if value = strings.TrimSpace(value); value != "" {
			fmt.Fprintf(&b, "%s: %s\n", name, value)
		}
	}
	field("Asker", SpokenAsker(item.AuthorFamily, item.AuthorName))
	field("Kind", item.Kind)
	field("Title", item.Title)
	field("Where", item.Where)
	field("Asker's text", item.AgentText)
	field("Context", item.Context)
	field("Recommendation", item.Recommendation)
	for _, c := range item.Commands {
		field("Command", c)
	}
	return b.String()
}

// familyNames are the agent families as they are spoken.
var familyNames = map[string]string{
	"agy": "Antigravity", "claude": "Claude", "codex": "Codex", "deepseek": "DeepSeek",
	"maintainer": "the maintainer", "opencode": "OpenCode",
}

// SpokenAsker says an item's author as a person would: the agent family, then the project.
// A Koinon peer name is the family, the repository and a short suffix, so
// "codex-koinon-voice-38" of family codex is "Codex, working in Koinon Voice". A name that
// does not start with its family is given as it is, after the family.
func SpokenAsker(family, name string) string {
	spoken := familyNames[family]
	if spoken == "" {
		spoken = family
	}
	rest, ok := strings.CutPrefix(name, family+"-")
	switch {
	case family == "":
		return name
	case name == "" || name == family:
		return spoken
	case !ok:
		return spoken + ", " + name
	}
	words := strings.FieldsFunc(rest, func(r rune) bool { return r == '-' || r == '_' })
	if n := len(words); n > 1 && isSuffix(words[n-1]) {
		words = words[:n-1]
	}
	for i, w := range words {
		words[i] = strings.ToUpper(w[:1]) + w[1:]
	}
	return spoken + ", working in " + strings.Join(words, " ")
}

// isSuffix reports whether w is a peer name's suffix: two hexadecimal digits.
func isSuffix(w string) bool {
	if len(w) != 2 {
		return false
	}
	for _, r := range w {
		if !strings.ContainsRune("0123456789abcdef", r) {
			return false
		}
	}
	return true
}
