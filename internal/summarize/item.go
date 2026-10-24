// Package summarize turns Koinon decision items into short spoken summaries with configured
// inference engines and scores them (Phase 0, criterion 1).
package summarize

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Item is the part of a Koinon decision item (decision_get) that a summary speaks. Field names
// follow Koinon v0.4.1's DecisionItem JSON, so an exported item reads without conversion.
type Item struct {
	Reference      string   `json:"reference"`
	Title          string   `json:"title"`
	Kind           string   `json:"kind"`
	Where          string   `json:"where"`
	AgentText      string   `json:"agent_text"`
	Context        string   `json:"context"`
	Recommendation string   `json:"recommendation"`
	Commands       []string `json:"commands"`
	AuthorFamily   string   `json:"author_family"`
	AuthorName     string   `json:"author_name"`
}

// Field is one answer-key field: every group must match the summary, and a group matches
// when any one of its alternatives appears. An empty Field means the item has no such fact.
type Field [][]string

// Key is a fixture's answer key for the correctness check.
type Key struct {
	Asker          Field `json:"asker"`
	Ask            Field `json:"ask"`
	Options        Field `json:"options"`
	Recommendation Field `json:"recommendation"`
	Deadline       Field `json:"deadline"`
}

// Fixture is a synthetic item with its answer key. Key is nil for an item exported from a
// daemon, whose correctness the maintainer judges.
type Fixture struct {
	Name string `json:"-"`
	Item Item   `json:"item"`
	Key  *Key   `json:"key"`
}

const maxItemFile = 1 << 20

// LoadFixtures reads every *.json fixture in dir, sorted by file name.
func LoadFixtures(dir string) ([]Fixture, error) {
	paths, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		return nil, err
	}
	if len(paths) == 0 {
		return nil, fmt.Errorf("no *.json fixtures in %s", dir)
	}
	sort.Strings(paths)
	fixtures := make([]Fixture, 0, len(paths))
	for _, path := range paths {
		f, err := readFixture(path)
		if err != nil {
			return nil, err
		}
		if f.Key == nil {
			return nil, fmt.Errorf("%s: fixture has no answer key", path)
		}
		fixtures = append(fixtures, f)
	}
	return fixtures, nil
}

// LoadItem reads one decision item exported as JSON: either the item itself, the
// decision_get result ({"item": {...}}) or a fixture.
func LoadItem(path string) (Fixture, error) {
	return readFixture(path)
}

func readFixture(path string) (Fixture, error) {
	data, err := readLimited(path)
	if err != nil {
		return Fixture{}, err
	}
	var f Fixture
	if err := json.Unmarshal(data, &f); err != nil {
		return Fixture{}, fmt.Errorf("%s: %w", path, err)
	}
	if f.Item.Title == "" {
		// Not wrapped: the file is the item itself.
		f = Fixture{}
		if err := json.Unmarshal(data, &f.Item); err != nil {
			return Fixture{}, fmt.Errorf("%s: %w", path, err)
		}
	}
	if f.Item.Title == "" {
		return Fixture{}, fmt.Errorf("%s: item has no title", path)
	}
	f.Name = strings.TrimSuffix(filepath.Base(path), ".json")
	if f.Item.Reference != "" {
		f.Name = f.Item.Reference
	}
	return f, nil
}

func readLimited(path string) ([]byte, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if info.Size() > maxItemFile {
		return nil, errors.New(path + ": larger than 1 MiB")
	}
	return os.ReadFile(path)
}
