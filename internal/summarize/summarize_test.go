package summarize

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPrompt(t *testing.T) {
	p := Prompt(Item{
		Title: "Revert a commit", Kind: "decision", Where: "issue 4", AgentText: "It broke CI.",
		Context: "A forward fix takes two hours.", Recommendation: "Revert.", Commands: []string{"git revert abc"},
		AuthorFamily: "codex", AuthorName: "codex-koinon-voice-38",
	})
	for _, want := range []string{
		"Asker: codex-koinon-voice-38 (codex)", "Title: Revert a commit", "Where: issue 4",
		"Asker's text: It broke CI.", "Context: A forward fix", "Recommendation: Revert.", "Command: git revert abc",
	} {
		if !strings.Contains(p, want) {
			t.Errorf("prompt lacks %q:\n%s", want, p)
		}
	}
	if strings.Contains(Prompt(Item{Title: "t"}), "Recommendation") {
		t.Error("empty recommendation rendered")
	}
}

// TestFixtures checks the definition of done's fixture coverage.
func TestFixtures(t *testing.T) {
	fixtures, err := LoadFixtures("testdata/items")
	if err != nil {
		t.Fatal(err)
	}
	if len(fixtures) < 20 {
		t.Fatalf("%d fixtures, want at least 20", len(fixtures))
	}
	var short, long, manyOptions, commands, links, deadline, noDeadline, noRecommendation int
	for _, f := range fixtures {
		k := f.Key
		if len(k.Asker) == 0 || len(k.Ask) == 0 {
			t.Errorf("%s: answer key has no asker or ask", f.Name)
		}
		if (f.Item.Recommendation == "") != (len(k.Recommendation) == 0) {
			t.Errorf("%s: recommendation key does not match the item", f.Name)
		}
		size := len(f.Item.AgentText + f.Item.Context)
		switch {
		case size < 120:
			short++
		case size > 250:
			long++
		}
		if len(k.Options) >= 3 {
			manyOptions++
		}
		if len(f.Item.Commands) > 0 {
			commands++
		}
		if strings.Contains(f.Item.AgentText+f.Item.Context, "https://") {
			links++
		}
		if len(k.Deadline) > 0 {
			deadline++
		} else {
			noDeadline++
		}
		if len(k.Recommendation) == 0 {
			noRecommendation++
		}
	}
	for name, n := range map[string]int{
		"short": short, "long": long, "three or more options": manyOptions, "commands": commands,
		"links": links, "deadline": deadline, "no deadline": noDeadline, "no recommendation": noRecommendation,
	} {
		if n == 0 {
			t.Errorf("no fixture with %s", name)
		}
	}
}

func TestLoadItem(t *testing.T) {
	dir := t.TempDir()
	write := func(name, body string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		return p
	}
	bare, err := LoadItem(write("bare.json", `{"reference":"R-1","title":"Bare item","context":"c"}`))
	if err != nil || bare.Name != "R-1" || bare.Item.Context != "c" || bare.Key != nil {
		t.Errorf("bare %+v %v", bare, err)
	}
	wrapped, err := LoadItem(write("wrapped.json", `{"item":{"title":"Wrapped","recommendation":"r"}}`))
	if err != nil || wrapped.Name != "wrapped" || wrapped.Item.Recommendation != "r" {
		t.Errorf("wrapped %+v %v", wrapped, err)
	}
	if _, err := LoadItem(write("untitled.json", `{"context":"c"}`)); err == nil {
		t.Error("untitled item loaded")
	}
	if _, err := LoadItem(write("junk.json", `{`)); err == nil {
		t.Error("junk loaded")
	}
	if _, err := LoadFixtures(dir); err == nil {
		t.Error("fixtures without keys loaded")
	}
}

