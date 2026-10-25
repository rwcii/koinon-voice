package speech

import (
	"slices"
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
	At         int // where the name starts in the resolved words
	Heard      string
	Name       string
}

// Resolve returns the transcript's normalized words with each run that sounds like a name
// replaced by the name, and the runs it replaced. A run that already is a name stays that
// name. Otherwise a run of up to two words more than the name sounds like it when their sound
// keys differ by at most one edit in six, so "Koinon" needs the same key and a peer name
// allows two or three edits; the closest runs win, then the longer names. When two names
// sound equally close to overlapping runs, the closer spelling wins, and when that ties too,
// as "codex koinon voice af" and "... ef" do for "... if", neither is chosen and the words
// stay as heard.
func (n *Names) Resolve(transcript string) ([]string, []Match) {
	words := Normalize(transcript)
	type candidate struct {
		start, end, dist int // dist -1: the run is the name
		spelling         int // edit distance of the letters, for ties of sound
		name             *name
	}
	var cands []candidate
	for i := range n.names {
		nm := &n.names[i]
		limit := len(nm.sound) / 6
		for start := range words {
			for end := start + 1; end <= len(words) && end-start <= len(nm.words)+2; end++ {
				heard := strings.Join(words[start:end], "")
				if slices.Equal(words[start:end], nm.words) {
					cands = append(cands, candidate{start, end, -1, 0, nm})
				} else if d := distance(sound(heard), nm.sound); d <= limit {
					cands = append(cands, candidate{start, end, d, distance(heard, strings.Join(nm.words, "")), nm})
				}
			}
		}
	}
	tier := func(c candidate) [2]int { return [2]int{c.dist, -len(c.name.words)} }
	sort.SliceStable(cands, func(a, b int) bool {
		if ta, tb := tier(cands[a]), tier(cands[b]); ta != tb {
			return ta[0] < tb[0] || ta[0] == tb[0] && ta[1] < tb[1]
		}
		return cands[a].end-cands[a].start < cands[b].end-cands[b].start
	})
	// A run that is a name is kept as it is, longer names first. Another name may replace a
	// kept run only by covering it and holding it whole, as "deepseek koinon 7 c" holds
	// "koinon"; so "codex koinon voice ef", heard right, never becomes "... af".
	type span struct {
		start, end int
		name       *name
	}
	kept := make([]*span, len(words))
	var rest []candidate
	for _, c := range cands {
		if c.dist >= 0 {
			rest = append(rest, c)
			continue
		}
		if !slices.ContainsFunc(kept[c.start:c.end], func(k *span) bool { return k != nil }) {
			k := &span{c.start, c.end, c.name}
			for i := c.start; i < c.end; i++ {
				kept[i] = k
			}
		}
	}
	cands = rest
	taken := make([]bool, len(words))
	free := func(c candidate) bool {
		for i := c.start; i < c.end; i++ {
			if taken[i] {
				return false
			}
			if k := kept[i]; k != nil && (k.start < c.start || k.end > c.end || len(c.name.words) <= len(k.name.words) || !holds(c.name.words, k.name.words)) {
				return false
			}
		}
		return true
	}
	var matches []Match
	for lo := 0; lo < len(cands); {
		hi := lo
		for hi < len(cands) && tier(cands[hi]) == tier(cands[lo]) {
			hi++
		}
		// Within one tier, a run that another name overlaps with a closer spelling loses to it,
		// and one that another name fits as well is ambiguous.
		group := cands[lo:hi]
		ambiguous := make([]bool, len(group))
		beaten := make([]bool, len(group))
		for i, a := range group {
			for _, b := range group {
				if a.name == b.name || a.start >= b.end || b.start >= a.end || !free(a) || !free(b) {
					continue
				}
				switch {
				case b.spelling < a.spelling:
					beaten[i] = true
				case b.spelling == a.spelling:
					ambiguous[i] = true
				}
			}
		}
		for i, c := range group {
			if ambiguous[i] || beaten[i] || !free(c) {
				continue
			}
			for j := c.start; j < c.end; j++ {
				taken[j] = true
			}
			heard := strings.Join(words[c.start:c.end], " ")
			if resolved := strings.Join(c.name.words, " "); heard != resolved {
				matches = append(matches, Match{Start: c.start, End: c.end, Heard: heard, Name: resolved})
			}
		}
		for i, c := range group {
			if ambiguous[i] {
				for j := c.start; j < c.end; j++ {
					taken[j] = true
				}
			}
		}
		lo = hi
	}
	sort.Slice(matches, func(a, b int) bool { return matches[a].Start < matches[b].Start })
	out := make([]string, 0, len(words))
	at := 0
	for i, m := range matches {
		out = append(out, words[at:m.Start]...)
		matches[i].At = len(out)
		out = append(out, strings.Fields(m.Name)...)
		at = m.End
	}
	return append(out, words[at:]...), matches
}

// holds reports whether part is a run of whole's words.
func holds(whole, part []string) bool {
	for i := 0; i+len(part) <= len(whole); i++ {
		if slices.Equal(whole[i:i+len(part)], part) {
			return true
		}
	}
	return false
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
