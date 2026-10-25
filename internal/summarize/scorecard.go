package summarize

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"time"
)

// WriteScorecard writes the results as Markdown: one table of engine totals, then one row per
// engine and item. The first spoken word is criterion 1's latency; first token, first
// sentence and synthesis first audio are diagnostic. "Not for speech" is what NotForSpeech
// found in a summary; it does not change the correctness verdicts.
func WriteScorecard(w io.Writer, results []Result, rows []Row) error {
	var b strings.Builder
	b.WriteString("## Engines\n\n| Engine | Adapter | Model | Where | Note |\n| --- | --- | --- | --- | --- |\n")
	for _, r := range rows {
		fmt.Fprintf(&b, "| %s | %s | %s | %s | %s |\n", r.Name, r.Adapter, r.Model, where(r), cell(r.Note))
	}

	b.WriteString("\n## Totals\n\n| Engine | Rows | Failed | Correct | Not for speech | Median first spoken word | Median first token | Median first sentence | Median synthesis first audio | Median total | Cost | Mean rating |\n|" + strings.Repeat(" --- |", 12) + "\n")
	for _, r := range rows {
		var mine []Result
		for _, res := range results {
			if res.Row.Name == r.Name {
				mine = append(mine, res)
			}
		}
		fmt.Fprintf(&b, "| %s | %s |\n", r.Name, strings.Join(totals(mine), " | "))
	}

	b.WriteString("\n## Rows\n\n| Item | Engine | " + strings.Join(Fields, " | ") + " | First spoken word | First token | First sentence | Synthesis first audio | Total | Tokens in/out | Cost | Rating | Not for speech | Summary |\n")
	b.WriteString("|" + strings.Repeat(" --- |", 12+len(Fields)) + "\n")
	for _, res := range results {
		verdicts := res.Verdicts
		if verdicts == nil {
			verdicts = make([]Verdict, len(Fields))
		}
		cols := []string{res.Item, res.Row.Name}
		for _, v := range verdicts {
			cols = append(cols, string(v))
		}
		s := res.Summary
		if res.Err != nil {
			cols = append(cols, "", "", "", "", "", "", "", "", "", "failed: "+cell(res.Err.Error()))
		} else {
			spoken, synthesis := "", ""
			switch {
			case res.Spoken != nil:
				spoken, synthesis = ms(res.Spoken.FirstWord), ms(res.Spoken.Synthesis)
			case res.SpeechErr != nil:
				spoken = "speech failed: " + cell(res.SpeechErr.Error())
			}
			cols = append(cols, spoken, ms(s.FirstToken), ms(s.FirstSentence), synthesis, ms(s.Total), tokens(s), cost(s.Cost, s.CostKnown), rating(res.Rating), strings.Join(NotForSpeech(s.Text), ", "), cell(s.Text))
		}
		b.WriteString("| " + strings.Join(cols, " | ") + " |\n")
	}
	_, err := io.WriteString(w, b.String())
	return err
}

func totals(results []Result) []string {
	var failed, correct, unspoken, rated, ratingSum int
	var spoken, first, sentence, synthesis, total []time.Duration
	var spend float64
	spendKnown := len(results) > 0
	for _, r := range results {
		if r.Err != nil {
			failed++
			continue
		}
		if Correct(r.Verdicts) {
			correct++
		}
		if len(NotForSpeech(r.Summary.Text)) > 0 {
			unspoken++
		}
		if r.Spoken != nil {
			spoken = append(spoken, r.Spoken.FirstWord)
			synthesis = append(synthesis, r.Spoken.Synthesis)
		}
		first = append(first, r.Summary.FirstToken)
		sentence = append(sentence, r.Summary.FirstSentence)
		total = append(total, r.Summary.Total)
		spend += r.Summary.Cost
		spendKnown = spendKnown && r.Summary.CostKnown
		if r.Rating > 0 {
			rated++
			ratingSum += r.Rating
		}
	}
	mean := ""
	if rated > 0 {
		mean = fmt.Sprintf("%.1f (%d)", float64(ratingSum)/float64(rated), rated)
	}
	return []string{
		fmt.Sprint(len(results)), fmt.Sprint(failed), fmt.Sprintf("%d/%d", correct, len(results)-failed), fmt.Sprint(unspoken),
		median(spoken), median(first), median(sentence), median(synthesis), median(total), cost(spend, spendKnown), mean,
	}
}

func where(r Row) string {
	if r.Local {
		return "local"
	}
	return "external: " + r.Provider
}

func median(ds []time.Duration) string {
	if len(ds) == 0 {
		return ""
	}
	s := append([]time.Duration(nil), ds...)
	sort.Slice(s, func(i, j int) bool { return s[i] < s[j] })
	m := s[len(s)/2]
	if len(s)%2 == 0 {
		m = (s[len(s)/2-1] + s[len(s)/2]) / 2
	}
	return ms(m)
}

func ms(d time.Duration) string { return fmt.Sprintf("%d ms", d.Milliseconds()) }

func tokens(s Summary) string {
	if !s.UsageKnown {
		return "unknown"
	}
	return fmt.Sprintf("%d/%d", s.InputTokens, s.OutputTokens)
}

func cost(c float64, known bool) string {
	if !known {
		return "unknown"
	}
	return fmt.Sprintf("$%.6f", c)
}

func rating(r int) string {
	if r == 0 {
		return ""
	}
	return fmt.Sprint(r)
}

// cell keeps a value inside one Markdown table cell.
func cell(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	return strings.ReplaceAll(s, "|", `\|`)
}
