package koinon

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"syscall"

	"github.com/rwcii/koinon-voice/internal/signed"
)

// Journal is the record of the signed writes whose outcome Voice does not know yet: each
// write is stored before it is first sent and removed when its outcome is final, so a new
// process can ask the target for it (R15). It is not a queue: nothing in it is sent as a new
// action, only asked about under its original envelope.
type Journal struct {
	Path string
}

// DefaultJournalPath is ~/.local/share/koinon-voice/pending.json, beside the enrollments.
func DefaultJournalPath() string {
	return filepath.Join(filepath.Dir(signed.DefaultEnrollmentsPath()), "pending.json")
}

// Entry is one write in the journal. NodePin and the envelope's target route it; Node is
// the row's label when it was written, for display only.
type Entry struct {
	NodePin   string          `json:"node_pin"`
	Node      string          `json:"node"`
	Envelope  signed.Envelope `json:"envelope"`
	FirstSent int64           `json:"first_sent"`
	// Final is "unknown" when the outcome can no longer be learned (authorization_changed,
	// or past the receipts' days); Reason says why. Such an entry is never asked about
	// again; the maintainer reads the target's record and dismisses it.
	Final  string `json:"final,omitempty"`
	Reason string `json:"reason,omitempty"`
}

// Request is the entry's request ID, its key in the journal.
func (e Entry) Request() string { return e.Envelope.Request }

// Reference is the decision item that the entry's verdict names.
func (e Entry) Reference() string { s, _ := e.Envelope.Args["reference"].(string); return s }

// ErrPending is a write for an item that already has one whose outcome is not known.
var ErrPending = errors.New("a verdict on this item is waiting for its outcome")

// Load reads every entry; a missing file has none.
func (j Journal) Load() (map[string]Entry, error) {
	b, err := os.ReadFile(j.Path)
	if errors.Is(err, os.ErrNotExist) {
		return map[string]Entry{}, nil
	}
	if err != nil {
		return nil, err
	}
	var m map[string]Entry
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	if err := dec.Decode(&m); err != nil || m == nil || dec.Decode(&struct{}{}) != io.EOF {
		return nil, fmt.Errorf("the pending writes file %s is not one JSON object; it is kept for inspection", j.Path)
	}
	for k, e := range m {
		e.Envelope.Args = intArgs(e.Envelope.Args)
		m[k] = e
	}
	return m, nil
}

// intArgs turns the integers of v1 arguments back into int64, as they were signed; another
// number stays a json.Number, which the signature check refuses.
func intArgs(m map[string]any) map[string]any {
	if m == nil {
		return nil
	}
	out := make(map[string]any, len(m))
	for k, v := range m {
		switch x := v.(type) {
		case json.Number:
			if i, err := x.Int64(); err == nil {
				out[k] = i
			} else {
				out[k] = x
			}
		case map[string]any:
			out[k] = intArgs(x)
		default:
			out[k] = v
		}
	}
	return out
}

// Reserve stores e unless an entry for the same target and reference is there, through any
// node; then it returns that entry with ErrPending, keeping its own node and authorization.
// The check and the store are one locked step, for goroutines and processes alike.
func (j Journal) Reserve(e Entry) (Entry, error) {
	var existing Entry
	err := j.change(func(m map[string]Entry) error {
		for _, o := range m {
			if o.Envelope.Target == e.Envelope.Target && o.Reference() == e.Reference() {
				existing = o
				return ErrPending
			}
		}
		m[e.Request()] = e
		return nil
	})
	return existing, err
}

// Update changes the entry with the request ID, if it is there.
func (j Journal) Update(request string, f func(*Entry)) error {
	return j.change(func(m map[string]Entry) error {
		if e, ok := m[request]; ok {
			f(&e)
			m[request] = e
		}
		return nil
	})
}

// Remove deletes the entry with the request ID.
func (j Journal) Remove(request string) error {
	return j.change(func(m map[string]Entry) error {
		delete(m, request)
		return nil
	})
}

// change reads, changes and writes the journal under its lock file. The file is written to
// a synced temporary file, renamed, and its directory synced.
func (j Journal) change(f func(map[string]Entry) error) error {
	dir := filepath.Dir(j.Path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	lock, err := os.OpenFile(j.Path+".lock", os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return err
	}
	defer lock.Close()
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX); err != nil {
		return err
	}
	defer syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)
	m, err := j.Load()
	if err != nil {
		return err
	}
	if err := f(m); err != nil {
		return err
	}
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	tmp := j.Path + ".tmp" + strconv.Itoa(os.Getpid())
	if err := writeFileSynced(tmp, append(b, '\n')); err != nil {
		os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, j.Path); err != nil {
		os.Remove(tmp)
		return err
	}
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer d.Close()
	return d.Sync()
}

func writeFileSynced(path string, b []byte) error {
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

// checkEntry checks an entry before it is asked about: a decision_verdict envelope of
// version v1 from this node's journal, with well-formed fields, signed by this key. It
// returns why the entry cannot be used, or "".
func checkEntry(e Entry, key signed.Key) string {
	env := e.Envelope
	switch {
	case env.V != signed.Version:
		return "the entry's envelope version is not " + signed.Version
	case env.Op != "decision_verdict":
		return "the entry is not a verdict"
	case signed.CheckPin(env.Target) != nil || signed.CheckPin(e.NodePin) != nil:
		return "the entry's pins are malformed"
	case e.Reference() == "":
		return "the entry names no item"
	case !isAuthID(env.Auth):
		return "the entry's authorization ID is malformed"
	case env.Created <= 0 || len(env.Request) < 16:
		return "the entry's request or creation time is malformed"
	case env.Key != key.ID():
		return "the entry was signed by another key"
	}
	if err := signed.Verify(key.Public(), env); err != nil {
		return "the entry's signature does not verify"
	}
	return ""
}
