package signed

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
)

// Enrolled records one enrollment statement that this install signed for a computer: the
// authorization ID that the statement makes when the maintainer adds it there, which every
// envelope to that computer carries (Koinon contract 05c531c, Decisions 1.3 and 1.7).
type Enrolled struct {
	Auth    string `json:"auth"`
	KeyID   string `json:"key_id"`
	User    string `json:"user"`
	Client  string `json:"client"`
	Created int64  `json:"created"`
}

// Enrollments is the file of this install's enrollments, by target pin. The latest statement
// for a pin replaces the earlier one: a computer adds each statement once, so the maintainer
// adds the latest.
type Enrollments struct {
	Path string
}

// DefaultEnrollmentsPath is ~/.local/share/koinon-voice/enrollments.json.
func DefaultEnrollmentsPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".local", "share", "koinon-voice", "enrollments.json")
	}
	return filepath.Join(home, ".local", "share", "koinon-voice", "enrollments.json")
}

// Load reads the enrollments; a missing file has none.
func (s Enrollments) Load() (map[string]Enrolled, error) {
	b, err := os.ReadFile(s.Path)
	if errors.Is(err, os.ErrNotExist) {
		return map[string]Enrolled{}, nil
	}
	if err != nil {
		return nil, err
	}
	var m map[string]Enrolled
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, fmt.Errorf("the enrollments file %s is not readable", s.Path)
	}
	return m, nil
}

// Record stores the enrollment of a signed statement for its target, synced, under a lock.
func (s Enrollments) Record(statement Envelope, user, client string) (Enrolled, error) {
	auth, err := AuthID(statement)
	if err != nil {
		return Enrolled{}, err
	}
	e := Enrolled{Auth: auth, KeyID: statement.Key, User: user, Client: client, Created: statement.Created}
	if err := os.MkdirAll(filepath.Dir(s.Path), 0o700); err != nil {
		return Enrolled{}, err
	}
	lock, err := os.OpenFile(s.Path+".lock", os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return Enrolled{}, err
	}
	defer lock.Close()
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX); err != nil {
		return Enrolled{}, err
	}
	defer syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)
	m, err := s.Load()
	if err != nil {
		return Enrolled{}, err
	}
	m[statement.Target] = e
	b, _ := json.MarshalIndent(m, "", "  ")
	tmp := s.Path + ".tmp" + strconv.Itoa(os.Getpid())
	if err := writeSynced(tmp, append(b, '\n')); err != nil {
		os.Remove(tmp)
		return Enrolled{}, err
	}
	if err := os.Rename(tmp, s.Path); err != nil {
		os.Remove(tmp)
		return Enrolled{}, err
	}
	return e, nil
}

func writeSynced(path string, b []byte) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	if _, err := f.Write(b); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}
