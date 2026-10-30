package speech

import (
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// wavBytes builds a WAV file with the given format fields and data chunk, whose declared size
// may differ from the bytes that follow it.
func wavBytes(channels, bits int, declared uint32, data []byte) []byte {
	b := []byte("RIFF\x00\x00\x00\x00WAVEfmt \x10\x00\x00\x00")
	f := make([]byte, 16)
	binary.LittleEndian.PutUint16(f[0:], 1)
	binary.LittleEndian.PutUint16(f[2:], uint16(channels))
	binary.LittleEndian.PutUint32(f[4:], 16000)
	binary.LittleEndian.PutUint16(f[14:], uint16(bits))
	b = append(b, f...)
	b = append(b, "data"...)
	b = binary.LittleEndian.AppendUint32(b, declared)
	return append(b, data...)
}

func TestReadWAV(t *testing.T) {
	dir := t.TempDir()
	write := func(name string, b []byte) string {
		p := filepath.Join(dir, name)
		os.WriteFile(p, b, 0o600)
		return p
	}
	good := write("good.wav", wavBytes(1, 16, 4, []byte{0, 0x40, 0, 0xC0}))
	a, err := CheckCorpusFile(good)
	if err != nil || a.Frames != 2 || a.Peak != 0.5 {
		t.Fatalf("good: %+v %v", a, err)
	}
	for name, b := range map[string][]byte{
		// A 1-bit header once divided by zero.
		"one-bit":     wavBytes(1, 1, 2, []byte{1, 0}),
		"no channels": wavBytes(0, 16, 2, []byte{1, 0}),
		// The data chunk claims 1024 bytes and has 2: one sample was once scored as a file.
		"truncated": wavBytes(1, 16, 1024, []byte{0, 0x40}),
		// Three bytes of 16-bit samples end inside a frame.
		"partial frame": wavBytes(1, 16, 3, []byte{0, 0x40, 0}),
		"not wav":       []byte("ID3\x03 not a wave file"),
		"no data":       wavBytes(1, 16, 0, nil)[:36],
		"huge fmt":      append([]byte("RIFF\x00\x00\x00\x00WAVEfmt \xff\xff\xff\x7f"), make([]byte, 16)...),
	} {
		if _, err := CheckCorpusFile(write(name+".wav", b)); !errors.Is(err, ErrBadAudio) {
			t.Errorf("%s: %v, want ErrBadAudio", name, err)
		}
	}
	// Formats the bench does not score are read and then refused by the check.
	stereo := write("stereo.wav", wavBytes(2, 16, 4, []byte{0, 0x40, 0, 0x40}))
	if a, err := CheckCorpusFile(stereo); !errors.Is(err, ErrBadAudio) || a.Frames != 1 {
		t.Errorf("stereo: %+v %v", a, err)
	}
	silent := write("silent.wav", wavBytes(1, 16, 4, []byte{0, 0, 0, 0}))
	if _, err := CheckCorpusFile(silent); !errors.Is(err, ErrBadAudio) {
		t.Errorf("silent: %v", err)
	}
	empty := write("empty.wav", wavBytes(1, 16, 0, nil))
	if _, err := CheckCorpusFile(empty); !errors.Is(err, ErrBadAudio) {
		t.Errorf("empty: %v", err)
	}
}

func TestClicks(t *testing.T) {
	path := filepath.Join(t.TempDir(), "clicks.wav")
	// One jump up and one down above 0.3 of full scale; a ramp of small steps is not a click.
	samples := []int16{0, 0, 10000, 0, 1000, 2000, 3000, -ClickJump, 0}
	if err := WriteWAV(path, 24000, samples); err != nil {
		t.Fatal(err)
	}
	a, err := ReadWAV(path)
	if err != nil {
		t.Fatal(err)
	}
	// 0 to 10000, 10000 to 0, 3000 to -9830; -9830 to 0 is exactly the bound, not above it.
	if a.Clicks != 3 {
		t.Errorf("clicks %d, want 3", a.Clicks)
	}
}

func TestSentences(t *testing.T) {
	got := Sentences(`Codex asks you to merge pull request 4.1. "It passes." Claude agrees!  No deadline`)
	want := []string{"Codex asks you to merge pull request 4.1.", `"It passes."`, "Claude agrees!", "No deadline"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("sentences %q", got)
	}
}
