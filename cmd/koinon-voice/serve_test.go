package main

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/rwcii/koinon-voice/internal/koinon/koinontest"
)

// The whole path: serve, the printed link, the page, and a verdict by click that the fake
// daemon records as mcp:koinon-voice.
func TestServe(t *testing.T) {
	hosts := boardHosts(t)
	ctx, cancel := context.WithCancel(context.Background())
	watchContext = func() (context.Context, context.CancelFunc) { return ctx, cancel }
	opened := ""
	openLink = func(link string) error { opened = link; return nil }
	t.Cleanup(func() {
		watchContext = func() (context.Context, context.CancelFunc) { return context.WithCancel(context.Background()) }
		openLink = func(string) error { return nil }
	})
	outR, outW := io.Pipe()
	var stderr bytes.Buffer
	done := make(chan int, 1)
	go func() {
		done <- run([]string{"serve", "--hosts", hosts, "--listen", "127.0.0.1:0", "--no-open"}, nil, outW, &stderr)
		outW.Close()
	}()
	var link string
	lines := bufio.NewScanner(outR)
	var printed strings.Builder
	for lines.Scan() {
		printed.WriteString(lines.Text() + "\n")
		if strings.Contains(lines.Text(), "/login?token=") {
			link = strings.TrimSpace(lines.Text())
			break
		}
	}
	go io.Copy(io.Discard, outR)
	if link == "" || opened != "" {
		t.Fatalf("link %q opened %q", link, opened)
	}
	jar, _ := cookiejar.New(nil)
	c := &http.Client{Jar: jar, Timeout: 5 * time.Second}
	var page string
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); time.Sleep(50 * time.Millisecond) {
		if page == "" {
			resp, err := c.Get(link) // follows the redirect to the page
			if err != nil {
				t.Fatal(err)
			}
			b, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			page = string(b)
		} else {
			u, _ := url.Parse(link)
			resp, err := c.Get("http://" + u.Host + "/board")
			if err != nil {
				t.Fatal(err)
			}
			b, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			page = string(b)
		}
		if strings.Contains(page, "Approve the plan") && strings.Contains(page, "maintainer_secret_invalid") {
			break
		}
	}
	if !strings.Contains(page, "Approve the plan") || !strings.Contains(page, "maintainer_secret_invalid") {
		t.Fatalf("page:\n%s", page)
	}
	csrf := regexp.MustCompile(`name="csrf" value="([0-9a-f]+)"`).FindStringSubmatch(page)[1]
	u, _ := url.Parse(link)
	form := url.Values{"csrf": {csrf}, "host": {"local"}, "reference": {"f-1"}, "revision": {"1"}, "verdict": {"dismissed"}, "note": {"seen"}}
	req, _ := http.NewRequest("POST", "http://"+u.Host+"/verdict", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", "http://"+u.Host)
	resp, err := c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if !strings.Contains(string(b), "Verdict recorded on f-1 (local)") {
		t.Errorf("after the verdict:\n%s", b)
	}
	recorded := koinontest.Control{Dir: filepath.Join(filepath.Dir(hosts), "local")}.Read("verdicts.jsonl")
	if !strings.Contains(recorded, `"via":"mcp:koinon-voice"`) || !strings.Contains(recorded, `"note":"seen"`) {
		t.Errorf("recorded %q", recorded)
	}
	cancel()
	select {
	case code := <-done:
		if code != 0 {
			t.Errorf("exit %d: %s", code, stderr.String())
		}
	case <-time.After(10 * time.Second):
		t.Fatal("serve did not stop")
	}
	if strings.Contains(printed.String()+stderr.String()+page, boardSecret) {
		t.Error("the secret was printed or served")
	}
}

func TestServeRefusesNonLoopback(t *testing.T) {
	var stdout, stderr bytes.Buffer
	for _, addr := range []string{"0.0.0.0:47680", "localhost:47680"} {
		if code := run([]string{"serve", "--listen", addr, "--hosts", filepath.Join(os.TempDir(), "none.toml")}, nil, &stdout, &stderr); code != 1 || !strings.Contains(stderr.String(), "loopback") {
			t.Errorf("%s: exit %d %q", addr, code, stderr.String())
		}
	}
}
