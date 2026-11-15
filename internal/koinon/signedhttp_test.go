package koinon

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rwcii/koinon-voice/internal/koinon/koinontest"
	"github.com/rwcii/koinon-voice/internal/signed"
)

type httpRig struct {
	s     *Signed
	node  *koinontest.SignedNode
	route *koinontest.Route
	srv   *httptest.Server
	key   signed.Key
}

func newHTTPRig(t *testing.T) *httpRig {
	t.Helper()
	key := signed.Key{User: "robert", Private: ed25519.NewKeyFromSeed(bytes.Repeat([]byte{3}, 32))}
	node := koinontest.NewSignedNode(nodePin, key.Public(), time.Now)
	node.AddTarget(otherPin)
	auths := map[string]string{nodePin: strings.Repeat("1", 64), otherPin: strings.Repeat("2", 64)}
	node.Change(nodePin, func(tg *koinontest.Target) { tg.Auth = auths[nodePin] })
	node.Change(otherPin, func(tg *koinontest.Target) { tg.Items["P-1"] = item("Approve it"); tg.Auth = auths[otherPin] })
	route := &koinontest.Route{Node: node}
	srv := httptest.NewTLSServer(route)
	t.Cleanup(srv.Close)
	s, err := OpenSignedHTTP(ctx(t), SignedOptions{NodePin: nodePin, Client: "koinon-voice", Key: key, Auths: auths}, srv.URL, srv.Client())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	return &httpRig{s: s, node: node, route: route, srv: srv, key: key}
}

func TestServeVerdict(t *testing.T) {
	r := newHTTPRig(t)
	shown, err := r.s.DecisionGet(ctx(t), otherPin, "P-1")
	if err != nil || shown.Context != "Approve it" {
		t.Fatalf("decision_get %+v %v", shown, err)
	}
	w, _ := r.s.NewVerdict(otherPin, shown, "approved", "")
	var v map[string]any
	if res := r.s.Submit(ctx(t), w, &v); res.State != Recorded || v["verdict"] != "approved" {
		t.Errorf("submit %+v %v", res, v)
	}
	// A refusal keeps its code and details over HTTP 409.
	shown.Revision = 1
	w, _ = r.s.NewVerdict(otherPin, shown, "approved", "")
	res := r.s.Submit(ctx(t), w, nil)
	if cur, ok := res.Refusal.CurrentRevision(); res.State != Refused || !ok || cur != 2 {
		t.Errorf("stale revision %+v", res)
	}
}

func TestServeRenewsOnceForConcurrentCallers(t *testing.T) {
	r := newHTTPRig(t)
	r.route.ExpireTokens()
	before := r.route.Sessions
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := r.s.DecisionGet(ctx(t), otherPin, "P-1"); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if n := r.route.Sessions - before; n != 1 {
		t.Errorf("%d renewals for 8 callers", n)
	}
}

func TestServeWriteKeepsItsEnvelopeAcrossRenewal(t *testing.T) {
	r := newHTTPRig(t)
	shown, _ := r.s.DecisionGet(ctx(t), otherPin, "P-1")
	w, _ := r.s.NewVerdict(otherPin, shown, "approved", "")
	r.route.ExpireTokens()
	if res := r.s.Submit(ctx(t), w, nil); res.State != Recorded {
		t.Fatalf("submit after expiry %+v", res)
	}
	var sent []signed.Envelope
	for _, e := range r.node.Seen {
		if e.Op == "decision_verdict" {
			sent = append(sent, e)
		}
	}
	if len(sent) != 1 || sent[0].Sig != w.Envelope().Sig || sent[0].Request != w.Envelope().Request {
		t.Errorf("the write changed across the renewal: %+v", sent)
	}
}

func TestServeRefusedRenewalDoesNotLoop(t *testing.T) {
	r := newHTTPRig(t)
	r.route.ExpireTokens()
	r.route.RefuseSessions = true
	before := r.route.Sessions
	_, err := r.s.DecisionGet(ctx(t), otherPin, "P-1")
	var ref *Refusal
	if !errors.As(err, &ref) || ref.Code != "signature_refused" {
		t.Errorf("refused renewal: %v", err)
	}
	if n := r.route.Sessions - before; n != 1 {
		t.Errorf("%d session requests", n)
	}
}

func TestServeOtherRefusalsDoNotRenew(t *testing.T) {
	r := newHTTPRig(t)
	before := r.route.Sessions
	r.node.Change(otherPin, func(tg *koinontest.Target) { tg.Auth = strings.Repeat("9", 64) })
	_, err := r.s.DecisionGet(ctx(t), otherPin, "P-1")
	var ref *Refusal
	if !errors.As(err, &ref) || ref.Reason() != "authorization_changed" || r.route.Sessions != before {
		t.Errorf("refusal %v, %d renewals", err, r.route.Sessions-before)
	}
}

