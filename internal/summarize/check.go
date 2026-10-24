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
		case Fields[i] == "recommendation" && claims(text, recommendationClaim, recommendationDenial):
			out[i] = Invented
		case Fields[i] == "deadline" && claims(text, deadlineClaim, deadlineDenial):
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
	// A denial is a negation within a few words of the claim: "no recommendation", "does not
	// give a recommendation", "no recommendation or deadline was given".
	recommendationDenial = regexp.MustCompile(` (no|not|nor|without|neither|doesnt|didnt)( \w+){0,6} (recommend|suggest|preference)`)
	deadlineClaim        = regexp.MustCompile(` (deadline|due|by (monday|tuesday|wednesday|thursday|friday|saturday|sunday|tomorrow|tonight|today|noon|midnight|the end|end of|\d)|before (monday|tuesday|wednesday|thursday|friday|saturday|sunday|tomorrow|tonight|noon|midnight|the end|end of|\d)|until \w+day) `)
	deadlineDenial       = regexp.MustCompile(` (no|not|nor|without|neither|doesnt|didnt)( \w+){0,6} (deadline|due date)`)
)

// sentenceBreak splits a summary into sentences and clauses; "4.1" stays whole.
var sentenceBreak = regexp.MustCompile(`[.!?;:](\s|$)`)

// claims reports whether the text states a fact the item lacks. A sentence that only says
// there is none does not count, and a denial counts only in its own sentence, so "without a
// new build. Claude recommends the folder" is a claim.
func claims(text string, claim, denial *regexp.Regexp) bool {
	for _, sentence := range sentenceBreak.Split(text, -1) {
		norm := normalize(sentence)
		if claim.MatchString(norm) && !denial.MatchString(norm) {
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
