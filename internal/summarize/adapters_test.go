package summarize

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

var testItem = Item{
	Reference: "T-1", Title: "Merge pull request 12", Kind: "decision",
	AgentText: "Checks pass.", Recommendation: "Merge it.", AuthorFamily: "codex", AuthorName: "codex-koinon-voice-38",
}

// recorder is an httptest handler that keeps each request body and answers with a script.
type recorder struct {
	mu     sync.Mutex
	bodies []map[string]any
	paths  []string
	auth   []string
	reply  func(w http.ResponseWriter)
}

func (rec *recorder) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	data, _ := io.ReadAll(r.Body)
	var body map[string]any
	_ = json.Unmarshal(data, &body)
	rec.mu.Lock()
	rec.bodies = append(rec.bodies, body)
	rec.paths = append(rec.paths, r.URL.Path)
	rec.auth = append(rec.auth, r.Header.Get("Authorization")+r.Header.Get("X-Api-Key"))
	rec.mu.Unlock()
	rec.reply(w)
}

func sse(w http.ResponseWriter, events ...string) {
	w.Header().Set("Content-Type", "text/event-stream")
	for _, e := range events {
		fmt.Fprint(w, e)
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
	}
}

func anthropicEvents(stop string, texts ...string) []string {
	ev := func(name, data string) string { return "event: " + name + "\ndata: " + data + "\n\n" }
	out := []string{
		ev("message_start", `{"type":"message_start","message":{"id":"msg_1","type":"message","role":"assistant","model":"m","content":[],"stop_reason":null,"usage":{"input_tokens":200,"output_tokens":1}}}`),
		ev("content_block_start", `{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`),
	}
	for _, t := range texts {
		b, _ := json.Marshal(t)
		out = append(out, ev("content_block_delta", `{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":`+string(b)+`}}`))
	}
	out = append(out,
		ev("content_block_stop", `{"type":"content_block_stop","index":0}`),
		ev("message_delta", `{"type":"message_delta","delta":{"stop_reason":"`+stop+`","stop_details":null},"usage":{"output_tokens":40}}`),
		ev("message_stop", `{"type":"message_stop"}`),
	)
	return out
}

func newAnthropic(url string) *Anthropic {
	return &Anthropic{Model: "claude-haiku-5-5", BaseURL: url, Key: "test-key", Effort: "low", Price: Price{Input: 1, Output: 5}}
}

func TestAnthropicSummary(t *testing.T) {
	rec := &recorder{reply: func(w http.ResponseWriter) {
		sse(w, anthropicEvents("end_turn", "Codex asks you to merge pull request twelve. ", "It recommends merging.")...)
	}}
	srv := httptest.NewServer(rec)
	defer srv.Close()

	s, err := newAnthropic(srv.URL).Summarize(context.Background(), testItem)
	if err != nil {
		t.Fatal(err)
	}
	if s.Text != "Codex asks you to merge pull request twelve. It recommends merging." {
		t.Errorf("text %q", s.Text)
	}
	if !s.UsageKnown || s.InputTokens != 200 || s.OutputTokens != 40 {
		t.Errorf("usage %+v", s)
	}
	if want := (200*1.0 + 40*5.0) / 1e6; !s.CostKnown || s.Cost != want {
		t.Errorf("cost %v %v, want %v", s.Cost, s.CostKnown, want)
	}
	if s.FirstToken <= 0 || s.FirstSentence < s.FirstToken || s.Total < s.FirstSentence {
		t.Errorf("timings %v %v %v", s.FirstToken, s.FirstSentence, s.Total)
	}
	body := rec.bodies[0]
	if rec.paths[0] != "/v1/messages" || body["model"] != "claude-haiku-5-5" || body["stream"] != true {
		t.Errorf("request %s %v", rec.paths[0], body)
	}
	if !strings.Contains(fmt.Sprint(body["system"]), "spoken summary") {
		t.Errorf("system %v", body["system"])
	}
	if !strings.Contains(fmt.Sprint(body["messages"]), "Merge pull request 12") {
		t.Errorf("messages %v", body["messages"])
	}
	if fmt.Sprint(body["output_config"]) != "map[effort:low]" {
		t.Errorf("output_config %v", body["output_config"])
	}
	if _, ok := body["temperature"]; ok {
		t.Error("temperature sent")
	}
	if rec.auth[0] != "test-key" {
		t.Errorf("auth %q", rec.auth[0])
	}
}

