package summarize

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// OpenAI summarizes through an OpenAI-compatible chat completions endpoint: OpenAI,
// OpenRouter or a local server such as oMLX (ADR 003, Decision 1).
type OpenAI struct {
	// BaseURL ends before /chat/completions, for example https://api.openai.com/v1.
	BaseURL   string
	Model     string
	Key       string
	MaxTokens int64
	Price     Price
	// DenyDataCollection asks OpenRouter to route only to hosts that neither retain nor
	// train on prompts (ADR 003, Decision 2).
	DenyDataCollection bool
	HTTP               *http.Client
	now                func() time.Time
}

// keyShaped matches API keys and providers' masked echoes of them, such as OpenAI's
// "sk-svcac****…WXYZ", which keep enough of the key to identify it.
var keyShaped = regexp.MustCompile(`\b(sk|pk|rk)-[A-Za-z0-9_*.-]{6,}`)

// redact removes key-shaped text from a provider's error before it reaches a log or the
// scorecard (ADR 003, Decision 2).
func redact(s string) string { return keyShaped.ReplaceAllString(s, "[key redacted]") }

type chatRequest struct {
	Model               string        `json:"model"`
	Messages            []chatMessage `json:"messages"`
	Stream              bool          `json:"stream"`
	StreamOptions       streamOptions `json:"stream_options"`
	MaxCompletionTokens int64         `json:"max_completion_tokens,omitempty"`
	Provider            *routing      `json:"provider,omitempty"`
}

type routing struct {
	DataCollection string `json:"data_collection"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type streamOptions struct {
	IncludeUsage bool `json:"include_usage"`
}

type chatChunk struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
			Refusal string `json:"refusal"`
		} `json:"delta"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage *struct {
		PromptTokens     int64    `json:"prompt_tokens"`
		CompletionTokens int64    `json:"completion_tokens"`
		Cost             *float64 `json:"cost"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// Summarize streams one summary. A refusal, a content filter stop, an unfinished stream or
// an empty answer is an error. A response without usage leaves the cost unknown.
func (o *OpenAI) Summarize(ctx context.Context, item Item) (Summary, error) {
	return o.Stream(ctx, item, nil)
}

// Stream is Summarize with each text delta handed to onText.
func (o *OpenAI) Stream(ctx context.Context, item Item, onText func(string)) (Summary, error) {
	s, err := o.summarize(ctx, item, onText)
	return s, scrub(err, o.Key)
}

func (o *OpenAI) summarize(ctx context.Context, item Item, onText func(string)) (Summary, error) {
	request := chatRequest{
		Model: o.Model,
		Messages: []chatMessage{
			{Role: "system", Content: Instructions},
			{Role: "user", Content: Prompt(item)},
		},
		Stream:              true,
		StreamOptions:       streamOptions{IncludeUsage: true},
		MaxCompletionTokens: o.MaxTokens,
	}
	if o.DenyDataCollection {
		request.Provider = &routing{DataCollection: "deny"}
	}
	body, err := json.Marshal(request)
	if err != nil {
		return Summary{}, err
	}
	url := strings.TrimSuffix(o.BaseURL, "/") + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return Summary{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	if o.Key != "" {
		req.Header.Set("Authorization", "Bearer "+o.Key)
	}
	client := o.HTTP
	if client == nil {
		client = http.DefaultClient
	}

	t := newTimer(o.now, onText)
	resp, err := client.Do(req)
	if err != nil {
		return Summary{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		detail, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return Summary{}, fmt.Errorf("HTTP %d: %s", resp.StatusCode, redact(strings.Join(strings.Fields(string(detail)), " ")))
	}

	var refusal strings.Builder
	finish, done := "", false
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 64*1024), 1<<20)
	for scanner.Scan() {
		line := scanner.Text()
		data, ok := strings.CutPrefix(line, "data:")
		if !ok {
			continue // blank separators, comments and other SSE fields
		}
		data = strings.TrimSpace(data)
		if data == "[DONE]" {
			done = true
			break
		}
		var chunk chatChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			return Summary{}, fmt.Errorf("malformed stream chunk: %w", err)
		}
		if chunk.Error != nil {
			return Summary{}, fmt.Errorf("provider error: %s", redact(chunk.Error.Message))
		}
		for _, c := range chunk.Choices {
			t.add(c.Delta.Content)
			refusal.WriteString(c.Delta.Refusal)
			if c.FinishReason != "" {
				finish, done = c.FinishReason, true
			}
		}
		if u := chunk.Usage; u != nil {
			t.usage(u.PromptTokens, u.CompletionTokens, o.Price)
			if u.Cost != nil {
				// OpenRouter reports what it charged; that beats a configured price.
				t.s.Cost, t.s.CostKnown = *u.Cost, true
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return Summary{}, fmt.Errorf("stream: %w", err)
	}
	if refusal.Len() > 0 || finish == "content_filter" {
		s, _ := t.finish()
		return s, fmt.Errorf("%w: %s", ErrRefused, strings.TrimSpace(refusal.String()+" "+finish))
	}
	switch {
	case !done:
		s, _ := t.finish()
		return s, fmt.Errorf("%w: the stream ended without a finish reason or [DONE]", ErrIncomplete)
	case finish == "length":
		s, _ := t.finish()
		return s, fmt.Errorf("%w: finish reason length", ErrIncomplete)
	}
	return t.finish()
}
