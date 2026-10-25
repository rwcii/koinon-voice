package speech

import (
	"strings"
	"testing"
)

var testNames = []string{"Koinon", "codex-koinon-voice-af", "claude koinon voice b 9", "deepseek koinon 7 c", "koinon voice"}

func TestSound(t *testing.T) {
	for _, w := range []string{"koinon", "quinan", "coynan", "coinin", "quinnan"} {
		if got := sound(w); got != "kanan" {
			t.Errorf("sound(%q) = %q", w, got)
		}
	}
	if sound("b99") != "b99" || sound("phone") != "fana" {
		t.Errorf("sound: %q %q", sound("b99"), sound("phone"))
	}
	if distance("kanan", "karnan") != 1 || distance("", "abc") != 3 || distance("abc", "abc") != 0 {
		t.Error("distance")
	}
}

func TestResolve(t *testing.T) {
	n := NewNames(testNames)
	for _, c := range []struct{ heard, want, matches string }{
		{"Quinan, what's waiting?", "koinon whats waiting", "quinan → koinon"},
		{"Coin in, read my inbox.", "koinon read my inbox", "coin in → koinon"},
		{"Merge pull request 5 on coin invoice.", "merge pull request 5 on koinon voice", "coin invoice → koinon voice"},
		{"Tell deep sea coinin 7c to wait.", "tell deepseek koinon 7 c to wait", "deep sea coinin 7 c → deepseek koinon 7 c"},
		{"What did Codex Quinn invoice AF ask?", "what did codex koinon voice af ask", "codex quinn invoice af → codex koinon voice af"},
		// A name heard right inside a longer misheard one, and a right name before a word.
		{"Tell deep sea koinon 7c to wait.", "tell deepseek koinon 7 c to wait", "deep sea koinon 7 c → deepseek koinon 7 c"},
		{"Tell deepseek koinon 7c to wait.", "tell deepseek koinon 7 c to wait", ""},
		// A name already right is not a match, and ordinary words stay.
		{"Koinon, approve it.", "koinon approve it", ""},
		{"Approve pull request twelve.", "approve pull request 12", ""},
		// The bare word sounds like ordinary words; the bench's held-out check counts them.
		{"Fire the cannon.", "fire the koinon", "cannon → koinon"},
	} {
		words, ms := n.Resolve(c.heard)
		for _, m := range ms {
			if got := strings.Join(words[m.At:m.At+len(strings.Fields(m.Name))], " "); got != m.Name {
				t.Errorf("%q: match %+v at %q", c.heard, m, got)
			}
		}
		var got []string
		for _, m := range ms {
			got = append(got, m.Heard+" → "+m.Name)
		}
		if strings.Join(words, " ") != c.want || strings.Join(got, "; ") != c.matches {
			t.Errorf("%q: %q %q, want %q %q", c.heard, strings.Join(words, " "), got, c.want, c.matches)
		}
	}
}

// A name heard right stays itself when another name sounds the same, and a run two names fit
// equally well is left as heard, whatever the names' order (peer review F1).
func TestResolveAmbiguous(t *testing.T) {
	for _, names := range [][]string{
		{"codex-koinon-voice-af", "codex-koinon-voice-ef"},
		{"codex-koinon-voice-ef", "codex-koinon-voice-af"},
	} {
		n := NewNames(names)
		for heard, want := range map[string]string{
			"Tell codex-koinon-voice-ef to wait.":  "tell codex koinon voice ef to wait",
			"Tell codex koinon voice AF to wait.":  "tell codex koinon voice af to wait",
			"Tell codex coin invoice if to wait.":  "tell codex coin invoice if to wait",
			"Tell codex coin invoice e f to wait.": "tell codex koinon voice ef to wait",
		} {
			if words, _ := n.Resolve(heard); strings.Join(words, " ") != want {
				t.Errorf("%v: %q resolved to %q, want %q", names, heard, strings.Join(words, " "), want)
			}
		}
	}
}

