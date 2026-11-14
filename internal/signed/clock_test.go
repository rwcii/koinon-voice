package signed

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func fixedClock(path string, at *int64) Clock {
	return Clock{Path: path, Now: func() time.Time { return time.Unix(*at, 0) }}
}

func newClock(t *testing.T, at *int64) Clock {
	t.Helper()
	c := fixedClock(filepath.Join(t.TempDir(), "clock.json"), at)
	if err := c.Init("SHA256:k"); err != nil {
		t.Fatal(err)
	}
	return c
}

func next(t *testing.T, c Clock, enroll bool) int64 {
	t.Helper()
	v, err := c.Next("SHA256:k", enroll)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

// Koinon contract fa95824, Decision 1.7: an enrollment at max(now, highest+1); any other
// envelope at max(now, highest, last_enrollment+1).
func TestClockRules(t *testing.T) {
	now := int64(1000)
	c := newClock(t, &now)
	// A burst of ordinary envelopes in one second keeps one value.
	for i := 0; i < 500; i++ {
		if v := next(t, c, false); v != 1000 {
			t.Fatalf("action %d created %d", i, v)
		}
	}
	// Same second: the enrollment is above every earlier envelope, and the session and
	// action after it are one step above its exclusive floor, in the same wall-clock second.
	if v := next(t, c, true); v != 1001 {
		t.Errorf("same-second enrollment %d", v)
	}
	for i := 0; i < 3; i++ {
		if v := next(t, c, false); v != 1002 {
			t.Errorf("envelope after the enrollment %d", v)
		}
	}
	// A future-skewed envelope (T+60), then the clock steps back: the enrollment is above
	// it, and the envelopes after are above the enrollment; nothing waits for the clock.
	now = 1060
	if v := next(t, c, false); v != 1060 {
		t.Errorf("skewed envelope %d", v)
	}
	now = 1001
	if v := next(t, c, true); v != 1061 {
		t.Errorf("enrollment after a step back %d", v)
	}
	if v := next(t, c, false); v != 1062 {
		t.Errorf("envelope after it %d", v)
	}
	// Another process reads the same history from the file.
	now = 900
	if v, err := fixedClock(c.Path, &now).Next("SHA256:k", true); err != nil || v != 1063 {
		t.Errorf("a new clock on the file: %d %v", v, err)
	}
	if info, err := os.Stat(c.Path); err != nil || info.Mode().Perm() != 0o600 {
		t.Errorf("file mode %v %v", info, err)
	}
}

func TestClockHistory(t *testing.T) {
	now := int64(2000)
	c := newClock(t, &now)
	if err := c.Init("SHA256:k"); err == nil {
		t.Error("Init replaced a history")
	}
	// A key with no history is refused, never started again from the clock.
	if _, err := c.Next("SHA256:other", false); !errors.Is(err, ErrNoHistory) {
		t.Errorf("no history: %v", err)
	}
	lost := fixedClock(filepath.Join(t.TempDir(), "clock.json"), &now)
	if _, err := lost.Next("SHA256:k", true); !errors.Is(err, ErrNoHistory) {
		t.Errorf("a missing file: %v", err)
	}
	// Restore starts RestoreMargin ahead, above every envelope that could still be valid.
	v, err := lost.Restore("SHA256:k")
	if err != nil || v != 2000+RestoreMargin {
		t.Errorf("restore %d %v", v, err)
	}
	if v := next(t, lost, false); v != 2000+RestoreMargin+1 {
		t.Errorf("first envelope after restore %d", v)
	}
	// Restore on a history keeps it rising.
	now = 1
	if v, _ := lost.Restore("SHA256:k"); v != 2000+RestoreMargin+2 {
		t.Errorf("second restore %d", v)
	}
}

func TestClockConcurrentSigners(t *testing.T) {
	now := int64(5000)
	c := newClock(t, &now)
	type value struct {
		v      int64
		enroll bool
	}
	var wg sync.WaitGroup
	got := make(chan value, 40)
	for i := 0; i < 40; i++ {
		wg.Add(1)
		go func(enroll bool) {
			defer wg.Done()
			// Each signer opens the file itself, as separate processes do.
			v, err := fixedClock(c.Path, &now).Next("SHA256:k", enroll)
			if err != nil {
				t.Error(err)
			}
			got <- value{v, enroll}
		}(i%4 == 0)
	}
	wg.Wait()
	close(got)
	enrollments, highest := map[int64]bool{}, int64(0)
	for g := range got {
		if g.enroll {
			if enrollments[g.v] {
				t.Errorf("two enrollments at %d", g.v)
			}
			enrollments[g.v] = true
		}
		highest = max(highest, g.v)
	}
	if len(enrollments) != 10 {
		t.Errorf("%d distinct enrollments", len(enrollments))
	}
	if final := next(t, c, false); final != highest && final != highest+1 {
		t.Errorf("after the signers %d, highest handed out %d", final, highest)
	}
}

func TestClockFailuresStopSigning(t *testing.T) {
	now := int64(1)
	c := newClock(t, &now)
	if err := os.WriteFile(c.Path, []byte("not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Next("SHA256:k", true); err == nil {
		t.Error("an unreadable state file was reset")
	}
	if b, _ := os.ReadFile(c.Path); string(b) != "not json" {
		t.Errorf("the state file was changed: %s", b)
	}
	// A directory that cannot be written gives no value.
	dir := filepath.Join(t.TempDir(), "ro")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	ro := fixedClock(filepath.Join(dir, "clock.json"), &now)
	if err := ro.Init("SHA256:k"); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(dir, 0o700) })
	if v, err := ro.Next("SHA256:k", false); err == nil {
		t.Errorf("a clock that could not save gave %d", v)
	}
	if _, err := (Clock{}).Next("k", false); err == nil {
		t.Error("a clock with no file worked")
	}
}
