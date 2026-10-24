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
	// Spoken holds the speech timings when the run speaks; SpeechErr is why speech failed.
	Spoken    *SpokenTiming
	SpeechErr error
}

// SpokenTiming is one summary's speech timing (criterion 1).
type SpokenTiming struct {
	// FirstWord is from sending the summarization request to the first audio sample at the
	// output; Synthesis is from the first complete sentence reaching the helper to that sample.
	FirstWord time.Duration
	Synthesis time.Duration
	Voice     string
}

// Options shape a run.
type Options struct {
	// Timeout bounds each summarization request.
	Timeout time.Duration
	// Voice, when set, speaks each summary as it streams and times the first spoken word.
	Voice Speaker
	// Label speaks an external engine's provider after its summary (ADR 003, Decision 2);
	// the blind rating leaves it off.
	Label bool
	// Order, when set, reorders the engines for each item, so a blind listener cannot tell
	// engines apart by their place.
	Order func(n int, swap func(i, j int))
	// After, when set, runs after each result, once its speech has played; an error stops
	// the run. The blind rating uses it, so each summary is rated as it is heard.
	After func(r *Result) error
}

// speechTimeout bounds one spoken summary, playback included.
const speechTimeout = 3 * time.Minute

// Run summarizes every item with every engine, one request at a time so timings do not
// compete. A failing engine fails its own rows and the run goes on. With a voice, each
// summary is spoken while it streams and the next request waits until it has played. The
// error is After's, with the results so far.
func Run(ctx context.Context, engines []Configured, items []Fixture, opts Options) ([]Result, error) {
	results := make([]Result, 0, len(engines)*len(items))
	for _, item := range items {
		order := append([]Configured(nil), engines...)
		if opts.Order != nil {
			opts.Order(len(order), func(i, j int) { order[i], order[j] = order[j], order[i] })
		}
		for _, e := range order {
			r := Result{Item: item.Name, Row: e.Row}
			if e.Err != nil {
				r.Err = e.Err
			} else {
				summarizeOne(ctx, &r, e, item, opts)
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
			if opts.After != nil {
				if err := opts.After(&r); err != nil {
					return append(results, r), err
				}
			}
			results = append(results, r)
		}
	}
	return results, nil
}

func summarizeOne(ctx context.Context, r *Result, e Configured, item Fixture, opts Options) {
	var u Speech
	var onText func(string)
	if opts.Voice != nil {
		sctx, cancel := context.WithTimeout(ctx, speechTimeout)
		defer cancel()
		var err error
		if u, err = opts.Voice.Start(sctx); err != nil {
			r.SpeechErr = err
		} else {
			onText = u.Write
		}
	}
	rctx, cancel := context.WithTimeout(ctx, opts.Timeout)
	start := time.Now()
	r.Summary, r.Err = e.Engine.Stream(rctx, item.Item, onText)
	cancel()
	if u == nil {
		return
	}
	if r.Err == nil && opts.Label && !e.Row.Local {
		u.Write(" Summary by " + e.Row.Provider + ".")
	}
	sp, err := u.Finish()
	if err != nil {
		r.SpeechErr = err
		return
	}
	if r.Err == nil && !sp.FirstAudio.IsZero() {
		r.Spoken = &SpokenTiming{FirstWord: sp.FirstAudio.Sub(start), Synthesis: sp.FirstAudio.Sub(sp.FirstSentence), Voice: sp.Voice}
	}
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
