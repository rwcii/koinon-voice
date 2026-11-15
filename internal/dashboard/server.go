// Package dashboard serves Koinon Voice's dashboard on a loopback address: each host's
// connection state, the items that wait for the maintainer, the maintainer's inbox, and a
// verdict by click (ADR 001, Decision 1). Its login, Host, Origin and CSRF checks follow
// Koinon's dashboard (docs/sprints/2026-10-23-phase0-spike/koinon-map.md, Koinon PROTOCOL.md
// "Request protection").
package dashboard

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"embed"
	"encoding/hex"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"

	"github.com/rwcii/koinon-voice/internal/koinon"
)

//go:embed assets
var assets embed.FS

// LinkLifetime is how long a login link works.
const LinkLifetime = 60 * time.Second

// CookieName is the session cookie.
const CookieName = "koinon_voice_session"

// Host is one connector, as the dashboard uses it.
type Host interface {
	State() koinon.State
	Verdict(ctx context.Context, reference string, revision int64, verdict, note string) (koinon.Verdict, error)
}

// SignedSource is a signed row: it shows the node and its computers, each by pin, and gives
// verdicts on the item as the page showed it (ADR 004).
type SignedSource interface {
	States() []koinon.State
	Verdict(ctx context.Context, req koinon.VerdictRequest) koinon.VerdictResult
	Dismiss(request string) error
	// NewIntent issues the token of one rendered verdict form.
	NewIntent() string
}

// Server is the dashboard. Its zero value is not usable; make it with New.
type Server struct {
	hosts  []Host
	signed []SignedSource
	host   string // the Host header every request must carry
	tmpl   *template.Template
	now    func() time.Time

	mu       sync.Mutex
	links    map[string]time.Time // one-time login tokens and their ends
	sessions map[string]bool
	csrfKey  []byte
	watchers map[chan struct{}]bool
}

// CheckListen refuses a listen address that is not a literal loopback IP with a port.
func CheckListen(addr string) error {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("listen address %q: %w", addr, err)
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return fmt.Errorf("listen address %q is not a loopback IP such as 127.0.0.1 or [::1]", addr)
	}
	return nil
}

// New returns a dashboard for hosts, answering only requests whose Host is addr, the
// address it listens on.
func New(hosts []Host, addr string) (*Server, error) {
	if err := CheckListen(addr); err != nil {
		return nil, err
	}
	srv := &Server{}
	tmpl, err := template.New("").Funcs(template.FuncMap{
		"intent": func(source int) string {
			if source < 0 || source >= len(srv.signed) {
				return ""
			}
			return srv.signed[source].NewIntent()
		},
		"time":     func(ms int64) string { return time.UnixMilli(ms).Format("2006-01-02 15:04") },
		"digest":   func(it koinon.Item) string { d, _ := koinon.SnapshotDigest(it); return d },
		"verdicts": func(kind string) []string { return koinon.KindVerdicts[kind] },
	}).ParseFS(assets, "assets/*.html")
	if err != nil {
		return nil, err
	}
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	*srv = Server{
		hosts: hosts, host: addr, tmpl: tmpl, now: time.Now,
		links: map[string]time.Time{}, sessions: map[string]bool{}, csrfKey: key,
		watchers: map[chan struct{}]bool{},
	}
	return srv, nil
}

// SetSigned sets the signed rows; call it before Handler.
func (s *Server) SetSigned(sources []SignedSource) { s.signed = sources }

// LoginLink returns a new login link, which works once within LinkLifetime.
func (s *Server) LoginLink() string {
	token := randomHex()
	s.mu.Lock()
	s.links[token] = s.now().Add(LinkLifetime)
	s.mu.Unlock()
	return "http://" + s.host + "/login?token=" + token
}

// Changed tells every open page that a host's state changed. Connectors call it.
func (s *Server) Changed() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for w := range s.watchers {
		select {
		case w <- struct{}{}:
		default:
		}
	}
}

// Handler returns the dashboard's handler.
func (s *Server) Handler() http.Handler {
	static, _ := fs.Sub(assets, "assets")
	mux := http.NewServeMux()
	mux.HandleFunc("GET /login", s.login)
	mux.HandleFunc("GET /{$}", s.authed(s.page))
	mux.HandleFunc("GET /board", s.authed(s.board))
	mux.HandleFunc("GET /events", s.authed(s.events))
	mux.HandleFunc("POST /verdict", s.authed(s.verdict))
	mux.HandleFunc("POST /dismiss", s.authed(s.dismiss))
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServerFS(static)))
	return s.protect(mux)
}

