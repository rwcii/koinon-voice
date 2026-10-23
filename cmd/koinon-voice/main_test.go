package main

import (
	"bytes"
	"testing"
)

func TestVersion(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run([]string{"version"}, &stdout, &stderr); code != 0 {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	if got := stdout.String(); got != "koinon-voice dev\n" {
		t.Fatalf("stdout %q", got)
	}
}

func TestUsage(t *testing.T) {
	for _, args := range [][]string{nil, {"bogus"}, {"version", "extra"}} {
		var stdout, stderr bytes.Buffer
		if code := run(args, &stdout, &stderr); code != 2 || stdout.Len() != 0 || stderr.Len() == 0 {
			t.Errorf("%q: exit %d stdout %q stderr %q", args, code, stdout.String(), stderr.String())
		}
	}
}
