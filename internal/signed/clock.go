package signed

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
	"time"
)

// Clock gives each envelope's created so that a key's values never go back (Koinon contract
// 072870b, Decision 1.7, with the clarification sent as G18c). It keeps, for each key, the
// highest created it signed and the created of its latest enrollment statement, in a file,
// so the rule holds across processes and restarts:
//
//   - an enrollment statement is signed at max(now, highest+1);
//   - any other envelope at max(now, highest, enrolled+1): one step above the enrollment's
//     exclusive floor, not a step per envelope, so a burst in one second keeps one value.
//
// The new values are on disk (synced) before they are returned, so a crash never lets a later
// enrollment sign at or below an envelope that was sent. A key's history is made when the key
// is made; a key without history is refused, never started again from the clock, because its
// earlier envelopes are unknown (see Restore).
type Clock struct {
	// Path is the state file; DefaultClockPath gives the usual one. Its lock is Path.lock, a
	// file that is never renamed.
	Path string
	// Now is the wall clock; nil is time.Now.
	Now func() time.Time
}

// history is one key's state.
type history struct {
	Highest  int64 `json:"highest"`
	Enrolled int64 `json:"enrolled"`
}

// ErrNoHistory is the answer of Next for a key that has no history in the file.
var ErrNoHistory = errors.New("this key has no signing history; if it was lost, run koinon-voice key clock --restore")

// RestoreMargin is how far ahead of the clock Restore puts a key's history: every envelope
// is valid for 5 minutes of a target's clock (Decision 3.3), so one signed before the loss
// with created up to now+RestoreMargin is below the restored values.
const RestoreMargin = 5 * 60

// DefaultClockPath is ~/.local/share/koinon-voice/signed-created.json.
func DefaultClockPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".local", "share", "koinon-voice", "signed-created.json")
	}
	return filepath.Join(home, ".local", "share", "koinon-voice", "signed-created.json")
}

// Init makes the empty history of a new key. It refuses a key that has one.
func (c Clock) Init(keyID string) error {
	_, err := c.update(func(h map[string]history, now int64) (int64, error) {
		if _, ok := h[keyID]; ok {
			return 0, fmt.Errorf("the key %s already has a signing history", keyID)
		}
		// A new key has signed nothing yet.
		h[keyID] = history{}
		return 0, nil
	})
	return err
}

// Restore makes a history for a key whose history was lost, RestoreMargin seconds ahead of
// the clock. It assumes the system clock did not step back by more than RestoreMargin before
// the loss; the outcome of a write sent before it stays unknown.
func (c Clock) Restore(keyID string) (int64, error) {
	return c.update(func(h map[string]history, now int64) (int64, error) {
		v := now + RestoreMargin
		if k, ok := h[keyID]; ok {
			v = max(v, k.Highest+1)
		}
		h[keyID] = history{Highest: v, Enrolled: v}
		return v, nil
	})
}

// Next returns the created for a new envelope of keyID and stores it.
func (c Clock) Next(keyID string, enroll bool) (int64, error) {
	return c.update(func(h map[string]history, now int64) (int64, error) {
		k, ok := h[keyID]
		if !ok {
			return 0, ErrNoHistory
		}
		created := now
		if enroll {
			created = max(created, k.Highest+1)
			k.Enrolled = created
		} else {
			created = max(created, k.Highest, k.Enrolled+1)
		}
		k.Highest = max(k.Highest, created)
		h[keyID] = k
		return created, nil
	})
}

// update runs f on the file's histories under an exclusive lock and writes them back, synced,
// before it returns f's value.
func (c Clock) update(f func(h map[string]history, now int64) (int64, error)) (int64, error) {
	if c.Path == "" {
		return 0, errors.New("the signing clock has no state file")
	}
	if err := os.MkdirAll(filepath.Dir(c.Path), 0o700); err != nil {
		return 0, err
	}
	lock, err := os.OpenFile(c.Path+".lock", os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return 0, err
	}
	defer lock.Close()
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX); err != nil {
		return 0, fmt.Errorf("lock %s: %w", c.Path, err)
	}
	defer syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)

	var state struct {
		Keys map[string]history `json:"keys"`
	}
	if b, err := os.ReadFile(c.Path); err == nil {
		if err := json.Unmarshal(b, &state); err != nil {
			return 0, fmt.Errorf("the signing clock %s is not readable; it is never reset by itself", c.Path)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return 0, err
	}
	if state.Keys == nil {
		state.Keys = map[string]history{}
	}
	now := time.Now
	if c.Now != nil {
		now = c.Now
	}
	v, err := f(state.Keys, now().Unix())
	if err != nil {
		return 0, err
	}
	b, _ := json.Marshal(state)
	tmp := c.Path + ".tmp" + strconv.Itoa(os.Getpid())
	if err := writeSynced(tmp, b); err != nil {
		os.Remove(tmp)
		return 0, fmt.Errorf("the signing clock was not saved: %w", err)
	}
	if err := os.Rename(tmp, c.Path); err != nil {
		os.Remove(tmp)
		return 0, fmt.Errorf("the signing clock was not saved: %w", err)
	}
	dir, err := os.Open(filepath.Dir(c.Path))
	if err != nil {
		return 0, fmt.Errorf("the signing clock was not saved: %w", err)
	}
	defer dir.Close()
	if err := dir.Sync(); err != nil {
		return 0, fmt.Errorf("the signing clock was not saved: %w", err)
	}
	return v, nil
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