func randomHex() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

// protect sets the security headers on every response and refuses a request whose Host
// is not the listen address, or whose Origin, when present, is not that host's.
func (s *Server) protect(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Security-Policy", "default-src 'none'; script-src 'self'; style-src 'self'; connect-src 'self'; form-action 'self'; base-uri 'none'; frame-ancestors 'none'")
		h.Set("X-Frame-Options", "DENY")
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "same-origin")
		h.Set("Cache-Control", "no-store")
		h.Set("Cross-Origin-Opener-Policy", "same-origin")
		if r.Host != s.host {
			http.Error(w, "wrong host", http.StatusForbidden)
			return
		}
		if o := r.Header.Get("Origin"); o != "" && o != "http://"+s.host {
			http.Error(w, "wrong origin", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// login takes a one-time link and starts a session.
func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	s.mu.Lock()
	end, ok := s.links[token]
	delete(s.links, token)
	var id string
	if ok && s.now().Before(end) {
		id = randomHex()
		s.sessions[id] = true
	}
	s.mu.Unlock()
	if id == "" {
		s.render(w, http.StatusUnauthorized, "login.html", nil)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: CookieName, Value: id, Path: "/", HttpOnly: true, SameSite: http.SameSiteStrictMode})
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) session(r *http.Request) string {
	c, err := r.Cookie(CookieName)
	if err != nil {
		return ""
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.sessions[c.Value] {
		return ""
	}
	return c.Value
}

// authed lets a request with a session through; a POST also needs the Origin of the
// listen address and the session's CSRF token.
func (s *Server) authed(next func(http.ResponseWriter, *http.Request, string)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := s.session(r)
		if id == "" {
			s.render(w, http.StatusUnauthorized, "login.html", nil)
			return
		}
		if r.Method == http.MethodPost {
			if r.Header.Get("Origin") != "http://"+s.host {
				http.Error(w, "wrong origin", http.StatusForbidden)
				return
			}
			r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
			if subtle.ConstantTimeCompare([]byte(r.PostFormValue("csrf")), []byte(s.csrf(id))) != 1 {
				http.Error(w, "wrong form token", http.StatusForbidden)
				return
			}
		}
		next(w, r, id)
	}
}

func (s *Server) csrf(session string) string {
	m := hmac.New(sha256.New, s.csrfKey)
	m.Write([]byte(session))
	return hex.EncodeToString(m.Sum(nil))
}

func (s *Server) render(w http.ResponseWriter, status int, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if err := s.tmpl.ExecuteTemplate(w, name, data); err != nil {
		fmt.Fprintln(w, "the page failed")
	}
}

// view is what the board template shows.
type view struct {
	CSRF   string
	Notice *notice
	Hosts  []koinon.State
	Signed []signedView
}

// signedView is one signed row's states; Source is its index, which forms carry.
type signedView struct {
	Source int
	States []koinon.State
}

// notice is the result of the last verdict, from fixed texts, with the refusal's details.
type notice struct {
	Host, Reference, Code, Detail string
	OK                            bool
}

func (s *Server) view(r *http.Request, session string) view {
	v := view{CSRF: s.csrf(session)}
	for _, h := range s.hosts {
		v.Hosts = append(v.Hosts, h.State())
	}
	for i, src := range s.signed {
		v.Signed = append(v.Signed, signedView{Source: i, States: src.States()})
	}
	q := r.URL.Query()
	if code := q.Get("notice"); code != "" {
		v.Notice = &notice{Host: q.Get("host"), Reference: q.Get("ref"), Code: code, Detail: q.Get("detail"), OK: code == "recorded"}
	}
	return v
}

func (s *Server) page(w http.ResponseWriter, r *http.Request, session string) {
	s.render(w, http.StatusOK, "page.html", s.view(r, session))
}

func (s *Server) board(w http.ResponseWriter, r *http.Request, session string) {
	s.render(w, http.StatusOK, "board.html", s.view(r, session))
}

// events tells the page when a host's state changes. It carries no item text.
func (s *Server) events(w http.ResponseWriter, r *http.Request, _ string) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "no streaming", http.StatusInternalServerError)
		return
	}
	ch := make(chan struct{}, 1)
	s.mu.Lock()
	s.watchers[ch] = true
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		delete(s.watchers, ch)
		s.mu.Unlock()
	}()
	w.Header().Set("Content-Type", "text/event-stream")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, ": open\n\n")
	flusher.Flush()
	keepalive := time.NewTicker(25 * time.Second)
	defer keepalive.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-ch:
			fmt.Fprint(w, "event: change\ndata: {}\n\n")
		case <-keepalive.C:
			fmt.Fprint(w, ": keepalive\n\n")
		}
		flusher.Flush()
	}
}