func TestCheck(t *testing.T) {
	key := &Key{
		Asker:          Field{{"codex"}},
		Ask:            Field{{"merge"}, {"12", "twelve"}},
		Options:        Field{{"merge"}, {"wait"}},
		Recommendation: Field{{"merge"}},
	}
	cases := []struct {
		text string
		want []Verdict
	}{
		{"Codex asks you to merge pull request twelve, or wait. It recommends merging.",
			[]Verdict{Found, Found, Found, Found, Absent}},
		{"Codex asks you to merge pull request 12. Merge or wait. It's due by Friday.",
			[]Verdict{Found, Found, Found, Found, Invented}},
		{"Codex asks you to merge PR 12. There's no deadline.",
			[]Verdict{Found, Found, Missing, Found, Absent}},
		{"Codex asks you to merge twelve, or keep waiting. It recommends merging.",
			[]Verdict{Found, Found, Found, Found, Absent}},
		{"Claude asks about a merge.",
			[]Verdict{Missing, Missing, Missing, Found, Absent}},
	}
	for _, c := range cases {
		got := Check(c.text, key)
		if strings.Join(verdicts(got), ",") != strings.Join(verdicts(c.want), ",") {
			t.Errorf("%q: %v, want %v", c.text, got, c.want)
		}
	}

	none := &Key{Asker: Field{{"codex"}}, Ask: Field{{"voice"}}}
	for text, want := range map[string]Verdict{
		"Codex asks you to pick a voice.":                                                            Absent,
		"Codex asks you to pick a voice and recommends Ava.":                                         Invented,
		"Codex asks you to pick a voice. It makes no recommendation.":                                Absent,
		"Codex asks you to pick a voice. Codex doesn't recommend either.":                            Absent,
		"Codex asks for voices. No recommendation or deadline was given.":                            Absent,
		"Ava starts without a delay. Codex recommends Ava.":                                          Invented,
		"Codex has no deadline; it recommends Ava.":                                                  Invented,
		"Claude asks you to choose a voice that does not require a paid licence and recommends Ava.": Invented,
		"Codex gives no deadline but recommends Ava.":                                                Invented,
		"Codex asks you to pick a voice and does not give a recommendation.":                         Absent,
		"There's no recommendation and no deadline given.":                                           Absent,
		"Codex asks you to pick a voice. A recommendation is not given.":                             Absent,
		"Codex asks you to pick a voice and recommends not waiting.":                                 Invented,
		"Codex asks you to merge. The recommendation is not to wait.":                                Invented,
		"The recommendation is unclear.":                                                             Absent,
	} {
		if got := Check(text, none)[3]; got != want {
			t.Errorf("%q recommendation %v, want %v", text, got, want)
		}
	}
	for text, want := range map[string]Verdict{
		"Codex asks you to pick a voice.":                                         Absent,
		"Codex needs a voice by tomorrow.":                                        Invented,
		"Codex needs a voice before 5 pm.":                                        Invented,
		"Codex asks you to pick a voice; no deadline.":                            Absent,
		"Codex asks for voices. It does not give a recommendation or a deadline.": Absent,
		"Codex asks for voices. No recommendation or deadline was given.":         Absent,
		"Codex makes no recommendation. It is due by Friday.":                     Invented,
		"Codex needs a voice that does not cost money, due by Friday.":            Invented,
		"Codex has no preference and needs a voice by Friday.":                    Invented,
		"Neither a recommendation nor a deadline is given.":                       Absent,
		"Codex asks you to pick a voice. Deadline is not specified.":              Absent,
		"The deadline is unknown, and Codex recommends Ava.":                      Absent,
		"The deadline has not been set.":                                          Absent,
		"The deadline is Friday, not Monday.":                                     Invented,
		"The deadline is not until Friday.":                                       Invented,
		"The deadline is not Monday but Friday.":                                  Invented,
		"The deadline has not yet been set, but Codex recommends Ava.":            Absent,
		"Codex asks you to upgrade due to a security advisory.":                   Absent,
		"The review is due Friday.":                                               Invented,
		"Codex needs it due tomorrow; it gives no recommendation.":                Invented,
	} {
		if got := Check(text, none)[4]; got != want {
			t.Errorf("%q deadline %v, want %v", text, got, want)
		}
	}
	if !Correct([]Verdict{Found, Absent}) || Correct([]Verdict{Found, Missing}) || Correct(Check("x", nil)) {
		t.Error("Correct")
	}
}

