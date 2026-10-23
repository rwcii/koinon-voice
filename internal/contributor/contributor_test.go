// Package contributor checks the repository contracts shared with Koinon.
package contributor

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

func root(t *testing.T) string {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	return filepath.Clean(filepath.Join(filepath.Dir(file), "../.."))
}
func read(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(root(t), name))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestSharedSkillsAndGuidance(t *testing.T) {
	for _, name := range []string{".claude/skills", ".agents/skills", ".codex/skills"} {
		target, err := os.Readlink(filepath.Join(root(t), name))
		if err != nil || target != "../agents/skills" {
			t.Fatalf("%s: %q %v", name, target, err)
		}
	}
	entries, err := os.ReadDir(filepath.Join(root(t), "agents/skills"))
	if err != nil {
		t.Fatal(err)
	}
	index := read(t, "agents/skills/README.md")
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		text := read(t, "agents/skills/"+name+"/SKILL.md")
		if !strings.HasPrefix(text, "---\nname: "+name+"\ndescription: ") || !strings.Contains(text, "agents/skills/AGENTS.md") {
			t.Errorf("%s: missing shared skill contract", name)
		}
		if !strings.Contains(index, "("+name+"/SKILL.md)") {
			t.Errorf("%s: missing index entry", name)
		}
	}
}

// raceTests is the race-enabled suite command; every place that runs it names one limit.
const raceTests = "go test -race -timeout 30m ./..."

func TestRaceCommandIsTheSameEverywhere(t *testing.T) {
	for _, file := range []string{".github/workflows/tests.yml", ".koinon/skills/check.md", ".koinon/skills/ship.md", "AGENTS.md", "CONTRIBUTING.md", "README.md"} {
		text := read(t, file)
		if !strings.Contains(text, raceTests) {
			t.Errorf("%s does not run %q", file, raceTests)
		}
		if strings.Count(text, "go test -race") != strings.Count(text, raceTests) {
			t.Errorf("%s runs another go test -race command", file)
		}
	}
	// The job limit must exceed the go test limit, or the job ends without the stack of a
	// hung test.
	if !strings.Contains(read(t, ".github/workflows/tests.yml"), "timeout-minutes: 40") {
		t.Error("the Go CI job needs a 40-minute limit above the 30-minute go test timeout")
	}
}

// Run the real repository command so missing, stale and retired wrappers fail CI.
func TestOpenCodeSkillCommandsCurrent(t *testing.T) {
	// Subprocess reads are invisible to Go's test cache. Read its inputs here too,
	// so a cached passing check cannot hide changed wrappers or generator code.
	for _, tree := range []string{"agents/skills", ".opencode/commands", "scripts/skill-commands"} {
		err := filepath.WalkDir(filepath.Join(root(t), tree), func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.Type().IsRegular() {
				_, err = os.ReadFile(path)
				return err
			}
			_, err = os.Lstat(path)
			return err
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	cmd := exec.Command("go", "run", "./scripts/skill-commands", "--check")
	cmd.Dir = root(t)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("OpenCode commands are out of date: %v\n%s", err, out)
	}
}

// Per WDR 001, each series README lists every record with the number, title and status in
// the record's own frontmatter.
func TestRecordIndexes(t *testing.T) {
	field := func(text, name string) string {
		m := regexp.MustCompile(`(?m)^` + name + `: "?([^"\n]*)"?$`).FindStringSubmatch(text)
		if m == nil {
			return ""
		}
		return m[1]
	}
	for _, series := range []struct{ dir, key string }{
		{"docs/architecture/adr", "adr"}, {"docs/product/pdr", "pdr"}, {"docs/engineering/wdr", "wdr"},
	} {
		index := read(t, series.dir+"/README.md")
		files, err := filepath.Glob(filepath.Join(root(t), series.dir, "[0-9][0-9][0-9]*-*.md"))
		if err != nil {
			t.Fatal(err)
		}
		listed := regexp.MustCompile(`(?m)^\| \d{3}`).FindAllString(index, -1)
		records := 0
		for _, path := range files {
			name := filepath.Base(path)
			if strings.HasPrefix(name, "000-") {
				continue
			}
			records++
			text := read(t, series.dir+"/"+name)
			number, title, status := field(text, series.key), field(text, "title"), field(text, "status")
			if number == "" || !strings.HasPrefix(name, number+"-") {
				t.Errorf("%s: frontmatter %s %q does not match the file name", name, series.key, number)
				continue
			}
			status = strings.ToUpper(status[:1]) + strings.ReplaceAll(status[1:], "_", " ")
			row := "| " + number + "    | [" + title + "](" + name + ") | " + status + " |"
			if !strings.Contains(index, row) {
				t.Errorf("%s/README.md lacks %q", series.dir, row)
			}
		}
		if len(listed) != records {
			t.Errorf("%s/README.md lists %d records, the directory has %d", series.dir, len(listed), records)
		}
	}
}

// Per WDR 004, Decision 5, the attribution check refuses tool attribution and passes the
// maintainer's ordinary commits.
func TestAttributionCheck(t *testing.T) {
	check := func(args ...string) bool {
		cmd := exec.Command("sh", append([]string{"scripts/check-attribution.sh"}, args...)...)
		cmd.Dir = root(t)
		return cmd.Run() == nil
	}
	message := func(text string) string {
		path := filepath.Join(t.TempDir(), "message")
		if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
		return path
	}
	for _, text := range []string{
		"Fix a thing\n\nCo-Authored-By: Claude Opus <noreply@anthropic.com>\n",
		"Fix a thing\n\n  co-authored-by: someone <a@b.c>\n",
		"Fix a thing\n\nClaude-Session: abc\n",
		"Body\n\n🤖 Generated with [Claude Code](https://claude.com/claude-code)\n",
		"See https://claude.ai/code/session_123\n",
	} {
		if check(message(text)) {
			t.Errorf("accepted %q", text)
		}
	}
	ok := "Add the record index test\n\nThe maintainer asked for it.\n\nSigned-off-by: Robert Capps <87878852+rwcii@users.noreply.github.com>\n"
	if !check(message(ok)) {
		t.Errorf("refused %q", ok)
	}
	if check("--identity", "Claude <noreply@anthropic.com>") || check("--identity", "Robert Capps <a@b.c>", "claude-bot <x@y.z>") {
		t.Error("accepted a Claude identity")
	}
	if !check("--identity", "Robert Capps <87878852+rwcii@users.noreply.github.com> 1 +0000", "GitHub <noreply@github.com>") {
		t.Error("refused the maintainer's identity")
	}
}
