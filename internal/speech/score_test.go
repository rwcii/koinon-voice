package speech

import (
	"os"
	"strings"
	"testing"
)

func TestNormalize(t *testing.T) {
	for in, want := range map[string]string{
		"Koinon, what's waiting?":                  "koinon whats waiting",
		"codex-koinon-voice-af":                    "codex koinon voice af",
		"Codex Koinon voice A F.":                  "codex koinon voice af",
		"claude-koinon-voice-b9":                   "claude koinon voice b 9",
		"Claude Koinon voice B nine":               "claude koinon voice b 9",
		"deepseek-koinon-7c":                       "deepseek koinon 7 c",
		"Reject pull request thirty-eight":         "reject pull request 38",
		"reject PR 38":                             "reject pr 38",
		"issue four sixty-four":                    "issue 464",
		"issue four hundred and sixty-four":        "issue 464",
		"issue 464":                                "issue 464",
		"rwcii/koinon":                             "rwcii koinon",
		"R W C I I slash Koinon":                   "rwcii slash koinon",
		"Send it a message":                        "send it a message",
		"Don’t merge yet.":                         "dont merge yet",
		"Choose option two.":                       "choose option 2",
		"Remind me in thirty minutes.":             "remind me in 30 minutes",
		"Is oMLX running? O M L X":                 "is omlx running omlx",
		"twelve":                                   "12",
		"Summarise the decision on issue four, 64": "summarize the decision on issue 464",
		"Approved 1st option.":                     "approved first option",
		"Koinon, good night.":                      "koinon goodnight",
		"chunk 4 a message":                        "chunk 4 a message",
	} {
		if got := strings.Join(Normalize(in), " "); got != want {
			t.Errorf("Normalize(%q) = %q, want %q", in, got, want)
		}
	}
}

func phrase(t *testing.T, s string) Phrase {
	t.Helper()
	p, err := parsePhrase(1, s)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestCompare(t *testing.T) {
	for _, c := range []struct {
		ref, hyp                 string
		words, errs, terms, terr int
	}{
		{"[Approve] pull request [twelve].", "Approve pull request 12.", 4, 0, 2, 0},
		{"[Approve] pull request [twelve].", "Approved pull request 120.", 4, 2, 2, 2},
		// "Koinon" heard as "Quinan" is one substitution on a term.
		{"[Koinon], read my inbox.", "Quinan, read my inbox.", 4, 1, 1, 1},
		// A hyphenated peer name is four words, each a term.
		{"What did [codex-koinon-voice-af] ask?", "What did Codex Koinon voice AF ask?", 7, 0, 4, 0},
		{"What did [codex-koinon-voice-af] ask?", "What did codecs coin on voice ask?", 7, 4, 4, 4},
		// A deleted term, and an inserted word after a term that is not the term's error.
		{"[Confirm], [merge] pull request [five].", "Confirm pull request five please.", 5, 2, 3, 1},
		// Nothing heard: every word is a deletion.
		{"[Koinon], goodnight.", "", 2, 2, 1, 1},
		// Words around a term are word errors but not the term's.
		{"[Approve] it.", "uh yes approve it", 2, 2, 1, 0},
		// A word inside a term is the term's error.
		{"Ask [codex-koinon-voice-af].", "ask codex koinon voice x af", 5, 1, 4, 1},
	} {
		s := Compare(phrase(t, c.ref), c.hyp)
		if s.Words != c.words || s.Errors != c.errs || s.TermWords != c.terms || s.TermErrors != c.terr {
			t.Errorf("%q vs %q: %+v, want words %d errors %d terms %d term errors %d", c.ref, c.hyp, s, c.words, c.errs, c.terms, c.terr)
		}
	}
	var total Score
	total.Add(Score{Words: 4, Errors: 1, TermWords: 2, TermErrors: 1})
	total.Add(Score{Words: 6, Errors: 2, TermWords: 2})
	if total.WER() != 0.3 || total.TermWER() != 0.25 || (Score{}).WER() != 0 {
		t.Errorf("rates %+v: %v %v", total, total.WER(), total.TermWER())
	}
}

func TestParsePhrases(t *testing.T) {
	ps, err := ParsePhrases(strings.NewReader("# Title\n\n01. [Koinon], what's waiting?\nprose 2. not a phrase\n02. Stop.\n"))
	if err != nil {
		t.Fatal(err)
	}
	if len(ps) != 2 || ps[0].N != 1 || ps[0].Text != "Koinon, what's waiting?" || !ps[0].Terms[0] || ps[0].Terms[1] || ps[1].Text != "Stop." {
		t.Errorf("%+v", ps)
	}
	for _, bad := range []string{"01. a\n01. b\n", "01. [open\n", "no phrases\n", "01. ...\n"} {
		if _, err := ParsePhrases(strings.NewReader(bad)); err == nil {
			t.Errorf("%q parsed", bad)
		}
	}
}

// The committed list parses: about 60 phrases with Koinon terms among them.
func TestCommittedPhrases(t *testing.T) {
	f, err := os.Open("../../docs/sprints/2026-10-23-phase0-spike/corpus-phrases.md")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	ps, err := ParsePhrases(f)
	if err != nil {
		t.Fatal(err)
	}
	terms := 0
	for i, p := range ps {
		if p.N != i+1 {
			t.Errorf("phrase %d is numbered %d", i+1, p.N)
		}
		for _, term := range p.Terms {
			if term {
				terms++
			}
		}
	}
	if len(ps) < 55 || terms < 60 {
		t.Errorf("%d phrases, %d term words", len(ps), terms)
	}
}

func TestKeywords(t *testing.T) {
	raw := "01. Ask [codex-koinon-voice-af] about [rwcii] slash [koinon].\n02. [Reject] pull request [thirty-eight] on [koinon-voice].\n03. Tell [codex-koinon-voice-af].\n"
	got := strings.Join(Keywords(raw), ", ")
	if want := "Koinon, codex koinon voice af, koinon voice, rwcii"; got != want {
		t.Errorf("Keywords = %q, want %q", got, want)
	}
}
