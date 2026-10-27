package dashboard

import (
	"encoding/json"
	"os/exec"
	"testing"
)

// TestScript runs the page's script under Node with a stand-in DOM (review of cb3fbd3,
// F1 to F3). It is skipped where Node is not installed.
func TestScript(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is not installed")
	}
	out, err := exec.Command(node, "testdata/script.cjs", "assets/app.js").CombinedOutput()
	if err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	var got map[string]any
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	want := map[string]any{
		"focusedNote":        "typed during the read",
		"focusedBoard":       "initial",
		"afterFocusoutBoard": "revision 2",
		"afterFocusoutNote":  "typed during the read",
		"unfocusedNote":      "unfocused note",
		"boardBeforeSecond":  "initial",
		"coalescedBoard":     "revision 3",
		"coalescedReads":     float64(2),
		"readsOnOpen":        float64(1),
		"openBoard":          "after open",
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s = %v, want %v", k, got[k], v)
		}
	}
}
