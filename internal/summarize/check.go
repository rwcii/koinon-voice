package summarize

import (
	"regexp"
	"strings"
	"unicode"
)

// Verdict is the deterministic result for one answer-key field.
type Verdict string

const (
	// Found: every group of the key matched.
	Found Verdict = "ok"
	// Missing: a group did not match. A paraphrase can cause this, so the maintainer
	// confirms it.
	Missing Verdict = "missing"
	// Absent: the item has no such fact and the summary does not claim one.
	Absent Verdict = "none"
	// Invented: the item has no such fact and the summary claims one.
	Invented Verdict = "invented"
	// Review: no answer key; the maintainer judges.
	Review Verdict = "review"
)

// Fields are the checked fields in scorecard order.
var Fields = []string{"asker", "ask", "options", "recommendation", "deadline"}

// Check scores a summary against an answer key, one verdict per field in Fields order.
func Check(text string, key *Key) []Verdict {
	out := make([]Verdict, len(Fields))
	if key == nil {
		for i := range out {
			out[i] = Review
		}
		return out
	}
	norm := normalize(text)
	for i, f := range []Field{key.Asker, key.Ask, key.Options, key.Recommendation, key.Deadline} {
		switch {
		case len(f) > 0 && matches(norm, f):
			out[i] = Found
		case len(f) > 0:
			out[i] = Missing
		case Fields[i] == "recommendation" && claims(text, recommendationClaim, recommendationDenial, recommendationDenialAfter):
			out[i] = Invented
		case Fields[i] == "deadline" && claims(text, deadlineClaim, deadlineDenial, deadlineDenialAfter):
			out[i] = Invented
		default:
			out[i] = Absent
		}
	}
	return out
}

// Correct reports whether no verdict is Missing, Invented or Review.
func Correct(verdicts []Verdict) bool {
	for _, v := range verdicts {
		if v != Found && v != Absent {
			return false
		}
	}
	return true
}

func matches(norm string, f Field) bool {
	for _, group := range f {
		hit := false
		for _, alt := range group {
			// Match at a word start, so "helper" finds "helpers" and "keep" finds "keeping".
			if a := strings.TrimSuffix(normalize(alt), " "); a != "" && strings.Contains(norm, a) {
				hit = true
				break
			}
		}
		if !hit {
			return false
		}
	}
	return true
}

var (
	recommendationClaim = regexp.MustCompile(` (recommend|recommends|recommended|recommendation|suggest|suggests|proposes|advises|prefers) `)
	// A denial is a negation of the claim itself, with only filler words between: "no
	// recommendation", "does not give a recommendation", "no recommendation or deadline was
	// given". "does not require a paid licence and recommends Ava" is not one.
	recommendationDenial = regexp.MustCompile(` (no|not|nor|without|neither|doesnt|didnt)(` + denialFiller + `){0,5} (recommend|suggest|preference)`)
	// A denial can also follow the noun it denies: "a recommendation is not given". A verb
	// is not denied this way, since "recommends not merging" recommends.
	recommendationDenialAfter = regexp.MustCompile(` (recommendation|preference)(` + afterFiller + `){0,3} (` + afterDenial + `)` + afterLimit)
	// "due" claims a deadline only with a time after it: "due to an advisory" does not.
	deadlineClaim       = regexp.MustCompile(` (deadline|due (by|on|at|in|before|this|next|tomorrow|tonight|today|date|\w+day|\d+)|by (monday|tuesday|wednesday|thursday|friday|saturday|sunday|tomorrow|tonight|today|noon|midnight|the end|end of|\d)|before (monday|tuesday|wednesday|thursday|friday|saturday|sunday|tomorrow|tonight|noon|midnight|the end|end of|\d)|until \w+day) `)
	deadlineDenial      = regexp.MustCompile(` (no|not|nor|without|neither|doesnt|didnt)(` + denialFiller + `){0,5} (deadline|due date)`)
	deadlineDenialAfter = regexp.MustCompile(` (deadline|due date)(` + afterFiller + `){0,3} (` + afterDenial + `)` + afterLimit)
)

// denialFiller is a word that may stand between a negation and what it denies.
const denialFiller = ` (a|an|the|any|or|and|nor|one|clear|explicit|specific|firm|set|give|gives|given|make|makes|made|offer|offers|state|states|stated|mention|mentions|mentioned|include|includes|provide|provides|express|expresses|have|has|had|is|was|there|recommendation|deadline)`

// afterFiller is a word that may stand between a noun and the denial after it ("the deadline
// is not"); afterDenial is that denial. "The deadline is Friday, not Monday" is a claim,
// since "Friday" is not filler. afterLimit captures a word that turns the denial into a
// claim: "the deadline is not until Friday".
const (
	afterFiller = ` (is|was|are|were|has|had|have|been|yet|also|still|or|and|deadline|recommendation)`
	afterDenial = `not|isnt|wasnt|arent|werent|hasnt|none|unspecified|unstated|unknown|unclear`
	afterLimit  = `( until| before| after| later| past)? `
)

// sentenceBreak splits a summary into sentences and clauses; "4.1" stays whole.
var sentenceBreak = regexp.MustCompile(`[.!?;:](\s|$)`)

// claims reports whether the text states a fact the item lacks. Each claim counts unless a
// denial ends on its own word, so "no recommendation" does not count but "does not
// recommend Kokoro but recommends Ava" does, or a denial after it starts on its own word,
// as in "the deadline is not specified". A denial counts only in its own sentence.
func claims(text string, claim, before, after *regexp.Regexp) bool {
	for _, sentence := range sentenceBreak.Split(text, -1) {
		norm := normalize(sentence)
		denied := before.FindAllStringIndex(norm, -1)
		var deniedAfter [][]int
		for _, d := range after.FindAllStringSubmatchIndex(norm, -1) {
			if d[len(d)-2] < 0 { // no afterLimit word
				deniedAfter = append(deniedAfter, d)
			}
		}
	next:
		for _, c := range claim.FindAllStringIndex(norm, -1) {
			for _, d := range denied {
				if d[1] > c[0] && d[1] <= c[1] {
					continue next
				}
			}
			for _, d := range deniedAfter {
				if d[0] >= c[0] && d[0] < c[1] {
					continue next
				}
			}
			return true
		}
	}
	return false
}

// normalize lowercases, turns every run of non-letters and non-digits into one space and
// pads the result, so whole words match with strings.Contains.
func normalize(s string) string {
	var b strings.Builder
	b.WriteByte(' ')
	space := true
	for _, r := range strings.ToLower(s) {
		if r == '\'' || r == '’' {
			continue // "doesn't" matches as "doesnt"
		}
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			space = false
			continue
		}
		if !space {
			b.WriteByte(' ')
			space = true
		}
	}
	if !space {
		b.WriteByte(' ')
	}
	return b.String()
}