func TestServeLostAnswerStaysUnknownAfterDialFailure(t *testing.T) {
	r := newHTTPRig(t)
	shown, _ := r.s.DecisionGet(ctx(t), otherPin, "P-1")
	r.node.Change(otherPin, func(tg *koinontest.Target) { tg.DropNext = true })
	w, _ := r.s.NewVerdict(otherPin, shown, "approved", "")
	if res := r.s.Submit(ctx(t), w, nil); res.State != Unknown {
		t.Fatalf("dropped answer %+v", res)
	}
	r.srv.Close()
	rec := r.s.Reconcile(ctx(t), w, nil)
	if rec.State != Unknown || rec.Final {
		t.Errorf("after a dial failure: %+v", rec)
	}
	// A first sending that cannot connect did not run.
	w2, _ := r.s.NewVerdict(otherPin, shown, "declined", "")
	if res := r.s.Submit(ctx(t), w2, nil); res.State != NotRun || !errors.Is(res.Err, ErrNotDispatched) {
		t.Errorf("first sending without a connection %+v", res)
	}
}

func TestServeRefusesBadAddressesAndAnswers(t *testing.T) {
	key := signed.Key{Private: ed25519.NewKeyFromSeed(bytes.Repeat([]byte{3}, 32))}
	opts := SignedOptions{NodePin: nodePin, Client: "koinon-voice", Key: key, Auths: map[string]string{nodePin: strings.Repeat("1", 64)}}
	for _, base := range []string{"http://node.example", "https://user@node.example", "https://node.example/?q=1", "https://node.example/#f", "https://node.example/path", "node.example"} {
		if _, err := OpenSignedHTTP(ctx(t), opts, base, nil); err == nil {
			t.Errorf("%s was accepted", base)
		}
	}
	// A server whose certificate the client does not trust: refused before any token.
	srv := httptest.NewTLSServer(http.NotFoundHandler())
	defer srv.Close()
	if _, err := OpenSignedHTTP(ctx(t), opts, srv.URL, nil); err == nil {
		t.Error("an untrusted certificate was accepted")
	}

	r := newHTTPRig(t)
	cases := map[string]func(w http.ResponseWriter, req *http.Request) bool{
		"redirect": func(w http.ResponseWriter, req *http.Request) bool {
			http.Redirect(w, req, "https://elsewhere.example/v1/maintainer/signed", http.StatusTemporaryRedirect)
			return true
		},
		"not json": func(w http.ResponseWriter, req *http.Request) bool {
			w.Header().Set("Content-Type", "text/html")
			io.WriteString(w, "<html>")
			return true
		},
		"malformed": func(w http.ResponseWriter, req *http.Request) bool {
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `{"ok":true,"result":`)
			return true
		},
		"too large": func(w http.ResponseWriter, req *http.Request) bool {
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `{"ok":true,"result":"`+strings.Repeat("x", MaxAnswer)+`"}`)
			return true
		},
		"echoed token": func(w http.ResponseWriter, req *http.Request) bool {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(409)
			io.WriteString(w, `{"ok":false,"code":"refused","error":"bad `+req.Header.Get("Authorization")+`","details":{"t":"`+req.Header.Get("Authorization")+`"}}`)
			return true
		},
	}
	for name, hook := range cases {
		r.route.Hook = hook
		_, err := r.s.DecisionGet(ctx(t), otherPin, "P-1")
		if err == nil {
			t.Errorf("%s: no error", name)
			continue
		}
		var ref *Refusal
		text := err.Error()
		if errors.As(err, &ref) {
			text += string(ref.Details)
		}
		if strings.Contains(text, "secretvalue") {
			t.Errorf("%s: the token is in the error: %s", name, text)
		}
	}
	r.route.Hook = nil
	big := strings.Repeat("x", MaxRequest)
	if err := r.s.Read(ctx(t), otherPin, "decision_get", map[string]any{"reference": big}, nil); !errors.Is(err, ErrNotDispatched) {
		t.Errorf("an oversized request: %v", err)
	}
}

func TestServeCloseEndsAChangesWait(t *testing.T) {
	r := newHTTPRig(t)
	r.node.Wait = 20 * time.Second
	done := make(chan error, 1)
	go func() {
		done <- r.s.Read(context.Background(), otherPin, "changes", map[string]any{
			"versions": map[string]any{}, "wait_seconds": 20}, nil)
	}()
	time.Sleep(200 * time.Millisecond)
	r.s.Close()
	select {
	case err := <-done:
		if err == nil {
			t.Error("a closed wait answered")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Close did not end the wait")
	}
	select {
	case <-r.s.Done():
	default:
		t.Error("Done is open after Close")
	}
}
