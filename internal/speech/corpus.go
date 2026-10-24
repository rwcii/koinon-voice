package speech

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// Sets are the corpus's two readings of the phrase list (definition of done, live check 2).
var Sets = []string{"clean", "noisy"}

// WakeSets hold the maintainer saying "Koinon" once per file at three distances, and
// BackgroundSets long recordings with no "Koinon" in them, for the wake bench (definition of
// done, live check 4).
var (
	WakeSets       = []string{"wake/near", "wake/mid", "wake/far"}
	BackgroundSets = []string{"background/music", "background/meeting", "background/podcast", "background/coffee"}
)

// Corpus is the recordings directory, ~/.local/share/koinon-voice/corpus by default:
// SET/NN.wav and SET/manifest.tsv for each set. It is private to its owner.
type Corpus struct {
	Dir string
}

// Entry is one manifest line: a phrase's recording and its SHA-256.
type Entry struct {
	N       int
	SHA256  string
	Seconds float64
}

// Path is where phrase n of a set is recorded.
func (c Corpus) Path(set string, n int) string {
	return filepath.Join(c.Dir, set, fmt.Sprintf("%02d.wav", n))
}

// Prepare makes the set's directory, readable only by its owner, and returns its path.
func (c Corpus) Prepare(set string) (string, error) {
	if !validSet(set) {
		return "", fmt.Errorf("corpus set %q: want one of %s", set, strings.Join(allSets(), ", "))
	}
	dir := filepath.Join(c.Dir, set)
	// Every directory from the corpus down is the owner's only, "wake" as well as "wake/near".
	dirs := []string{c.Dir}
	if parent := filepath.Dir(set); parent != "." {
		dirs = append(dirs, filepath.Join(c.Dir, parent))
	}
	for _, d := range append(dirs, dir) {
		if err := os.MkdirAll(d, 0o700); err != nil {
			return "", err
		}
		if err := os.Chmod(d, 0o700); err != nil {
			return "", err
		}
	}
	return dir, nil
}

func allSets() []string {
	return append(append(append([]string(nil), Sets...), WakeSets...), BackgroundSets...)
}

func validSet(set string) bool {
	for _, s := range allSets() {
		if s == set {
			return true
		}
	}
	return false
}

// Manifest reads a set's manifest; a set not yet recorded has none.
func (c Corpus) Manifest(set string) (map[int]Entry, error) {
	f, err := os.Open(filepath.Join(c.Dir, set, "manifest.tsv"))
	if os.IsNotExist(err) {
		return map[int]Entry{}, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	out := map[int]Entry{}
	sc := bufio.NewScanner(f)
	for line := 1; sc.Scan(); line++ {
		if line == 1 || strings.TrimSpace(sc.Text()) == "" {
			continue // header
		}
		parts := strings.Split(sc.Text(), "\t")
		if len(parts) != 3 {
			return nil, fmt.Errorf("%s manifest line %d: want phrase, sha256, seconds", set, line)
		}
		n, err1 := strconv.Atoi(parts[0])
		secs, err2 := strconv.ParseFloat(parts[2], 64)
		if err1 != nil || err2 != nil || len(parts[1]) != 64 {
			return nil, fmt.Errorf("%s manifest line %d: %q", set, line, sc.Text())
		}
		out[n] = Entry{N: n, SHA256: parts[1], Seconds: secs}
	}
	return out, sc.Err()
}

// WriteManifest replaces a set's manifest with entries in phrase order.
func (c Corpus) WriteManifest(set string, entries map[int]Entry) error {
	var ns []int
	for n := range entries {
		ns = append(ns, n)
	}
	sort.Ints(ns)
	var b strings.Builder
	b.WriteString("phrase\tsha256\tseconds\n")
	for _, n := range ns {
		e := entries[n]
		fmt.Fprintf(&b, "%02d\t%s\t%.2f\n", e.N, e.SHA256, e.Seconds)
	}
	path := filepath.Join(c.Dir, set, "manifest.tsv")
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(b.String()), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// Hash is a file's SHA-256 in hex.
func Hash(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// Verify checks a recorded file against its manifest entry: the bench scores only files
// whose hash the manifest records.
func (c Corpus) Verify(set string, e Entry) (Audio, error) {
	path := c.Path(set, e.N)
	sum, err := Hash(path)
	if err != nil {
		return Audio{}, err
	}
	if sum != e.SHA256 {
		return Audio{}, fmt.Errorf("%s changed since it was recorded: its SHA-256 is not the manifest's", path)
	}
	return CheckCorpusFile(path)
}

// RecordSeconds is how long to record a phrase: enough for a normal pace and a pause.
func RecordSeconds(p Phrase) float64 {
	return max(3, 2+0.5*float64(len(p.Words)))
}