func TestAnthropicFailures(t *testing.T) {
	cases := map[string]struct {
		reply func(w http.ResponseWriter)
		want  string
		is    error
	}{
		"server error": {func(w http.ResponseWriter) {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprint(w, `{"type":"error","error":{"type":"api_error","message":"boom"}}`)
		}, "500", nil},
		"client error": {func(w http.ResponseWriter) {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(w, `{"type":"error","error":{"type":"invalid_request_error","message":"bad model"}}`)
		}, "400", nil},
		"refusal":    {func(w http.ResponseWriter) { sse(w, anthropicEvents("refusal")...) }, "", ErrRefused},
		"empty":      {func(w http.ResponseWriter) { sse(w, anthropicEvents("end_turn", "  ")...) }, "", ErrEmpty},
		"nothing":    {func(w http.ResponseWriter) { sse(w) }, "", ErrIncomplete},
		"max tokens": {func(w http.ResponseWriter) { sse(w, anthropicEvents("max_tokens", "Codex asks you to")...) }, "max_tokens", ErrIncomplete},
		"cut off": {func(w http.ResponseWriter) {
			events := anthropicEvents("end_turn", "Codex asks you to")
			sse(w, events[:3]...)
		}, "", ErrIncomplete},
		"echoed key": {func(w http.ResponseWriter) {
			w.WriteHeader(http.StatusUnauthorized)
			fmt.Fprint(w, `{"type":"error","error":{"type":"authentication_error","message":"invalid key test-key"}}`)
		}, "[key redacted]", nil},
		"malformed": {func(w http.ResponseWriter) {
			sse(w, "event: message_start\ndata: {not json\n\n")
		}, "", nil},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			srv := httptest.NewServer(&recorder{reply: c.reply})
			defer srv.Close()
			_, err := newAnthropic(srv.URL).Summarize(context.Background(), testItem)
			if err == nil {
				t.Fatal("no error")
			}
			if strings.Contains(err.Error(), "test-key") {
				t.Errorf("error keeps the key: %v", err)
			}
			if c.is != nil && !errors.Is(err, c.is) {
				t.Errorf("error %v, want %v", err, c.is)
			}
			if c.want != "" && !strings.Contains(err.Error(), c.want) {
				t.Errorf("error %v, want %q", err, c.want)
			}
		})
	}
}

func TestAnthropicTimeout(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(&recorder{reply: func(w http.ResponseWriter) { <-release }})
	defer srv.Close()
	defer close(release)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if _, err := newAnthropic(srv.URL).Summarize(ctx, testItem); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error %v", err)
	}
}

func openAIChunk(content string) string {
	b, _ := json.Marshal(content)
	return `data: {"choices":[{"delta":{"content":` + string(b) + `}}]}` + "\n\n"
}

func newOpenAI(url string) *OpenAI {
	return &OpenAI{BaseURL: url + "/v1", Model: "local-model", Price: Price{Input: 2, Output: 8}}
}

