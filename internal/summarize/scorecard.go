package summarize

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"time"
)

// WriteScorecard writes the results as Markdown: one table of engine totals, then one row per
// engine and item. Timings are diagnostic; spoken latency is chunk 03's.
func WriteScorecard(w io.Writer, results []Result, rows []Row) error {
	var b strings.Builder
	b.WriteString("## Engines\n\n| Engine | Adapter | Model | Where | Note |\n| --- | --- | --- | --- | --- |\n")
	for _, r := range rows {
		fmt.Fprintf(&b, "| %s | %s | %s | %s | %s |\n", r.Name, r.Adapter, r.Model, where(r), cell(r.Note))
	}

	b.WriteString("\n## Totals\n\n| Engine | Rows | Failed | Correct | Median first token | Median first sentence | Median total | Cost | Mean rating |\n| --- | --- | --- | --- | --- | --- | --- | --- | --- |\n")
	for _, r := range rows {
		var mine []Result
		for _, res := range results {
			if res.Row.Name == r.Name {
				mine = append(mine, res)
			}
		}
		fmt.Fprintf(&b, "| %s | %s |\n", r.Name, strings.Join(totals(mine), " | "))
	}

	b.WriteString("\n## Rows\n\n| Item | Engine | " + strings.Join(Fields, " | ") + " | First token | First sentence | Total | Tokens in/out | Cost | Rating | Summary |\n")
	b.WriteString("|" + strings.Repeat(" --- |", 9+len(Fields)) + "\n")
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
			cols = append(cols, "", "", "", "", "", "", "failed: "+cell(res.Err.Error()))
		} else {
			cols = append(cols, ms(s.FirstToken), ms(s.FirstSentence), ms(s.Total), tokens(s), cost(s.Cost, s.CostKnown), rating(res.Rating), cell(s.Text))
		}
		b.WriteString("| " + strings.Join(cols, " | ") + " |\n")
	}
	_, err := io.WriteString(w, b.String())
	return err
}

func totals(results []Result) []string {
	var failed, correct, rated, ratingSum int
	var first, sentence, total []time.Duration
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
		fmt.Sprint(len(results)), fmt.Sprint(failed), fmt.Sprintf("%d/%d", correct, len(results)-failed),
		median(first), median(sentence), median(total), cost(spend, spendKnown), mean,
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