func verdicts(vs []Verdict) []string {
	out := make([]string, len(vs))
	for i, v := range vs {
		out[i] = string(v)
	}
	return out
}

func TestLoadEngines(t *testing.T) {
	config, err := LoadConfig("testdata/engines.toml")
	if err != nil {
		t.Fatal(err)
	}
	rows := config.Engine
	if len(rows) != 7 || rows[0].Model != "claude-haiku-5-5" || !rows[5].Local || rows[2].InputPrice != 4 || !rows[4].DenyDataCollection {
		t.Errorf("rows %+v", rows)
	}
	if !strings.HasSuffix(config.Helper, "/koinon-voice-apple") || config.Voice != "com.apple.voice.enhanced.en-US.Evan" || rows[6].Adapter != "apple" || !rows[6].Local {
		t.Errorf("helper %q, apple row %+v", config.Helper, rows[6])
	}
	dir := t.TempDir()
	for name, body := range map[string]string{
		"empty":        ``,
		"no name":      "[[engine]]\nadapter = \"openai\"\nmodel = \"m\"\nbase_url = \"u\"\nlocal = true\n",
		"bad adapter":  "[[engine]]\nname = \"a\"\nadapter = \"grpc\"\nmodel = \"m\"\nlocal = true\n",
		"no base":      "[[engine]]\nname = \"a\"\nadapter = \"openai\"\nmodel = \"m\"\nlocal = true\n",
		"no provider":  "[[engine]]\nname = \"a\"\nadapter = \"anthropic\"\nmodel = \"m\"\n",
		"apple remote": "[[engine]]\nname = \"a\"\nadapter = \"apple\"\nmodel = \"m\"\nprovider = \"Apple\"\n",
		"unknown":      "[[engine]]\nname = \"a\"\nadapter = \"anthropic\"\nmodel = \"m\"\nprovider = \"p\"\ntemperature = 1\n",
		"duplicate":    "[[engine]]\nname = \"a\"\nadapter = \"anthropic\"\nmodel = \"m\"\nprovider = \"p\"\n[[engine]]\nname = \"a\"\nadapter = \"anthropic\"\nmodel = \"m\"\nprovider = \"p\"\n",
	} {
		p := filepath.Join(dir, strings.ReplaceAll(name, " ", "-")+".toml")
		if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := LoadEngines(p); err == nil {
			t.Errorf("%s: loaded", name)
		}
	}
}