func TestOpenAISummary(t *testing.T) {
	rec := &recorder{reply: func(w http.ResponseWriter) {
		sse(w, ": keep-alive\n\n", openAIChunk("Codex asks"), openAIChunk(" you to merge. "), openAIChunk("It recommends it."),
			`data: {"choices":[{"delta":{},"finish_reason":"stop"}]}`+"\n\n",
			`data: {"choices":[],"usage":{"prompt_tokens":150,"completion_tokens":30}}`+"\n\n",
			"data: [DONE]\n\n")
	}}
	srv := httptest.NewServer(rec)
	defer srv.Close()

	o := newOpenAI(srv.URL)
	o.Key = "router-key"
	s, err := o.Summarize(context.Background(), testItem)
	if err != nil {
		t.Fatal(err)
	}
	if s.Text != "Codex asks you to merge. It recommends it." {
		t.Errorf("text %q", s.Text)
	}
	if want := (150*2.0 + 30*8.0) / 1e6; !s.CostKnown || s.Cost != want || s.InputTokens != 150 || s.OutputTokens != 30 {
		t.Errorf("usage %+v, want cost %v", s, want)
	}
	body := rec.bodies[0]
	if rec.paths[0] != "/v1/chat/completions" || body["model"] != "local-model" || body["stream"] != true {
		t.Errorf("request %s %v", rec.paths[0], body)
	}
	if fmt.Sprint(body["stream_options"]) != "map[include_usage:true]" {
		t.Errorf("stream_options %v", body["stream_options"])
	}
	msgs := fmt.Sprint(body["messages"])
	if !strings.Contains(msgs, "spoken summary") || !strings.Contains(msgs, "Merge pull request 12") {
		t.Errorf("messages %v", msgs)
	}
	if _, ok := body["provider"]; ok {
		t.Errorf("provider routing sent unasked: %v", body["provider"])
	}
	o.DenyDataCollection = true
	if _, err := o.Summarize(context.Background(), testItem); err != nil {
		t.Fatal(err)
	}
	if got := fmt.Sprint(rec.bodies[1]["provider"]); got != "map[data_collection:deny]" {
		t.Errorf("provider %s", got)
	}
	if rec.auth[0] != "Bearer router-key" {
		t.Errorf("auth %q", rec.auth[0])
	}
}

func TestOpenAIUsage(t *testing.T) {
	cases := map[string]struct {
		usage     string
		known     bool
		costKnown bool
		cost      float64
	}{
		"none":             {"", false, false, 0},
		"provider cost":    {`{"prompt_tokens":10,"completion_tokens":5,"cost":0.0042}`, true, true, 0.0042},
		"no price or cost": {`{"prompt_tokens":10,"completion_tokens":5}`, true, false, 0},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			srv := httptest.NewServer(&recorder{reply: func(w http.ResponseWriter) {
				events := []string{openAIChunk("A summary.")}
				if c.usage != "" {
					events = append(events, `data: {"choices":[],"usage":`+c.usage+`}`+"\n\n")
				}
				sse(w, append(events, "data: [DONE]\n\n")...)
			}})
			defer srv.Close()
			o := newOpenAI(srv.URL)
			if name == "no price or cost" {
				o.Price = Price{}
			}
			s, err := o.Summarize(context.Background(), testItem)
			if err != nil {
				t.Fatal(err)
			}
			if s.UsageKnown != c.known || s.CostKnown != c.costKnown || s.Cost != c.cost {
				t.Errorf("summary %+v", s)
			}
		})
	}
}

func TestOpenAIFailures(t *testing.T) {
	cases := map[string]struct {
		reply func(w http.ResponseWriter)
		want  string
		is    error
	}{
		"unauthorized": {func(w http.ResponseWriter) {
			w.WriteHeader(http.StatusUnauthorized)
			fmt.Fprint(w, `{"error":{"message":"Incorrect API key provided: sk-svcac************WXYZ."}}`)
		}, "HTTP 401: {\"error\":{\"message\":\"Incorrect API key provided: [key redacted]", nil},
		"server error": {func(w http.ResponseWriter) { w.WriteHeader(http.StatusBadGateway) }, "HTTP 502", nil},
		"malformed":    {func(w http.ResponseWriter) { sse(w, "data: {nope\n\n") }, "malformed", nil},
		"nothing":      {func(w http.ResponseWriter) { sse(w, "data: [DONE]\n\n") }, "", ErrEmpty},
		"closed early": {func(w http.ResponseWriter) { sse(w) }, "", ErrIncomplete},
		"partial text": {func(w http.ResponseWriter) { sse(w, openAIChunk("Codex asks you to")) }, "", ErrIncomplete},
		"length": {func(w http.ResponseWriter) {
			sse(w, openAIChunk("Codex asks you to"), `data: {"choices":[{"delta":{},"finish_reason":"length"}]}`+"\n\n", "data: [DONE]\n\n")
		}, "length", ErrIncomplete},
		"echoed key": {func(w http.ResponseWriter) {
			w.WriteHeader(http.StatusUnauthorized)
			fmt.Fprint(w, `{"error":{"message":"bad credential synthetic-api-secret-123456"}}`)
		}, "bad credential [key redacted]", nil},
		"refusal": {func(w http.ResponseWriter) {
			sse(w, `data: {"choices":[{"delta":{"refusal":"I can't help."}}]}`+"\n\n", "data: [DONE]\n\n")
		}, "", ErrRefused},
		"content filter": {func(w http.ResponseWriter) {
			sse(w, openAIChunk("Part"), `data: {"choices":[{"delta":{},"finish_reason":"content_filter"}]}`+"\n\n")
		}, "", ErrRefused},
		"stream error": {func(w http.ResponseWriter) {
			sse(w, `data: {"error":{"message":"overloaded for sk-or-v1-abcdef123"}}`+"\n\n")
		}, "overloaded for [key redacted]", nil},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			srv := httptest.NewServer(&recorder{reply: c.reply})
			defer srv.Close()
			o := newOpenAI(srv.URL)
			o.Key = "synthetic-api-secret-123456" // any shape a compatible server accepts
			_, err := o.Summarize(context.Background(), testItem)
			if err == nil {
				t.Fatal("no error")
			}
			if c.is != nil && !errors.Is(err, c.is) {
				t.Errorf("error %v, want %v", err, c.is)
			}
			if c.want != "" && !strings.Contains(err.Error(), c.want) {
				t.Errorf("error %v, want %q", err, c.want)
			}
			if strings.Contains(err.Error(), "WXYZ") || strings.Contains(err.Error(), "abcdef") || strings.Contains(err.Error(), "synthetic-api-secret") {
				t.Errorf("error keeps key text: %v", err)
			}
		})
	}
}

