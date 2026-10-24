package summarize

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

// Anthropic summarizes with Claude through the official Go SDK, streaming so time to first
// token is measured (ADR 003, Decision 1).
type Anthropic struct {
	Model     string
	BaseURL   string
	Key       string
	Effort    string
	MaxTokens int64
	Price     Price
	HTTP      *http.Client
	now       func() time.Time
}

// Summarize streams one summary. A refusal, an unfinished answer or an empty answer is an
// error.
func (a *Anthropic) Summarize(ctx context.Context, item Item) (Summary, error) {
	s, err := a.summarize(ctx, item)
	return s, scrub(err, a.Key)
}

func (a *Anthropic) summarize(ctx context.Context, item Item) (Summary, error) {
	opts := []option.RequestOption{
		option.WithAPIKey(a.Key),
		// A retry would hide the latency being measured; a failed row says what happened.
		option.WithMaxRetries(0),
	}
	if a.BaseURL != "" {
		opts = append(opts, option.WithBaseURL(a.BaseURL))
	}
	if a.HTTP != nil {
		opts = append(opts, option.WithHTTPClient(a.HTTP))
	}
	client := anthropic.NewClient(opts...)
	maxTokens := a.MaxTokens
	if maxTokens == 0 {
		maxTokens = 4000
	}
	params := anthropic.MessageNewParams{
		Model:     anthropic.Model(a.Model),
		MaxTokens: maxTokens,
		System:    []anthropic.TextBlockParam{{Text: Instructions}},
		Messages:  []anthropic.MessageParam{anthropic.NewUserMessage(anthropic.NewTextBlock(Prompt(item)))},
	}
	if a.Effort != "" {
		params.OutputConfig = anthropic.OutputConfigParam{Effort: anthropic.OutputConfigEffort(a.Effort)}
	}

	t := newTimer(a.now)
	stream := client.Messages.NewStreaming(ctx, params)
	defer stream.Close()
	var message anthropic.Message
	for stream.Next() {
		event := stream.Current()
		if err := message.Accumulate(event); err != nil {
			return Summary{}, fmt.Errorf("stream: %w", err)
		}
		if delta, ok := event.AsAny().(anthropic.ContentBlockDeltaEvent); ok {
			if text, ok := delta.Delta.AsAny().(anthropic.TextDelta); ok {
				t.add(text.Text)
			}
		}
	}
	if err := stream.Err(); err != nil {
		return Summary{}, err
	}
	if message.JSON.Usage.Valid() {
		t.usage(message.Usage.InputTokens, message.Usage.OutputTokens, a.Price)
	}
	if message.StopReason == anthropic.StopReasonRefusal {
		s, _ := t.finish()
		return s, fmt.Errorf("%w (category %q)", ErrRefused, message.StopDetails.Category)
	}
	switch message.StopReason {
	case anthropic.StopReasonEndTurn, anthropic.StopReasonStopSequence:
		return t.finish()
	case "":
		s, _ := t.finish()
		return s, fmt.Errorf("%w: the stream ended without a stop reason", ErrIncomplete)
	default:
		s, _ := t.finish()
		return s, fmt.Errorf("%w: stop reason %s", ErrIncomplete, message.StopReason)
	}
}
