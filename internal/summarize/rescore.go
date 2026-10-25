package summarize

import (
	"fmt"
	"strings"
)

const notForSpeechHeader = "Not for speech"

// Rescore checks a scorecard's recorded summaries again with the fixtures' current answer
// keys, check and NotForSpeech, without running an engine. It rewrites the verdicts, each
// engine's correct count and the "Not for speech" columns, adding them to a scorecard
// written before they existed, and leaves every other cell and line as it was. It returns
// the scorecard and one line per changed verdict.
func Rescore(scorecard string, fixtures []Fixture) (string, []string, error) {
	keys := make(map[string]*Key, len(fixtures))
	for _, f := range fixtures {
		keys[f.Name] = f.Key
	}
	lines := strings.Split(scorecard, "\n")
	type tally struct{ rows, failed, correct, unspoken int }
	tallies := map[string]*tally{}
	var changes []string

	rows, ok := table(lines, "## Rows")
	if !ok {
		return "", nil, fmt.Errorf("no Rows table")
	}
	// Item, engine, the verdicts and eight measures, then "Not for speech" and the summary.
	unspokenCol := 2 + len(Fields) + 8
	summaryCol := unspokenCol + 1
	if !strings.Contains(lines[rows], notForSpeechHeader) {
		lines[rows] = insertCell(lines[rows], unspokenCol, notForSpeechHeader)
		lines[rows+1] = insertCell(lines[rows+1], unspokenCol, "---")
	}
	for i := rows + 2; i < len(lines) && strings.HasPrefix(lines[i], "|"); i++ {
		cells := splitRow(lines[i])
		if len(cells) == summaryCol { // written before the column existed
			cells = append(cells[:unspokenCol], "", cells[unspokenCol])
		}
		if len(cells) != summaryCol+1 {
			return "", nil, fmt.Errorf("row %q has %d cells", lines[i], len(cells))
		}
		item, engine := cells[0], cells[1]
		t := tallies[engine]
		if t == nil {
			t = &tally{}
			tallies[engine] = t
		}
		t.rows++
		text := strings.ReplaceAll(cells[summaryCol], `\|`, "|")
		if strings.HasPrefix(text, "failed: ") && cells[2] == "" {
			t.failed++
			cells[unspokenCol] = ""
			lines[i] = joinRow(cells)
			continue
		}
		key, found := keys[item]
		if !found {
			return "", nil, fmt.Errorf("no fixture for %s", item)
		}
		verdicts := Check(text, key)
		for j, v := range verdicts {
			if cells[2+j] != string(v) {
				changes = append(changes, fmt.Sprintf("%s %s %s: %s to %s", item, engine, Fields[j], cells[2+j], v))
				cells[2+j] = string(v)
			}
		}
		if Correct(verdicts) {
			t.correct++
		}
		kinds := NotForSpeech(text)
		if len(kinds) > 0 {
			t.unspoken++
		}
		cells[unspokenCol] = strings.Join(kinds, ", ")
		lines[i] = joinRow(cells)
	}

	totals, ok := table(lines, "## Totals")
	if !ok {
		return "", nil, fmt.Errorf("no Totals table")
	}
	added := !strings.Contains(lines[totals], notForSpeechHeader)
	if added {
		lines[totals] = insertCell(lines[totals], 4, notForSpeechHeader)
		lines[totals+1] = insertCell(lines[totals+1], 4, "---")
	}
	for i := totals + 2; i < len(lines) && strings.HasPrefix(lines[i], "|"); i++ {
		cells := splitRow(lines[i])
		t := tallies[cells[0]]
		if t == nil {
			return "", nil, fmt.Errorf("no rows for engine %s", cells[0])
		}
		if added {
			cells = append(cells[:4], append([]string{""}, cells[4:]...)...)
		}
		cells[3] = fmt.Sprintf("%d/%d", t.correct, t.rows-t.failed)
		cells[4] = fmt.Sprint(t.unspoken)
		lines[i] = joinRow(cells)
	}
	return strings.Join(lines, "\n"), changes, nil
}

// table returns the index of the header line of the first table under heading.
func table(lines []string, heading string) (int, bool) {
	for i, l := range lines {
		if l != heading {
			continue
		}
		for j := i + 1; j+1 < len(lines); j++ {
			if strings.HasPrefix(lines[j], "|") {
				return j, true
			}
			if strings.HasPrefix(lines[j], "## ") {
				break
			}
		}
	}
	return 0, false
}

// splitRow splits a Markdown table row into its cells; an escaped \| stays in its cell.
func splitRow(line string) []string {
	line = strings.TrimSuffix(strings.TrimPrefix(line, "| "), " |")
	var cells []string
	var b strings.Builder
	for i := 0; i < len(line); i++ {
		if line[i] == '\\' && i+1 < len(line) && line[i+1] == '|' {
			b.WriteString(`\|`)
			i++
			continue
		}
		if line[i] == '|' {
			cells = append(cells, strings.TrimSpace(b.String()))
			b.Reset()
			continue
		}
		b.WriteByte(line[i])
	}
	return append(cells, strings.TrimSpace(b.String()))
}

func joinRow(cells []string) string { return "| " + strings.Join(cells, " | ") + " |" }

func insertCell(line string, at int, cell string) string {
	cells := splitRow(line)
	cells = append(cells[:at], append([]string{cell}, cells[at:]...)...)
	return joinRow(cells)
}
