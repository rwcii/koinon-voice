// Package sidecar runs an engine helper as a separate process and speaks a framed protocol
// with it on stdin and stdout (ADR 002, Decision 1).
//
// A frame is a 4-byte big-endian length, then that many bytes: one kind byte and the
// payload. A JSON frame ('J') holds one Message. An audio frame ('A') holds an 8-byte
// big-endian request ID and then PCM samples in the format the request names.
package sidecar

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

// Version is the protocol version the bridge speaks; a helper's hello must match it.
const Version = 1

// MaxFrame bounds one frame, so a helper that writes junk cannot make the bridge allocate
// without limit.
const MaxFrame = 16 << 20

const (
	kindJSON  = 'J'
	kindAudio = 'A'
)

// Message is one JSON frame. Type is hello, request, input, cancel, event, result or error.
type Message struct {
	Type string `json:"type"`
	ID   uint64 `json:"id,omitempty"`

	// hello
	Protocol     int             `json:"protocol,omitempty"`
	Helper       string          `json:"helper,omitempty"`
	HelperVer    string          `json:"version,omitempty"`
	Capabilities []string        `json:"capabilities,omitempty"`
	Info         json.RawMessage `json:"info,omitempty"`

	// request
	Method     string `json:"method,omitempty"`
	DeadlineMS int64  `json:"deadline_ms,omitempty"`

	// request and input carry Params; input with End closes the request's input.
	Params json.RawMessage `json:"params,omitempty"`
	End    bool            `json:"end,omitempty"`

	// event
	Event string `json:"event,omitempty"`

	// event and result carry Data.
	Data json.RawMessage `json:"data,omitempty"`

	// error
	Code    string `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}

// ErrProtocol is a frame the bridge cannot read: a bad length, kind or JSON.
var ErrProtocol = errors.New("protocol error")

// frame is one decoded frame: a message, or audio for a request.
type frame struct {
	msg   *Message
	id    uint64
	audio []byte
}

// WriteMessage writes one JSON frame. Synthetic helpers in tests use it and ReadMessage.
func WriteMessage(w io.Writer, m Message) error {
	return writeMessage(w, m)
}

// ReadMessage reads the next JSON frame, skipping audio frames.
func ReadMessage(r io.Reader) (Message, error) {
	for {
		f, err := readFrame(r)
		if err != nil {
			return Message{}, err
		}
		if f.msg != nil {
			return *f.msg, nil
		}
	}
}

func writeMessage(w io.Writer, m Message) error {
	body, err := json.Marshal(m)
	if err != nil {
		return err
	}
	return writeFrame(w, kindJSON, body)
}

func writeAudio(w io.Writer, id uint64, pcm []byte) error {
	body := make([]byte, 8+len(pcm))
	binary.BigEndian.PutUint64(body, id)
	copy(body[8:], pcm)
	return writeFrame(w, kindAudio, body)
}

func writeFrame(w io.Writer, kind byte, body []byte) error {
	if len(body)+1 > MaxFrame {
		return fmt.Errorf("frame of %d bytes is over the %d byte limit", len(body)+1, MaxFrame)
	}
	buf := make([]byte, 5+len(body))
	binary.BigEndian.PutUint32(buf, uint32(len(body)+1))
	buf[4] = kind
	copy(buf[5:], body)
	_, err := w.Write(buf)
	return err
}

// readFrame reads one frame. A clean end of stream before a frame is io.EOF; anything
// malformed is ErrProtocol.
func readFrame(r io.Reader) (frame, error) {
	var head [4]byte
	if _, err := io.ReadFull(r, head[:]); err != nil {
		if errors.Is(err, io.ErrUnexpectedEOF) {
			return frame{}, fmt.Errorf("%w: truncated frame length", ErrProtocol)
		}
		return frame{}, err
	}
	n := binary.BigEndian.Uint32(head[:])
	if n == 0 || n > MaxFrame {
		return frame{}, fmt.Errorf("%w: frame length %d", ErrProtocol, n)
	}
	body := make([]byte, n)
	if _, err := io.ReadFull(r, body); err != nil {
		return frame{}, fmt.Errorf("%w: truncated frame: %v", ErrProtocol, err)
	}
	switch body[0] {
	case kindJSON:
		var m Message
		if err := json.Unmarshal(body[1:], &m); err != nil {
			return frame{}, fmt.Errorf("%w: %v", ErrProtocol, err)
		}
		if m.Type == "" {
			return frame{}, fmt.Errorf("%w: message without a type", ErrProtocol)
		}
		return frame{msg: &m}, nil
	case kindAudio:
		if len(body) < 9 {
			return frame{}, fmt.Errorf("%w: audio frame without a request ID", ErrProtocol)
		}
		return frame{id: binary.BigEndian.Uint64(body[1:9]), audio: body[9:]}, nil
	default:
		return frame{}, fmt.Errorf("%w: frame kind %q", ErrProtocol, body[0])
	}
}
