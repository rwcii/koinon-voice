package speech

import "sort"

// common words say nothing about where a transcript came from: both the bridge's speech and
// an interruption may have them.
var common = map[string]bool{
	"a": true, "an": true, "and": true, "are": true, "as": true, "at": true, "be": true, "but": true,
	"by": true, "for": true, "from": true, "i": true, "if": true, "in": true, "is": true, "it": true,
	"its": true, "me": true, "my": true, "no": true, "not": true, "of": true, "on": true, "or": true,
	"so": true, "that": true, "the": true, "this": true, "to": true, "was": true, "we": true,
	"with": true, "you": true, "your": true,
}

// Interruption scores a duplex transcript (definition of done, live check 5): which words of
// the maintainer's interruption it holds, and which of the bridge's own words leaked into it
// through the echo canceller. Common words and words of both texts count for neither.
type Interruption struct {
	Heard, Missed []string // the interruption's words found and not found
	Leaked        []string // transcript words that are the bridge's, not the interruption's
}

// Passed is the check: the whole interruption is heard and nothing of the bridge's speech.
func (i Interruption) Passed() bool { return len(i.Missed) == 0 && len(i.Leaked) == 0 }

// ScoreInterruption compares a transcript with the bridge's text and the interruption said
// over it. With no interruption (the echo control) only leaks count.
func ScoreInterruption(bridge, interruption, transcript string) Interruption {
	set := func(s string) map[string]bool {
		m := map[string]bool{}
		for _, w := range Normalize(s) {
			if !common[w] {
				m[w] = true
			}
		}
		return m
	}
	b, said, heard := set(bridge), set(interruption), set(transcript)
	var r Interruption
	for w := range said {
		if heard[w] {
			r.Heard = append(r.Heard, w)
		} else {
			r.Missed = append(r.Missed, w)
		}
	}
	for w := range heard {
		if b[w] && !said[w] {
			r.Leaked = append(r.Leaked, w)
		}
	}
	sort.Strings(r.Heard)
	sort.Strings(r.Missed)
	sort.Strings(r.Leaked)
	return r
}
