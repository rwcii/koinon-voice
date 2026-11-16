package main

import (
	"context"
	"sync"

	"github.com/rwcii/koinon-voice/internal/koinon"
)

// The signed rows' journal and options; tests give a temporary journal and a fake node.
var (
	signedJournal = koinon.Journal{Path: koinon.DefaultJournalPath()}
	sourceOptions = koinon.SourceOptions{}
)

// sources are the signed rows of a hosts file, in its order, with the journal's records that
// no row serves.
type sources struct {
	rows       []*koinon.SignedSource
	unrouted   []koinon.Unrouted
	journalErr error
}

// newSources makes each row's source; nothing here waits on the Keychain. The maintainer
// key and enrollments are read once, when the first row starts; a failure there shows on
// every row.
func newSources(hosts []koinon.Host, changed func()) sources {
	var s sources
	// The key and enrollments are read once, by the first row to start, under its
	// context: a Keychain that does not answer ends with the command.
	var once sync.Once
	var access koinon.Access
	var accessErr error
	load := func(ctx context.Context) (koinon.Access, error) {
		once.Do(func() {
			access, accessErr = koinon.SignedAccess{Store: keyStore, Enrollments: keyEnrollments, Client: koinon.ClientName}.Load(ctx)
		})
		return access, accessErr
	}
	pins := map[string]bool{}
	for _, h := range hosts {
		so := sourceOptions
		so.Host, so.LoadAccess, so.Version, so.Journal = h, load, version, signedJournal
		so.OnChange = func([]koinon.State) { changed() }
		s.rows = append(s.rows, koinon.NewSignedSource(so))
		pins[h.NodePin] = true
	}
	entries, err := signedJournal.Load()
	if err != nil {
		s.journalErr = err
	} else {
		s.unrouted = koinon.UnroutedEntries(entries, pins, signedJournal.Path)
	}
	return s
}

// run runs every row until ctx ends; wg waits for them.
func (s sources) run(ctx context.Context, wg *sync.WaitGroup) {
	for _, r := range s.rows {
		wg.Add(1)
		go func(r *koinon.SignedSource) {
			defer wg.Done()
			r.Run(ctx)
		}(r)
	}
}

// states are every row's states in file order: one for its node and one for each computer
// paired with it.
func (s sources) states() []koinon.State {
	var out []koinon.State
	for _, r := range s.rows {
		out = append(out, r.States()...)
	}
	return out
}
