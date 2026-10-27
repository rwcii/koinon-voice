package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os/exec"
	"runtime"
	"sync"
	"time"

	"github.com/rwcii/koinon-voice/internal/dashboard"
	"github.com/rwcii/koinon-voice/internal/koinon"
)

// openLink opens the login link in the browser; tests replace it.
var openLink = func(link string) error {
	name := "xdg-open"
	if runtime.GOOS == "darwin" {
		name = "open"
	}
	return exec.Command(name, link).Start()
}

const dashboardUsage = `Usage: koinon-voice dashboard [flags]

Connect to each Koinon daemon in the hosts file and serve the dashboard on a loopback
address: each host's state, the items that wait for the maintainer, the maintainer's inbox,
and a verdict by click. It prints a login link, which works once within 60 seconds, and
opens it unless --no-open.

`

func runDashboard(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("dashboard", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprint(stderr, dashboardUsage)
		fs.PrintDefaults()
	}
	hostsFile := fs.String("hosts", defaultHosts(), "hosts file")
	listen := fs.String("listen", "127.0.0.1:17680", "loopback address to listen on")
	noOpen := fs.Bool("no-open", false, "print the login link without opening it")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 0 {
		fs.Usage()
		return 2
	}
	if err := dashboard.CheckListen(*listen); err != nil {
		fmt.Fprintln(stderr, "koinon-voice dashboard:", err)
		return 1
	}
	hosts, err := koinon.LoadHosts(*hostsFile)
	if err != nil {
		fmt.Fprintln(stderr, "koinon-voice dashboard:", err)
		return 1
	}
	ln, err := net.Listen("tcp", *listen)
	if err != nil {
		fmt.Fprintln(stderr, "koinon-voice dashboard:", err)
		return 1
	}

	conns := make([]*koinon.Connector, len(hosts))
	views := make([]dashboard.Host, len(hosts))
	var srv *dashboard.Server
	opts := connectorOptions
	opts.Secrets, opts.Version = hostSecrets, version
	opts.OnChange = func(koinon.State) {
		if srv != nil {
			srv.Changed()
		}
	}
	for i, h := range hosts {
		conns[i] = koinon.New(h, opts)
		views[i] = conns[i]
	}
	srv, err = dashboard.New(views, ln.Addr().String())
	if err != nil {
		ln.Close()
		fmt.Fprintln(stderr, "koinon-voice dashboard:", err)
		return 1
	}

	ctx, cancel := watchContext()
	defer cancel()
	var wg sync.WaitGroup
	for _, c := range conns {
		wg.Add(1)
		go func(c *koinon.Connector) {
			defer wg.Done()
			c.Run(ctx)
		}(c)
	}
	// Requests take the run's context, so open event streams end when the dashboard stops.
	httpSrv := &http.Server{Handler: srv.Handler(), ReadHeaderTimeout: 10 * time.Second,
		BaseContext: func(net.Listener) context.Context { return ctx }}
	served := make(chan error, 1)
	go func() { served <- httpSrv.Serve(ln) }()

	link := srv.LoginLink()
	fmt.Fprintf(stdout, "Koinon Voice dashboard at http://%s/\nLogin link (works once, within 60 seconds):\n%s\n", ln.Addr(), link)
	if !*noOpen {
		if err := openLink(link); err != nil {
			fmt.Fprintln(stderr, "koinon-voice dashboard: open the link by hand:", err)
		}
	}

	code := 0
	select {
	case <-ctx.Done():
	case err := <-served:
		if !errors.Is(err, http.ErrServerClosed) {
			fmt.Fprintln(stderr, "koinon-voice dashboard:", err)
			code = 1
		}
	}
	cancel()
	shutdown, stop := context.WithTimeout(context.Background(), 2*time.Second)
	defer stop()
	_ = httpSrv.Shutdown(shutdown)
	wg.Wait()
	return code
}