// verdict gives a verdict through the item's host at the revision the page showed, and
// returns to the page with the result.
func (s *Server) verdict(w http.ResponseWriter, r *http.Request, _ string) {
	if r.PostFormValue("source") != "" {
		s.signedVerdict(w, r)
		return
	}
	label, ref, verdict := r.PostFormValue("host"), r.PostFormValue("reference"), r.PostFormValue("verdict")
	revision, err := strconv.ParseInt(r.PostFormValue("revision"), 10, 64)
	var host Host
	for _, h := range s.hosts {
		if h.State().Host == label {
			host = h
		}
	}
	code := "recorded"
	switch {
	case host == nil || ref == "" || err != nil:
		code = "invalid_request"
	case verdict != "approved" && verdict != "declined" && verdict != "dismissed":
		code = "invalid_verdict"
	default:
		_, err := host.Verdict(r.Context(), ref, revision, verdict, r.PostFormValue("note"))
		switch {
		case errors.Is(err, koinon.ErrDisconnected):
			code = "disconnected"
		case err != nil:
			code = koinon.Code(err)
			if code == "" {
				code = "failed"
			}
		}
	}
	q := url.Values{"notice": {code}, "host": {label}, "ref": {ref}}
	http.Redirect(w, r, "/?"+q.Encode(), http.StatusSeeOther)
}

// signedSource returns the signed row that the form names and the state of the computer
// with the form's pin, which that row must show now.
func (s *Server) signedSource(r *http.Request) (SignedSource, koinon.State, bool) {
	i, err := strconv.Atoi(r.PostFormValue("source"))
	if err != nil || i < 0 || i >= len(s.signed) {
		return nil, koinon.State{}, false
	}
	pin := r.PostFormValue("pin")
	for _, st := range s.signed[i].States() {
		if pin != "" && st.Pin == pin {
			return s.signed[i], st, true
		}
	}
	return nil, koinon.State{}, false
}

// signedVerdict gives a verdict on a signed row's item, as the page showed it: the form
// carries the item's revision and snapshot digest, and the row signs only that item.
func (s *Server) signedVerdict(w http.ResponseWriter, r *http.Request) {
	ref := r.PostFormValue("reference")
	revision, err := strconv.ParseInt(r.PostFormValue("revision"), 10, 64)
	src, st, ok := s.signedSource(r)
	code, detail := "invalid_request", ""
	if ok && ref != "" && err == nil {
		res := src.Verdict(r.Context(), koinon.VerdictRequest{Pin: st.Pin, Reference: ref, Revision: revision,
			Digest: r.PostFormValue("digest"), Verdict: r.PostFormValue("verdict"), Note: r.PostFormValue("note"),
			Intent: r.PostFormValue("intent")})
		code, detail = resultCode(res), koinon.DetailText(res.Details)
	}
	q := url.Values{"notice": {code}, "host": {st.Host}, "ref": {ref}}
	if detail != "" {
		q.Set("detail", detail)
	}
	http.Redirect(w, r, "/?"+q.Encode(), http.StatusSeeOther)
}

// resultCode is the notice for a signed verdict's result.
func resultCode(res koinon.VerdictResult) string {
	switch {
	case res.Repeat:
		return "repeat"
	case res.State == koinon.Recorded:
		return "recorded"
	case res.State == koinon.NotRun:
		return "not_run"
	case res.Pending:
		return "pending"
	case res.State == koinon.Unknown && res.Final:
		return "final_unknown"
	case res.State == koinon.Unknown:
		return "unknown"
	case res.Code != "":
		return res.Code
	}
	return "failed"
}

// dismiss removes a signed row's local record of a verdict whose outcome can no longer be
// learned. It sends nothing.
func (s *Server) dismiss(w http.ResponseWriter, r *http.Request, _ string) {
	src, st, ok := s.signedSource(r)
	code := "invalid_request"
	if ok {
		code = "dismissed"
		if err := src.Dismiss(r.PostFormValue("request")); err != nil {
			code = "dismiss_failed"
		}
	}
	q := url.Values{"notice": {code}, "host": {st.Host}}
	http.Redirect(w, r, "/?"+q.Encode(), http.StatusSeeOther)
}
