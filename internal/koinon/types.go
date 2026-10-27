// Package koinon connects Koinon Voice to Koinon daemons through Koinon's maintainer mode,
// `koinon mcp --maintainer`, on stdio: locally as a child process, or on a remote host
// through ssh (ADR 001).
package koinon

import (
	"encoding/json"
	"errors"
	"fmt"
)

// Counts are the decision board's counts, as board_list gives them.
type Counts struct {
	Waiting    int `json:"waiting"`
	InProgress int `json:"in_progress"`
	OpenFYI    int `json:"open_fyi"`
	Archived   int `json:"archived"`
}

// Summary is one decision item as board_list lists it.
type Summary struct {
	Reference string `json:"reference"`
	Title     string `json:"title"`
	Kind      string `json:"kind"`
	State     string `json:"state"`
	Revision  int64  `json:"revision"`
	Order     int64  `json:"order"`
}

type boardPage struct {
	Counts    Counts    `json:"counts"`
	Items     []Summary `json:"items"`
	More      bool      `json:"more"`
	NextAfter *int64    `json:"next_after"`
}

// Item is one decision item as decision_get gives it.
type Item struct {
	Reference      string   `json:"reference"`
	Order          int64    `json:"order"`
	Title          string   `json:"title"`
	Where          string   `json:"where"`
	AgentText      string   `json:"agent_text"`
	Context        string   `json:"context"`
	Recommendation string   `json:"recommendation"`
	Kind           string   `json:"kind"`
	State          string   `json:"state"`
	Revision       int64    `json:"revision"`
	Commands       []string `json:"commands"`
	AuthorFamily   string   `json:"author_family"`
	AuthorName     string   `json:"author_name"`
	CreatedAt      int64    `json:"created_at"`
	UpdatedAt      int64    `json:"updated_at"`
}

// Message is one message in the maintainer's inbox.
type Message struct {
	ID           int64  `json:"id"`
	Seq          int64  `json:"seq"`
	SenderFamily string `json:"sender_family"`
	SenderName   string `json:"sender_name"`
	Body         string `json:"body"`
	CreatedAt    int64  `json:"created_at"`
}

type inboxPage struct {
	Messages     []Message `json:"messages"`
	LastSeq      int64     `json:"last_seq"`
	AckedThrough int64     `json:"acked_through"`
	More         bool      `json:"more"`
}

// Room is one room as room_list lists it.
type Room struct {
	RoomID    string `json:"room_id"`
	Name      string `json:"name"`
	Revision  int64  `json:"revision"`
	CreatedAt int64  `json:"created_at"`
}

// Post is one retained room post.
type Post struct {
	Seq          int64  `json:"seq"`
	SenderFamily string `json:"sender_family"`
	SenderName   string `json:"sender_name"`
	Origin       string `json:"origin"`
	Body         string `json:"body"`
	CreatedAt    int64  `json:"created_at"`
}

type roomPage struct {
	RoomID string `json:"room_id"`
	Posts  []Post `json:"posts"`
	More   bool   `json:"more"`
}

// Verdict is a verdict that Koinon recorded, with the client that gave it in Via.
type Verdict struct {
	VerdictID int64  `json:"verdict_id"`
	Revision  int64  `json:"revision"`
	Verdict   string `json:"verdict"`
	Note      string `json:"note"`
	At        int64  `json:"at"`
	Via       string `json:"via"`
}

// Refusal is Koinon's refusal of a tool call or resource read, with its code.
type Refusal struct {
	Code    string
	Message string
	Details json.RawMessage
}

func (r *Refusal) Error() string {
	if r.Message == "" {
		return r.Code
	}
	return r.Code + ": " + r.Message
}

// CurrentRevision is the item's revision that a stale_revision refusal names.
func (r *Refusal) CurrentRevision() (int64, bool) {
	var d struct {
		CurrentRevision *int64 `json:"current_revision"`
	}
	if r.Code != "stale_revision" || json.Unmarshal(r.Details, &d) != nil || d.CurrentRevision == nil {
		return 0, false
	}
	return *d.CurrentRevision, true
}

// RPCError is a JSON-RPC error answer, such as -32603 with the data code
// daemon_unavailable.
type RPCError struct {
	Code    int
	Message string
	Data    json.RawMessage
}

func (e *RPCError) Error() string {
	if c := e.DataCode(); c != "" {
		return fmt.Sprintf("json-rpc %d: %s", e.Code, c)
	}
	return fmt.Sprintf("json-rpc %d: %s", e.Code, e.Message)
}

// DataCode is the Koinon code in the error's data, if any.
func (e *RPCError) DataCode() string {
	var d struct {
		Code string `json:"code"`
	}
	_ = json.Unmarshal(e.Data, &d)
	return d.Code
}

// Code returns the Koinon code that err carries, or "".
func Code(err error) string {
	var r *Refusal
	if errors.As(err, &r) {
		return r.Code
	}
	var e *RPCError
	if errors.As(err, &e) {
		return e.DataCode()
	}
	var d *Down
	if errors.As(err, &d) {
		return d.Code
	}
	return ""
}
