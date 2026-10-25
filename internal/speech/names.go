package speech

import (
	"sort"
	"strings"
)

// Names resolves Koinon's names in a transcript after recognition (ADR 002, Decision 2,
// proposed): the word "Koinon", peers and repositories, which recognizers without a keyword
// list mishear ("Quinan", "coin in"). A run of transcript words that sounds like a known name
// becomes that name. The names are the ones the connected daemons report; the bench uses the
// keyword list.
type Names struct {
	names []name
}

type name struct {
	words []string
	sound string
}

// NewNames prepares names in any spelling Normalize accepts ("codex-koinon-voice-af").
func NewNames(names []string) *Names {
	n := &Names{}
	for _, s := range names {
		words := Normalize(s)
		if len(words) > 0 {
			n.names = append(n.names, name{words: words, sound: sound(strings.Join(words, ""))})
		}
	}
	return n
}

// Match is one run of transcript words resolved to a name.
type Match struct {
	Start, End int // the run is words[Start:End] of the normalized transcript
	Heard      string
	Name       string
}

// Resolve returns the transcript's normalized words with each run that sounds like a name
// replaced by the name, and the runs it replaced. A run of up to two words more than the name
// sounds like it when their sound keys differ by at most one edit in six, so "Koinon" needs
// the same key and a peer name allows two or three edits; the closest runs win, then the
// longer names.
func (n *Names) Resolve(transcript string) ([]string, []Match) {
	words := Normalize(transcript)
	type candidate struct {
		start, end, dist int
		name             *name
	}
	var cands []candidate
	for i := range n.names {
		nm := &n.names[i]
		limit := len(nm.sound) / 6
		for start := range words {
			for end := start + 1; end <= len(words) && end-start <= len(nm.words)+2; end++ {
				heard := strings.Join(words[start:end], "")
				if d := distance(sound(heard), nm.sound); d <= limit {
					cands = append(cands, candidate{start, end, d, nm})
				}
			}
		}
	}
	sort.SliceStable(cands, func(a, b int) bool {
		if cands[a].dist != cands[b].dist {
			return cands[a].dist < cands[b].dist
		}
		if len(cands[a].name.words) != len(cands[b].name.words) {
			return len(cands[a].name.words) > len(cands[b].name.words)
		}
		return cands[a].end-cands[a].start < cands[b].end-cands[b].start
	})
	taken := make([]bool, len(words))
	var matches []Match
next:
	for _, c := range cands {
		for i := c.start; i < c.end; i++ {
			if taken[i] {
				continue next
			}
		}
		for i := c.start; i < c.end; i++ {
			taken[i] = true
		}
		heard := strings.Join(words[c.start:c.end], " ")
		if resolved := strings.Join(c.name.words, " "); heard != resolved {
			matches = append(matches, Match{Start: c.start, End: c.end, Heard: heard, Name: resolved})
		}
	}
	sort.Slice(matches, func(a, b int) bool { return matches[a].Start < matches[b].Start })
	out := make([]string, 0, len(words))
	at := 0
	for _, m := range matches {
		out = append(out, words[at:m.Start]...)
		out = append(out, strings.Fields(m.Name)...)
		at = m.End
	}
	return append(out, words[at:]...), matches
}

// sound is a rough key for how a word is said in English: c, k and q sound alike, a run of
// vowels is one vowel, a doubled letter is one, and silent or doubled spellings ("ph",
// "ck", "gh") are rewritten. "Koinon", "Quinan", "Coynan" and "coin in" all give "kanan".
func sound(w string) string {
	for _, r := range []struct{ from, to string }{
		{"ph", "f"}, {"qu", "k"}, {"ck", "k"}, {"gh", ""}, {"wh", "w"}, {"x", "ks"},
	} {
		w = strings.ReplaceAll(w, r.from, r.to)
	}
	var b strings.Builder
	var last byte
	for i := 0; i < len(w); i++ {
		c := w[i]
		switch c {
		case 'c', 'q':
			c = 'k'
		case 'z':
			c = 's'
		case 'e', 'i', 'o', 'u', 'y':
			c = 'a'
		}
		if c == last && (c < '0' || c > '9') {
			continue
		}
		b.WriteByte(c)
		last = c
	}
	return b.String()
}

// distance is the Levenshtein distance between two strings of bytes.
func distance(a, b string) int {
	prev := make([]int, len(b)+1)
	cur := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		cur[0] = i
		for j := 1; j <= len(b); j++ {
			sub := prev[j-1]
			if a[i-1] != b[j-1] {
				sub++
			}
			cur[j] = min(sub, prev[j]+1, cur[j-1]+1)
		}
		prev, cur = cur, prev
	}
	return prev[len(b)]
}
