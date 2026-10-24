// Package speech scores recognition engines against the corpus phrase list (chunk 04): word
// error rate overall and on Koinon terms.
package speech

import (
	"bufio"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

// Phrase is one numbered line of the phrase list. Terms marks which of Words are Koinon terms.
type Phrase struct {
	N     int
	Text  string // as read, without brackets
	Words []string
	Terms []bool
}

var phraseLine = regexp.MustCompile(`^(\d+)\. (.+)$`)

// ParsePhrases reads the phrase list: one phrase per line as "NN. text", with Koinon terms in
// brackets. Other lines are ignored.
func ParsePhrases(r io.Reader) ([]Phrase, error) {
	var out []Phrase
	seen := map[int]bool{}
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		m := phraseLine.FindStringSubmatch(strings.TrimSpace(sc.Text()))
		if m == nil {
			continue
		}
		n, _ := strconv.Atoi(m[1])
		if seen[n] {
			return nil, fmt.Errorf("phrase %d appears twice", n)
		}
		seen[n] = true
		p, err := parsePhrase(n, m[2])
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no phrases")
	}
	return out, nil
}

func parsePhrase(n int, s string) (Phrase, error) {
	p := Phrase{N: n, Text: strings.NewReplacer("[", "", "]", "").Replace(s)}
	for rest := s; rest != ""; {
		open := strings.IndexByte(rest, '[')
		if open < 0 {
			p.add(rest, false)
			break
		}
		end := strings.IndexByte(rest[open:], ']')
		if end < 0 {
			return Phrase{}, fmt.Errorf("phrase %d: unclosed bracket", n)
		}
		p.add(rest[:open], false)
		p.add(rest[open+1:open+end], true)
		rest = rest[open+end+1:]
	}
	if len(p.Words) == 0 {
		return Phrase{}, fmt.Errorf("phrase %d has no words", n)
	}
	return p, nil
}

func (p *Phrase) add(s string, term bool) {
	for _, w := range Normalize(s) {
		p.Words = append(p.Words, w)
		p.Terms = append(p.Terms, term)
	}
}

var (
	units = map[string]int{"zero": 0, "one": 1, "two": 2, "three": 3, "four": 4, "five": 5, "six": 6, "seven": 7, "eight": 8, "nine": 9}
	teens = map[string]int{"ten": 10, "eleven": 11, "twelve": 12, "thirteen": 13, "fourteen": 14, "fifteen": 15, "sixteen": 16, "seventeen": 17, "eighteen": 18, "nineteen": 19}
	tens  = map[string]int{"twenty": 20, "thirty": 30, "forty": 40, "fifty": 50, "sixty": 60, "seventy": 70, "eighty": 80, "ninety": 90}

	letterDigit = regexp.MustCompile(`([a-z])(\d)|(\d)([a-z])`)
	ordinal     = regexp.MustCompile(`\b(1st|2nd|3rd)\b`)

	// Spellings that are the same spoken word: British and American spellings, and a
	// compound written as one word or two. An engine's choice between them is not an error.
	spellings = map[string]string{"summarise": "summarize", "1st": "first", "2nd": "second", "3rd": "third"}
	compounds = map[[2]string]string{{"good", "night"}: "goodnight"}
)

// Normalize turns text into the words both sides of a score are compared on: lower case,
// no punctuation, hyphens and slashes as spaces, number words as digits, a letter-digit run
// split ("b9" is "b 9") and spelled letters joined ("a f" is "af").
func Normalize(s string) []string {
	s = strings.ToLower(s)
	s = strings.Map(func(r rune) rune {
		switch {
		case r == '\'' || r == '’':
			return -1 // "what's" is "whats" on both sides
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			return r
		}
		return ' '
	}, s)
	s = ordinal.ReplaceAllStringFunc(s, func(o string) string { return spellings[o] })
	for letterDigit.MatchString(s) {
		s = letterDigit.ReplaceAllString(s, "$1$3 $2$4")
	}
	words := numbers(strings.Fields(s))
	// Join runs of single letters, which are spelled-out names: "a f" is "af".
	var out []string
	for i := 0; i < len(words); i++ {
		w := words[i]
		if s, ok := spellings[w]; ok {
			w = s
		}
		if i+1 < len(words) {
			if c, ok := compounds[[2]string{w, words[i+1]}]; ok {
				out = append(out, c)
				i++
				continue
			}
		}
		if isLetter(w) {
			for i+1 < len(words) && isLetter(words[i+1]) {
				i++
				w += words[i]
			}
		}
		out = append(out, w)
	}
	return out
}

func isLetter(w string) bool { return len(w) == 1 && w[0] >= 'a' && w[0] <= 'z' }

