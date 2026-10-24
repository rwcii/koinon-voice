package speech

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
)

// Audio describes a WAV file.
type Audio struct {
	Rate, Channels, Bits int
	Frames               int
	Peak                 float64 // largest sample magnitude, 0 to 1
}

// Seconds is the file's duration.
func (a Audio) Seconds() float64 {
	if a.Rate == 0 {
		return 0
	}
	return float64(a.Frames) / float64(a.Rate)
}

// ErrBadAudio is a file the bench does not score: not a 16-bit PCM WAV, not 16 kHz mono,
// empty or silent.
var ErrBadAudio = errors.New("bad audio")

// CheckCorpusFile reads a corpus file and requires what every engine takes: 16 kHz mono
// 16-bit PCM with sound in it.
func CheckCorpusFile(path string) (Audio, error) {
	a, err := ReadWAV(path)
	if err != nil {
		return a, err
	}
	switch {
	case a.Rate != 16000 || a.Channels != 1 || a.Bits != 16:
		return a, fmt.Errorf("%w: %s is %d Hz, %d channels, %d bits; want 16000 Hz mono 16-bit", ErrBadAudio, path, a.Rate, a.Channels, a.Bits)
	case a.Frames == 0:
		return a, fmt.Errorf("%w: %s is empty", ErrBadAudio, path)
	case a.Peak == 0:
		return a, fmt.Errorf("%w: %s is silent", ErrBadAudio, path)
	}
	return a, nil
}

// ReadWAV reads a PCM WAV file's format, length and peak. A malformed file is ErrBadAudio:
// a chunk that runs past the end of the file, a bit depth that is not 8, 16, 24 or 32, or
// data that ends inside a frame.
func ReadWAV(path string) (Audio, error) {
	f, err := os.Open(path)
	if err != nil {
		return Audio{}, err
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return Audio{}, err
	}
	bad := func(format string, args ...any) (Audio, error) {
		return Audio{}, fmt.Errorf("%w: %s %s", ErrBadAudio, path, fmt.Sprintf(format, args...))
	}
	var head [12]byte
	if _, err := io.ReadFull(f, head[:]); err != nil || string(head[0:4]) != "RIFF" || string(head[8:12]) != "WAVE" {
		return bad("is not a WAV file")
	}
	var a Audio
	format := false
	pos := int64(12)
	for {
		var ch [8]byte
		if _, err := io.ReadFull(f, ch[:]); err != nil {
			return bad("has no data chunk")
		}
		pos += 8
		size := int64(binary.LittleEndian.Uint32(ch[4:]))
		// Every chunk must fit in the file, which also bounds what is read into memory.
		if size > fi.Size()-pos {
			return bad("is truncated: its %q chunk claims %d bytes and the file has %d left", ch[0:4], size, fi.Size()-pos)
		}
		switch string(ch[0:4]) {
		case "fmt ":
			if size < 16 {
				return bad("has a short format chunk")
			}
			b := make([]byte, size)
			if _, err := io.ReadFull(f, b); err != nil {
				return bad("has a short format chunk")
			}
			if tag := binary.LittleEndian.Uint16(b[0:]); tag != 1 && tag != 0xFFFE {
				return bad("is not PCM (format %d)", tag)
			}
			a.Channels = int(binary.LittleEndian.Uint16(b[2:]))
			a.Rate = int(binary.LittleEndian.Uint32(b[4:]))
			a.Bits = int(binary.LittleEndian.Uint16(b[14:]))
			switch {
			case a.Channels < 1:
				return bad("has no channels")
			case a.Bits != 8 && a.Bits != 16 && a.Bits != 24 && a.Bits != 32:
				return bad("has %d-bit samples", a.Bits)
			}
			format = true
			if size%2 == 1 {
				if _, err := f.Seek(1, io.SeekCurrent); err != nil { // the chunk's pad byte
					return Audio{}, err
				}
			}
		case "data":
			if !format {
				return bad("has data before its format")
			}
			frame := int64(a.Bits / 8 * a.Channels)
			if size%frame != 0 {
				return bad("ends inside a frame: %d data bytes in %d-byte frames", size, frame)
			}
			a.Frames = int(size / frame)
			if a.Bits != 16 {
				return a, nil // the caller rejects the format; the length is still useful
			}
			b := make([]byte, size)
			if _, err := io.ReadFull(f, b); err != nil {
				return bad("is truncated: %v", err)
			}
			peak := 0
			for i := 0; i+1 < len(b); i += 2 {
				v := int(int16(binary.LittleEndian.Uint16(b[i:])))
				peak = max(peak, v, -v)
			}
			a.Peak = float64(peak) / 32768
			return a, nil
		default:
			if _, err := f.Seek(size+size%2, io.SeekCurrent); err != nil {
				return Audio{}, err
			}
		}
		pos += size + size%2
	}
}

// WriteWAV writes 16-bit mono PCM samples, for tests and synthetic files.
func WriteWAV(path string, rate int, samples []int16) error {
	b := make([]byte, 44+2*len(samples))
	copy(b[0:], "RIFF")
	binary.LittleEndian.PutUint32(b[4:], uint32(36+2*len(samples)))
	copy(b[8:], "WAVEfmt ")
	binary.LittleEndian.PutUint32(b[16:], 16)
	binary.LittleEndian.PutUint16(b[20:], 1)
	binary.LittleEndian.PutUint16(b[22:], 1)
	binary.LittleEndian.PutUint32(b[24:], uint32(rate))
	binary.LittleEndian.PutUint32(b[28:], uint32(rate*2))
	binary.LittleEndian.PutUint16(b[32:], 2)
	binary.LittleEndian.PutUint16(b[34:], 16)
	copy(b[36:], "data")
	binary.LittleEndian.PutUint32(b[40:], uint32(2*len(samples)))
	for i, s := range samples {
		binary.LittleEndian.PutUint16(b[44+2*i:], uint16(s))
	}
	return os.WriteFile(path, b, 0o600)
}