func TestOpenAITimeout(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(&recorder{reply: func(w http.ResponseWriter) {
		sse(w, openAIChunk("Slow"))
		<-release
	}})
	defer srv.Close()
	defer close(release)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if _, err := newOpenAI(srv.URL).Summarize(ctx, testItem); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error %v", err)
	}
}

func TestTimerFirstSentence(t *testing.T) {
	clock := time.Unix(0, 0)
	tm := newTimer(func() time.Time { return clock })
	step := func(d string) {
		clock = clock.Add(10 * time.Millisecond)
		tm.add(d)
	}
	step(" ")
	step("Version 0.4")
	step(".1 shipped. ")
	step("Done")
	clock = clock.Add(10 * time.Millisecond)
	s, err := tm.finish()
	if err != nil {
		t.Fatal(err)
	}
	if s.FirstToken != 20*time.Millisecond || s.FirstSentence != 30*time.Millisecond || s.Total != 50*time.Millisecond {
		t.Errorf("timings %v %v %v", s.FirstToken, s.FirstSentence, s.Total)
	}
}

// A compatible server that echoes the credential it received must not put it in Run's
// error or the scorecard, whatever the key's shape.
func TestEchoedKeyStaysOutOfScorecard(t *testing.T) {
	const secret = "synthetic-api-secret-123456"
	srv := httptest.NewServer(&recorder{reply: func(w http.ResponseWriter) {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `{"error":{"message":"key `+secret+` is not valid"}}`)
	}})
	defer srv.Close()
	row := Row{Name: "compat", Adapter: "openai", BaseURL: srv.URL + "/v1", Model: "m", Provider: "Example", Key: "compat"}
	e, err := Build(context.Background(), row, staticKeys{secret}, nil)
	if err != nil {
		t.Fatal(err)
	}
	results := Run(context.Background(), []Configured{{Row: row, Engine: e}}, []Fixture{{Name: "SYN-01", Item: testItem}}, time.Second)
	var b strings.Builder
	if err := WriteScorecard(&b, results, []Row{row}); err != nil {
		t.Fatal(err)
	}
	if results[0].Err == nil || strings.Contains(results[0].Err.Error(), secret) || strings.Contains(b.String(), secret) {
		t.Fatalf("key reached output: %v\n%s", results[0].Err, b.String())
	}
	if !strings.Contains(b.String(), "[key redacted]") {
		t.Errorf("scorecard lacks the redacted error:\n%s", b.String())
	}
}

type staticKeys struct{ key string }

func (k staticKeys) Key(context.Context, string) (string, error) { return k.key, nil }
