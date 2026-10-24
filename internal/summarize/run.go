package summarize

import (
	"context"
	"strings"
	"time"
)

// Configured is a row with its engine, or the reason it could not be built.
type Configured struct {
	Row    Row
	Engine Engine
	Err    error
}

// Result is one engine's outcome for one item.
type Result struct {
	Item     string
	Row      Row
	Summary  Summary
	Err      error
	Verdicts []Verdict
	// Rating is the maintainer's blind listenability rating, 1 to 5; 0 is not rated.
	Rating int
}

// Run summarizes every item with every engine, one request at a time so timings do not
// compete. A failing engine fails its own rows and the run goes on.
func Run(ctx context.Context, engines []Configured, items []Fixture, timeout time.Duration) []Result {
	results := make([]Result, 0, len(engines)*len(items))
	for _, item := range items {
		for _, e := range engines {
			r := Result{Item: item.Name, Row: e.Row}
			if e.Err != nil {
				r.Err = e.Err
			} else {
				rctx, cancel := context.WithTimeout(ctx, timeout)
				r.Summary, r.Err = e.Engine.Summarize(rctx, item.Item)
				cancel()
			}
			if r.Err == nil {
				r.Verdicts = Check(r.Summary.Text, item.Key)
				if e.Row.Local && !r.Summary.CostKnown {
					// A local engine sends nothing to a provider and is not billed.
					r.Summary.Cost, r.Summary.CostKnown = 0, true
				}
			} else {
				r.Err = redactedError{err: r.Err}
			}
			results = append(results, r)
		}
	}
	return results
}

// redactedError keeps an adapter's error message free of its configured key and of
// key-shaped text, which the scorecard would otherwise commit.
type redactedError struct {
	err    error
	secret string
}

func (e redactedError) Error() string {
	s := e.err.Error()
	if e.secret != "" {
		s = strings.ReplaceAll(s, e.secret, "[key redacted]")
	}
	return redact(s)
}

func (e redactedError) Unwrap() error { return e.err }

// scrub hides the adapter's own key in its error. A provider can echo any credential it
// received, whatever its shape (ADR 003, Decision 2).
func scrub(err error, key string) error {
	if err == nil {
		return nil
	}
	return redactedError{err: err, secret: key}
}