// fakeSecurity writes a stand-in for macOS security that knows one item.
func fakeSecurity(t *testing.T) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "security")
	script := `#!/bin/sh
[ "$1" = find-generic-password ] && [ "$2" = -s ] && [ "$4" = -w ] || exit 2
case "$3" in
koinon-voice-anthropic) echo "sk-test-anthropic" ;;
koinon-voice-blank) echo "" ;;
*) echo "security: SecKeychainSearchCopyNext: The specified item could not be found in the keychain." >&2; exit 44 ;;
esac
`
	if err := os.WriteFile(p, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestKeychain(t *testing.T) {
	k := Keychain{Command: fakeSecurity(t), GOOS: "darwin"}
	ctx := context.Background()
	if key, err := k.Key(ctx, "anthropic"); err != nil || key != "sk-test-anthropic" {
		t.Errorf("key %q %v", key, err)
	}
	_, err := k.Key(ctx, "openai")
	if err == nil || !strings.Contains(err.Error(), "koinon-voice-openai not found") {
		t.Errorf("missing: %v", err)
	}
	if _, err := k.Key(ctx, "blank"); err == nil || !strings.Contains(err.Error(), "empty") {
		t.Errorf("blank: %v", err)
	}
	if _, err := (Keychain{GOOS: "linux"}).Key(ctx, "anthropic"); err == nil || !strings.Contains(err.Error(), "macOS only") {
		t.Errorf("linux: %v", err)
	}
	if strings.Contains(err.Error(), "sk-") {
		t.Error("error carries a key")
	}
}

func TestBuildMissingKey(t *testing.T) {
	keys := Keychain{Command: fakeSecurity(t), GOOS: "darwin"}
	_, err := Build(context.Background(), Row{Name: "o", Adapter: "openai", BaseURL: "u", Model: "m", Key: "openai", Provider: "OpenAI"}, Env{Keys: keys})
	if err == nil || !strings.Contains(err.Error(), "no key for openai") {
		t.Errorf("error %v", err)
	}
	e, err := Build(context.Background(), Row{Name: "a", Adapter: "anthropic", Model: "m", Key: "anthropic", Provider: "Anthropic", Effort: "low"}, Env{Keys: keys})
	if a, ok := e.(*Anthropic); err != nil || !ok || a.Key != "sk-test-anthropic" || a.Effort != "low" {
		t.Errorf("anthropic %+v %v", e, err)
	}
	if e, err := Build(context.Background(), Row{Name: "l", Adapter: "openai", BaseURL: "u", Model: "m", Local: true}, Env{Keys: keys}); err != nil || e.(*OpenAI).Key != "" {
		t.Errorf("local %+v %v", e, err)
	}
}

type stubEngine struct {
	text string
	err  error
}

func (s stubEngine) Summarize(ctx context.Context, item Item) (Summary, error) {
	return s.Stream(ctx, item, nil)
}

func (s stubEngine) Stream(ctx context.Context, item Item, onText func(string)) (Summary, error) {
	return Summary{Text: s.text, FirstToken: time.Millisecond, FirstSentence: 2 * time.Millisecond, Total: 3 * time.Millisecond}, s.err
}

func TestRunAndScorecard(t *testing.T) {
	rows := []Row{{Name: "good", Adapter: "openai", Model: "m", Local: true}, {Name: "down", Adapter: "openai", Model: "m", Provider: "OpenRouter"}, {Name: "nokey", Adapter: "anthropic", Model: "m", Provider: "Anthropic"}}
	engines := []Configured{
		{Row: rows[0], Engine: stubEngine{text: "Codex asks you to merge twelve | now."}},
		{Row: rows[1], Engine: stubEngine{err: fmt.Errorf("HTTP 503: down for sk-ant-api03-secretvalue: %w", ErrEmpty)}},
		{Row: rows[2], Err: errors.New("no key for anthropic")},
	}
	items := []Fixture{
		{Name: "one", Item: testItem, Key: &Key{Asker: Field{{"codex"}}, Ask: Field{{"merge"}}}},
		{Name: "two", Item: testItem},
	}
	results, _ := Run(context.Background(), engines, items, Options{Timeout: time.Second})
	if len(results) != 6 {
		t.Fatalf("%d results", len(results))
	}
	if results[0].Err != nil || !Correct(results[0].Verdicts) || results[1].Err == nil || results[2].Err == nil || results[3].Err != nil {
		t.Errorf("results %+v", results)
	}
	if !errors.Is(results[1].Err, ErrEmpty) || strings.Contains(results[1].Err.Error(), "secretvalue") {
		t.Errorf("engine error %v", results[1].Err)
	}
	if results[3].Verdicts[0] != Review {
		t.Errorf("unkeyed verdicts %v", results[3].Verdicts)
	}
	results[0].Rating = 4

	var b strings.Builder
	if err := WriteScorecard(&b, results, rows); err != nil {
		t.Fatal(err)
	}
	out := b.String()
	for _, want := range []string{
		"| good | openai | m | local |", "| down | openai | m | external: OpenRouter |",
		"| good | 2 | 0 | 1/2 | 0 |  | 1 ms | 2 ms |  | 3 ms | $0.000000 | 4.0 (1) |",
		"| down | 2 | 2 | 0/0 |", "failed: HTTP 503: down for [key redacted]", "failed: no key for anthropic",
		`merge twelve \| now.`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("scorecard lacks %q:\n%s", want, out)
		}
	}
	rowsTable := out[strings.Index(out, "| Item "):]
	for _, line := range strings.Split(strings.TrimSpace(rowsTable), "\n") {
		// 17 columns: item, engine, five verdicts, nine measures, not for speech.
		if n := strings.Count(strings.ReplaceAll(line, `\|`, ""), "|"); n != 18 {
			t.Errorf("%d separators in %q", n, line)
		}
	}
}