func TestResolveNamesAndReport(t *testing.T) {
	phrases, err := ParsePhrases(strings.NewReader("01. [Koinon], what's waiting?\n02. Fire at noon.\n"))
	if err != nil {
		t.Fatal(err)
	}
	card := "## Transcripts\n\n| Phrase | Set | Engine | Errors | Term errors | Latency | Transcript |\n| --- | --- | --- | --- | --- | --- | --- |\n" +
		"| 01 | clean | e | 1/3 | 1/1 | 9 ms | Quinan, what's waiting? |\n" +
		"| 02 | clean | e | 1/3 | 0/0 | 9 ms | Fire a cannon \\| noon. |\n" +
		"| 02 | noisy | e | 3/3 | 0/0 | 9 ms | failed: timeout |\n"
	ts, err := ParseTranscripts(strings.NewReader(card), phrases)
	if err != nil {
		t.Fatal(err)
	}
	if len(ts) != 2 || ts[1].Text != "Fire a cannon | noon." || ts[0].Score.TermErrors != 1 {
		t.Fatalf("transcripts %+v", ts)
	}
	rs := ResolveNames(ts, NewNames(testNames))
	if rs[0].After.Errors != 0 || len(rs[0].Wrong) != 0 || len(rs[1].Wrong) != 1 || rs[1].After.Errors != rs[1].Score.Errors {
		t.Errorf("results %+v", rs)
	}
	o := ResolveOrdinary("near", "Fire the cannon.\n\nCome along.\n", NewNames(testNames))
	if o.Lines != 2 || len(o.Matched) != 1 || o.Matched[0] != "Fire the cannon.: cannon → koinon" {
		t.Errorf("ordinary %+v", o)
	}
	var b strings.Builder
	WriteNamesReport(&b, rs, testNames, []Ordinary{o})
	for _, want := range []string{
		"| e | clean | 2 | 50.0% (3/6) | 33.3% (2/6) | 100.0% (1/1) | 0.0% (0/1) | 2 | 1 | 0 |",
		"| near | 2 | 1 | Fire the cannon.: cannon → koinon |",
		"| 01 | clean | e | 1 to 0 | 1 to 0 | quinan → koinon | koinon whats waiting |",
		"cannon → koinon (wrong)",
	} {
		if !strings.Contains(b.String(), want) {
			t.Errorf("report lacks %q:\n%s", want, b.String())
		}
	}
	if _, err := ParseTranscripts(strings.NewReader("| 09 | clean | e | 1/3 | 0/0 | 9 ms | x |\n"), phrases); err == nil {
		t.Error("a transcript of an unknown phrase")
	}
	if _, err := ParseTranscripts(strings.NewReader("nothing\n"), phrases); err == nil {
		t.Error("no transcripts")
	}
}

// A wrong match is judged where it lies in the phrase, not by whether the name is elsewhere
// in it (peer review F2).
func TestWrongMatchAligned(t *testing.T) {
	phrases, err := ParsePhrases(strings.NewReader("01. [Koinon], fire the cannon.\n02. Ask [codex-koinon-voice-af] about [Koinon].\n"))
	if err != nil {
		t.Fatal(err)
	}
	n := NewNames(testNames)
	for _, c := range []struct {
		phrase       int
		heard        string
		wrong, right int
	}{
		{0, "Koinon, fire the cannon.", 1, 0},
		{0, "Quinan, fire the cannon.", 1, 1},
		{1, "Ask codex coin invoice AF about Quinan.", 0, 2},
	} {
		p := phrases[c.phrase]
		r := ResolveNames([]Transcript{{Phrase: p, Text: c.heard, Score: Compare(p, c.heard)}}, n)[0]
		if len(r.Wrong) != c.wrong || len(r.Matches)-len(r.Wrong) != c.right {
			t.Errorf("%q: matches %+v wrong %+v", c.heard, r.Matches, r.Wrong)
		}
	}
}