// numbers turns number words into digits: "thirty eight" is 38, "four sixty four" (an issue
// number read in pairs) is 464 and "four hundred twelve" is 412. "one" is always 1, on both
// sides alike. A digit followed by a two-digit number, in words or digits, is one number:
// "four, 64" is 464.
func numbers(words []string) []string {
	var out []string
	for i := 0; i < len(words); {
		n, used := number(words[i:])
		if used == 0 {
			out = append(out, words[i])
			i++
			continue
		}
		out = append(out, strconv.Itoa(n))
		i += used
	}
	for i := 0; i+1 < len(out); i++ {
		a, err1 := strconv.Atoi(out[i])
		b, err2 := strconv.Atoi(out[i+1])
		if err1 == nil && err2 == nil && a >= 1 && a <= 9 && len(out[i+1]) == 2 && b >= 10 {
			out = append(out[:i], append([]string{strconv.Itoa(a*100 + b)}, out[i+2:]...)...)
		}
	}
	return out
}

// number reads one number from the start of words and returns how many words it used, 0
// when the first is not a number word.
func number(words []string) (int, int) {
	small := func(ws []string) (int, int) {
		if len(ws) == 0 {
			return 0, 0
		}
		if v, ok := teens[ws[0]]; ok {
			return v, 1
		}
		if v, ok := tens[ws[0]]; ok {
			if len(ws) > 1 {
				if u, ok := units[ws[1]]; ok && u > 0 {
					return v + u, 2
				}
			}
			return v, 1
		}
		if v, ok := units[ws[0]]; ok {
			return v, 1
		}
		return 0, 0
	}
	v, used := small(words)
	if used == 0 {
		return 0, 0
	}
	rest := words[used:]
	switch {
	case v < 10 && len(rest) > 0 && rest[0] == "hundred":
		n, u := small(rest[1:])
		if len(rest) > 1 && rest[1] == "and" {
			n, u = small(rest[2:])
			if u > 0 {
				u++
			}
		}
		return v*100 + n, used + 1 + u
	case v > 0 && v < 10 && len(rest) > 0:
		// "four sixty four" and "four twelve": a digit then a two-digit number.
		if n, u := small(rest); u > 0 && n >= 10 {
			return v*100 + n, used + u
		}
	}
	return v, used
}

// Score is word errors against reference words, overall and on Koinon terms.
type Score struct {
	Words, Errors         int // reference words, and substitutions + deletions + insertions
	TermWords, TermErrors int // reference words that are Koinon terms, and errors on them
}

// Add adds another score's counts.
func (s *Score) Add(o Score) {
	s.Words += o.Words
	s.Errors += o.Errors
	s.TermWords += o.TermWords
	s.TermErrors += o.TermErrors
}

// WER is the word error rate; TermWER is the same on Koinon terms only.
func (s Score) WER() float64     { return ratio(s.Errors, s.Words) }
func (s Score) TermWER() float64 { return ratio(s.TermErrors, s.TermWords) }

func ratio(a, b int) float64 {
	if b == 0 {
		return 0
	}
	return float64(a) / float64(b)
}

// Compare aligns a transcript with the phrase by edit distance. Every substitution, deletion
// and insertion is a word error. On Koinon terms, a substituted or deleted term word is an
// error, and so is a word inserted inside a term ("codex koinon voice X af"); words inserted
// around a term are not the term's errors.
func Compare(p Phrase, transcript string) Score {
	ref, hyp := p.Words, Normalize(transcript)
	// d[i][j] is the edit distance between ref[:i] and hyp[:j].
	d := make([][]int, len(ref)+1)
	for i := range d {
		d[i] = make([]int, len(hyp)+1)
		d[i][0] = i
	}
	for j := range d[0] {
		d[0][j] = j
	}
	for i := 1; i <= len(ref); i++ {
		for j := 1; j <= len(hyp); j++ {
			sub := d[i-1][j-1]
			if ref[i-1] != hyp[j-1] {
				sub++
			}
			d[i][j] = min(sub, d[i-1][j]+1, d[i][j-1]+1)
		}
	}
	s := Score{Words: len(ref), Errors: d[len(ref)][len(hyp)]}
	for _, t := range p.Terms {
		if t {
			s.TermWords++
		}
	}
	// Walk back through one optimal alignment and count the errors on terms. An insertion
	// between ref[i-1] and ref[i] is inside a term when both are term words.
	term := func(i int) bool { return i >= 0 && i < len(ref) && p.Terms[i] }
	for i, j := len(ref), len(hyp); i > 0 || j > 0; {
		switch {
		case i > 0 && j > 0 && d[i][j] == d[i-1][j-1] && ref[i-1] == hyp[j-1]:
			i, j = i-1, j-1
		case i > 0 && j > 0 && d[i][j] == d[i-1][j-1]+1:
			if term(i - 1) {
				s.TermErrors++
			}
			i, j = i-1, j-1
		case i > 0 && d[i][j] == d[i-1][j]+1:
			if term(i - 1) {
				s.TermErrors++
			}
			i--
		default:
			if term(i-1) && term(i) {
				s.TermErrors++
			}
			j--
		}
	}
	return s
}
