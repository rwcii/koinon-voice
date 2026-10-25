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
		// A name already right is not a match, and ordinary words stay.
		{"Koinon, approve it.", "koinon approve it", ""},
		{"Approve pull request twelve.", "approve pull request 12", ""},
		// The bare word sounds like ordinary words; the bench's held-out check counts them.
		{"Fire the cannon.", "fire the koinon", "cannon → koinon"},
	} {
		words, ms := n.Resolve(c.heard)
		var got []string
		for _, m := range ms {
			got = append(got, m.Heard+" → "+m.Name)
		}
		if strings.Join(words, " ") != c.want || strings.Join(got, "; ") != c.matches {
			t.Errorf("%q: %q %q, want %q %q", c.heard, strings.Join(words, " "), got, c.want, c.matches)
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
