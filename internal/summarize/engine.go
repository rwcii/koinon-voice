package summarize

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"
)

// Engine produces one summary for one item.
type Engine interface {
	Summarize(ctx context.Context, item Item) (Summary, error)
}

// Summary is one engine's result for one item.
type Summary struct {
	Text string
	// InputTokens and OutputTokens are the provider's usage; UsageKnown is false when the
	// response carried none.
	InputTokens  int64
	OutputTokens int64
	UsageKnown   bool
	// Cost is in US dollars; CostKnown is false without usage or a price.
	Cost      float64
	CostKnown bool
	// FirstToken, FirstSentence and Total are measured from the request start.
	FirstToken    time.Duration
	FirstSentence time.Duration
	Total         time.Duration
}

// Price is a row's price in US dollars per million tokens.
type Price struct {
	Input  float64
	Output float64
}

// Known reports whether the row configures a price.
func (p Price) Known() bool { return p.Input > 0 || p.Output > 0 }

// Cost of the usage at this price.
func (p Price) Cost(input, output int64) float64 {
	return (float64(input)*p.Input + float64(output)*p.Output) / 1e6
}

// ErrRefused is a provider's refusal: a failed row, not a summary.
var ErrRefused = errors.New("refused")

// ErrEmpty is an answer with no text.
var ErrEmpty = errors.New("empty answer")

var sentenceEnd = regexp.MustCompile(`[.!?]["')\]]?\s`)

// timer records the streamed timings of one request.
type timer struct {
	start time.Time
	now   func() time.Time
	text  strings.Builder
	s     Summary
	first bool
	sent  bool
}

func newTimer(now func() time.Time) *timer {
	if now == nil {
		now = time.Now
	}
	return &timer{start: now(), now: now}
}

// add appends a streamed text delta.
func (t *timer) add(delta string) {
	if delta == "" {
		return
	}
	elapsed := t.now().Sub(t.start)
	if !t.first && strings.TrimSpace(delta) != "" {
		t.first = true
		t.s.FirstToken = elapsed
	}
	t.text.WriteString(delta)
	if !t.sent && sentenceEnd.MatchString(t.text.String()) {
		t.sent = true
		t.s.FirstSentence = elapsed
	}
}

// finish returns the summary; an answer without text is ErrEmpty.
func (t *timer) finish() (Summary, error) {
	t.s.Total = t.now().Sub(t.start)
	t.s.Text = strings.TrimSpace(t.text.String())
	if t.s.Text == "" {
		return t.s, ErrEmpty
	}
	if !t.sent {
		// The text ended without a sentence break: its first sentence ended with it.
		t.s.FirstSentence = t.s.Total
	}
	return t.s, nil
}

func (t *timer) usage(input, output int64, price Price) {
	t.s.InputTokens, t.s.OutputTokens, t.s.UsageKnown = input, output, true
	if price.Known() {
		t.s.Cost, t.s.CostKnown = price.Cost(input, output), true
	}
}