func TestNotForSpeech(t *testing.T) {
	for text, want := range map[string]string{
		"Codex asks you to merge pull request twelve.":                      "",
		"Revert commit 9f3c2ab on develop.":                                 "hash",
		"Revert commit nine f three c two a b. The deadline is 2026.":       "",
		"Install it in ~/.local/share/koinon-voice/bin.":                    "path",
		"Upgrade golang.org/x/net, which affects its HTTP/2 server.":        "path",
		"Cite it in AGENTS.md, and/or the README.":                          "path",
		"See https://github.com/rwcii/koinon/issues/464.":                   "url",
		"The options are: * Fail without a retry. * Retry.":                 "markdown",
		"Run `go test` first.":                                              "markdown",
		"# Decision\nMerge 4f2e9a1c at /tmp/x, per http://example.com/a/b.": "hash, path, url, markdown",
	} {
		if got := strings.Join(NotForSpeech(text), ", "); got != want {
			t.Errorf("%q: %q, want %q", text, got, want)
		}
	}
}

func TestRescore(t *testing.T) {
	fixtures := []Fixture{{Name: "SYN-01", Key: &Key{Asker: Field{{"codex"}}, Ask: Field{{"merge"}}}}}
	// A scorecard written before "Not for speech" existed, with a verdict the current check
	// changes, a failed row and a summary holding an escaped pipe.
	old := "# Scorecard\n\nProse stays.\n\n## Totals\n\n" +
		"| Engine | Rows | Failed | Correct | Median first spoken word | Median first token | Median first sentence | Median synthesis first audio | Median total | Cost | Mean rating |\n" +
		"|" + strings.Repeat(" --- |", 11) + "\n" +
		"| a | 2 | 0 | 0/2 | 1 ms | 1 ms | 1 ms | 1 ms | 1 ms | $0.000000 |  |\n" +
		"| b | 1 | 1 | 0/0 |  |  |  |  |  | unknown |  |\n" +
		"\n## Rows\n\n" +
		"| Item | Engine | asker | ask | options | recommendation | deadline | First spoken word | First token | First sentence | Synthesis first audio | Total | Tokens in/out | Cost | Rating | Summary |\n" +
		"|" + strings.Repeat(" --- |", 16) + "\n" +
		"| SYN-01 | a | ok | ok | none | none | invented | 1 ms | 1 ms | 1 ms | 1 ms | 1 ms | 1/1 | $0.000000 | 4 | Codex asks you to merge \\| now. Deadline is not specified. |\n" +
		"| SYN-01 | a | ok | ok | none | none | none | 1 ms | 1 ms | 1 ms | 1 ms | 1 ms | 1/1 | $0.000000 |  | Codex asks you to merge 9f3c2ab. |\n" +
		"| SYN-01 | b |  |  |  |  |  |  |  |  |  |  |  |  |  | failed: HTTP 503 |\n"
	got, changes, err := Rescore(old, fixtures)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(changes, ";") != "SYN-01 a deadline: invented to none" {
		t.Errorf("changes %q", changes)
	}
	for _, want := range []string{
		"# Scorecard\n\nProse stays.\n",
		"| Engine | Rows | Failed | Correct | Not for speech | Median first spoken word |",
		"| a | 2 | 0 | 2/2 | 1 | 1 ms |",
		"| b | 1 | 1 | 0/0 | 0 |  |",
		"| Rating | Not for speech | Summary |",
		"| 4 |  | Codex asks you to merge \\| now. Deadline is not specified. |",
		"| SYN-01 | a | ok | ok | none | none | none | 1 ms | 1 ms | 1 ms | 1 ms | 1 ms | 1/1 | $0.000000 |  | hash | Codex asks you to merge 9f3c2ab. |",
		"| SYN-01 | b |  |  |  |  |  |  |  |  |  |  |  |  |  |  | failed: HTTP 503 |",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("rescored lacks %q:\n%s", want, got)
		}
	}
	// Rescoring again changes nothing.
	again, changes, err := Rescore(got, fixtures)
	if err != nil || again != got || len(changes) != 0 {
		t.Errorf("second rescore: %v %q\n%s", err, changes, again)
	}
	if _, _, err := Rescore(old, nil); err == nil {
		t.Error("rescore without the fixture")
	}
}

