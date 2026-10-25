package speech

import (
	"bufio"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
)

// transcriptRow is a row of a recognition scorecard's Transcripts table, as
// WriteSTTScorecard writes it.
var transcriptRow = regexp.MustCompile(`^\| (\d+) \| (\S+) \| (\S+) \| \d+/\d+ \| \d+/\d+ \| \d+ ms \| (.*) \|$`)

// ParseTranscripts reads the transcripts of a recognition scorecard, so they can be scored
// again without the recordings. A failed file is left out.
func ParseTranscripts(r io.Reader, phrases []Phrase) ([]Transcript, error) {
	byN := map[int]Phrase{}
	for _, p := range phrases {
		byN[p.N] = p
	}
	var out []Transcript
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	for sc.Scan() {
		m := transcriptRow.FindStringSubmatch(sc.Text())
		if m == nil || strings.HasPrefix(m[4], "failed: ") {
			continue
		}
		n, _ := strconv.Atoi(m[1])
		p, ok := byN[n]
		if !ok {
			return nil, fmt.Errorf("transcript of phrase %d, which is not in the phrase list", n)
		}
		text := strings.ReplaceAll(m[4], `\|`, "|")
		out = append(out, Transcript{Engine: m[3], Set: m[2], Phrase: p, Text: text, Score: Compare(p, text)})
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no transcripts")
	}
	return out, nil
}

// rightName reports whether a match's name lies on the same words of the phrase, and those
// are Koinon terms: "cannon" resolved to "koinon" in "Koinon, fire the cannon" is wrong,
// though the phrase has the name elsewhere.
func rightName(p Phrase, resolved []string, at []int, m Match) bool {
	n := len(strings.Fields(m.Name))
	for k := 0; k < n; k++ {
		j := m.At + k
		r := at[j]
		if r < 0 || p.Words[r] != resolved[j] || !p.Terms[r] || (k > 0 && r != at[j-1]+1) {
			return false
		}
	}
	return true
}

// align maps each hyp word to the ref word an optimal edit-distance alignment pairs it with,
// or -1 for an inserted word. Matches are preferred to substitutions on ties.
func align(ref, hyp []string) []int {
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
	at := make([]int, len(hyp))
	for i, j := len(ref), len(hyp); j > 0; {
		switch {
		case i > 0 && d[i][j] == d[i-1][j-1] && ref[i-1] == hyp[j-1], i > 0 && d[i][j] == d[i-1][j-1]+1:
			at[j-1] = i - 1
			i, j = i-1, j-1
		case i > 0 && d[i][j] == d[i-1][j]+1:
			i--
		default:
			at[j-1] = -1
			j--
		}
	}
	return at
}

// NameResult is one transcript before and after name resolution.
type NameResult struct {
	Transcript
	Resolved []string
	After    Score
	Matches  []Match
	// Wrong are the matches whose name does not lie on the same Koinon-term words of the
	// phrase: an ordinary word, or another name, turned into a name.
	Wrong []Match
}

// ResolveNames resolves names in each transcript and scores it again against its phrase.
func ResolveNames(ts []Transcript, names *Names) []NameResult {
	out := make([]NameResult, len(ts))
	for i, t := range ts {
		words, matches := names.Resolve(t.Text)
		r := NameResult{Transcript: t, Resolved: words, After: Compare(t.Phrase, strings.Join(words, " ")), Matches: matches}
		at := align(t.Phrase.Words, words)
		for _, m := range matches {
			if !rightName(t.Phrase, words, at, m) {
				r.Wrong = append(r.Wrong, m)
			}
		}
		out[i] = r
	}
	return out
}

// Ordinary is a held-out check: text that names nothing, one utterance per line, and the
// lines that name resolution changed.
type Ordinary struct {
	Source  string
	Lines   int
	Matched []string // "line: heard → name"
}

// ResolveOrdinary resolves names in each non-empty line of text that should name nothing.
func ResolveOrdinary(source, text string, names *Names) Ordinary {
	o := Ordinary{Source: source}
	for _, line := range strings.Split(text, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		o.Lines++
		if _, ms := names.Resolve(line); len(ms) > 0 {
			var heard []string
			for _, m := range ms {
				heard = append(heard, m.Heard+" → "+m.Name)
			}
			o.Matched = append(o.Matched, strings.TrimSpace(line)+": "+strings.Join(heard, "; "))
		}
	}
	return o
}

// WriteNamesReport writes the error rates before and after name resolution for each engine
// and set, the held-out ordinary text, then every transcript that resolution changed.
func WriteNamesReport(w io.Writer, rs []NameResult, names []string, ordinary []Ordinary) {
	b := &strings.Builder{}
	b.WriteString("## Names\n\n" + strings.Join(names, ", ") + "\n\n")
	b.WriteString("## Totals\n\n| Engine | Set | Files | WER before | WER after | Koinon-term WER before | Koinon-term WER after | Resolved | Wrong | Worse |\n| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |\n")
	type key struct{ engine, set string }
	var order []key
	type tally struct {
		files, resolved, wrong, worse int
		before, after                 Score
	}
	tallies := map[key]*tally{}
	for _, r := range rs {
		k := key{r.Engine, r.Set}
		t := tallies[k]
		if t == nil {
			t = &tally{}
			tallies[k] = t
			order = append(order, k)
		}
		t.files++
		t.before.Add(r.Score)
		t.after.Add(r.After)
		t.resolved += len(r.Matches)
		t.wrong += len(r.Wrong)
		if r.After.Errors > r.Score.Errors {
			t.worse++
		}
	}
	rate := func(e, n int) string { return fmt.Sprintf("%.1f%% (%d/%d)", 100*ratio(e, n), e, n) }
	for _, k := range order {
		t := tallies[k]
		fmt.Fprintf(b, "| %s | %s | %d | %s | %s | %s | %s | %d | %d | %d |\n", k.engine, k.set, t.files,
			rate(t.before.Errors, t.before.Words), rate(t.after.Errors, t.after.Words),
			rate(t.before.TermErrors, t.before.TermWords), rate(t.after.TermErrors, t.after.TermWords),
			t.resolved, t.wrong, t.worse)
	}
	if len(ordinary) > 0 {
		b.WriteString("\n## Ordinary text\n\nLines that name nothing; each one resolution changed is listed.\n\n| Source | Lines | Changed | Changes |\n| --- | --- | --- | --- |\n")
		for _, o := range ordinary {
			fmt.Fprintf(b, "| %s | %d | %d | %s |\n", cell(o.Source), o.Lines, len(o.Matched), cell(strings.Join(o.Matched, "; ")))
		}
	}
	b.WriteString("\n## Changed transcripts\n\nErrors before and after, over reference words; a wrong match is marked.\n\n| Phrase | Set | Engine | Errors | Term errors | Heard | Resolved |\n| --- | --- | --- | --- | --- | --- | --- |\n")
	for _, r := range rs {
		if len(r.Matches) == 0 {
			continue
		}
		var heard []string
		for _, m := range r.Matches {
			s := m.Heard + " → " + m.Name
			for _, x := range r.Wrong {
				if x == m {
					s += " (wrong)"
				}
			}
			heard = append(heard, s)
		}
		fmt.Fprintf(b, "| %02d | %s | %s | %d to %d | %d to %d | %s | %s |\n", r.Phrase.N, r.Set, r.Engine,
			r.Score.Errors, r.After.Errors, r.Score.TermErrors, r.After.TermErrors, cell(strings.Join(heard, "; ")), cell(strings.Join(r.Resolved, " ")))
	}
	io.WriteString(w, b.String())
}
