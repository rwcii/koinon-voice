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
	asker := item.AuthorName
	if item.AuthorFamily != "" && item.AuthorFamily != asker {
		asker = strings.TrimSpace(asker + " (" + item.AuthorFamily + ")")
	}
	field("Asker", asker)
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