// A scorecard written by WriteScorecard rescores unchanged, Markdown in a multiline summary
// included, and a malformed table is an error, not a panic.
func TestRescoreRoundTrip(t *testing.T) {
	key := &Key{Asker: Field{{"codex"}}, Ask: Field{{"merge"}}}
	rows := []Row{{Name: "test", Adapter: "openai", Model: "m", Local: true}}
	var results []Result
	for i, text := range []string{
		"Codex asks you to merge.\n- Merge now.\n- Wait until tomorrow.",
		"Codex asks you to decide\n- Merge now\n- Wait",
		"# Merge\nCodex asks you to merge.",
		"Codex asks you to merge 4f2e9a1c.",
	} {
		results = append(results, Result{Item: fmt.Sprintf("SYN-0%d", i+1), Row: rows[0], Summary: Summary{Text: text}, Verdicts: Check(text, key)})
	}
	var b strings.Builder
	if err := WriteScorecard(&b, results, rows); err != nil {
		t.Fatal(err)
	}
	card := b.String()
	if !strings.Contains(card, "| test | 4 | 0 | 4/4 | 4 |") || !strings.Contains(card, "| markdown | Codex asks you to decide<br>- Merge now<br>- Wait |") {
		t.Fatalf("scorecard:\n%s", card)
	}
	fixtures := []Fixture{{Name: "SYN-01", Key: key}, {Name: "SYN-02", Key: key}, {Name: "SYN-03", Key: key}, {Name: "SYN-04", Key: key}}
	got, changes, err := Rescore(card, fixtures)
	if err != nil || got != card || len(changes) != 0 {
		t.Errorf("rescore %v %q:\n%s", err, changes, got)
	}

	lines := strings.Split(card, "\n")
	find := func(prefix string) int {
		for i, l := range lines {
			if strings.HasPrefix(l, prefix) {
				return i
			}
		}
		t.Fatalf("no line %q", prefix)
		return 0
	}
	totalsHeader, totalsRow, rowsHeader := find("| Engine | Rows |"), find("| test | 4 |"), find("| Item |")
	for name, edit := range map[string]func([]string){
		"short totals row":   func(l []string) { l[totalsRow] = "| test |" },
		"short totals head":  func(l []string) { l[totalsHeader] = "| Engine |" },
		"short separator":    func(l []string) { l[totalsHeader+1] = "| --- |" },
		"no separator":       func(l []string) { l[rowsHeader+1] = strings.ReplaceAll(l[rowsHeader+1], "---", "x") },
		"short rows header":  func(l []string) { l[rowsHeader] = "| Item | Engine |" },
		"short rows content": func(l []string) { l[rowsHeader+2] = "| SYN-01 |" },
	} {
		bad := append([]string(nil), lines...)
		edit(bad)
		if _, _, err := Rescore(strings.Join(bad, "\n"), fixtures); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
}
