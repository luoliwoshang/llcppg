/*
 * Copyright (c) 2026 The XGo Authors (xgo.dev). All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package docserver

import (
	"embed"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

//go:embed assets/page.html assets/page.css assets/live.js assets/outline.js
var assets embed.FS

// pageTemplate is parsed once from the embedded shell template.
var pageTemplate = template.Must(template.ParseFS(assets, "assets/page.html"))

// byeURL is the endpoint a monitored page beacons when it is closing (see
// server.handleBye and the Monitor script in assets/page.html). It lives under
// the "/_doc/" prefix, matching "/_events", so it cannot collide with a package
// symbol route under "/".
const byeURL = "/_doc/bye"

// exitGrace is how long handleBye waits before shutting the server down. A
// reload sends bye from the old page and then requests "/" from the new page
// within milliseconds; the window gives the new GET / time to arrive and cancel
// the timer, so a reload does not look like a close. It only delays the exit,
// so a longer value is cheap over slow links (see the proposal's open question).
// It is a var, not a const, only so tests can shorten it.
var exitGrace = 2 * time.Second

// server holds the handler state for one documentation server. Most of it is
// the immutable render configuration (dir, allDecls, broker); the rest is the
// auto-exit machinery, which is only engaged when autoExit is true.
type server struct {
	dir      string
	allDecls bool
	b        *broker
	done     <-chan struct{}

	// autoExit turns on monitoring of the auto-opened page: the first GET /
	// is rendered with the Monitor marker, /_doc/bye is registered, and a page
	// close shuts the server down. When false the page carries no monitor
	// script and /_doc/bye is not registered, exactly as before this existed.
	autoExit bool

	// host is the address the server listens on ("host:port"), used by
	// handleBye to reject requests whose Host header does not match (DNS
	// rebinding, or a forwarded host where auto-exit should simply not fire).
	host string

	// cancel stops the server, running the same graceful shutdown path as
	// SIGINT. handleBye's timer calls it when the monitored page is closed.
	cancel func()

	// out receives the one-line shutdown notice when the monitored page is
	// closed. It is the same writer as the startup message (Options.Stdout).
	out io.Writer

	// claimed records whether the monitored page has been handed out. The first
	// GET / wins the compare-and-swap and is the only response rendered with
	// Monitor=true; pages the user opens later by hand are never monitored.
	claimed atomic.Bool

	// mu guards timer. timer is the pending exit timer started by handleBye and
	// cancelled by a subsequent GET /; nil means no exit is pending.
	mu    sync.Mutex
	timer *time.Timer
}

// newHandler builds the HTTP handler. "GET /" loads and renders the package in
// dir on every request so edits are picked up without a restart; "GET
// /static/..." serves llcppg's own assets (the page.css stylesheet and, when
// live reload is on, live.js); "GET /src/..." serves the package source so
// documentation links can point at it. Nothing else on the file system is
// served, and package code is never executed. dir is the absolute package
// directory resolved by load(), so the source handler and the symbol links
// generated during rendering agree on the same path.
//
// When b is non-nil the server is in live-reload mode: it registers the
// "GET /_events" Server-Sent Events route and the rendered page pulls in the
// live-reload client script. When b is nil the server is degraded (the file
// watcher could not start); /_events is not registered, so it returns 404, and
// the page carries no live-reload script.
//
// When autoExit is true the server monitors the first page it serves: that
// response carries a small script that beacons POST /_doc/bye on pagehide, and
// a close shuts the server down via cancel (host is the listen address used to
// validate the beacon, out receives the shutdown notice). When autoExit is
// false none of this is wired, so cancel, host and out may be zero and the page
// is identical to a server without it.
//
// done is the server's lifetime signal passed to the event handler so open
// streams close on shutdown (see broker.serveEvents); it is ignored when b is
// nil.
func newHandler(dir string, allDecls bool, b *broker, done <-chan struct{}, autoExit bool, host string, cancel func(), out io.Writer) http.Handler {
	s := &server{
		dir:      dir,
		allDecls: allDecls,
		b:        b,
		done:     done,
		autoExit: autoExit,
		host:     host,
		cancel:   cancel,
		out:      out,
	}
	return s.mux()
}

// mux assembles the route table for s.
func (s *server) mux() http.Handler {
	mux := http.NewServeMux()

	// Serve llcppg's assets (page.css, outline.js, live.js). The embed root has
	// an "assets/" prefix; strip the URL "/static/" prefix and re-root onto it.
	static, _ := fs.Sub(assets, "assets")
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(static))))

	// Serve the package source so symbol and "View Source" links from the
	// documentation can jump to the exact line of a declaration. Only .go files
	// in dir are served (see serveSource); package code is never executed.
	mux.HandleFunc(srcURLPrefix, serveSource(s.dir))

	if s.b != nil {
		mux.HandleFunc("/_events", s.b.serveEvents(keepAlivePeriod, s.done))
	}

	// Register the close beacon only in auto-exit mode. Without it the endpoint
	// 404s and the page carries no monitor script.
	if s.autoExit {
		mux.HandleFunc(byeURL, s.handleBye)
	}

	mux.HandleFunc("/", s.handleIndex)

	return mux
}

// handleIndex serves "GET /": it loads and renders the package on every request
// so edits appear without a restart. In auto-exit mode the first request also
// claims the monitored page (rendered with Monitor=true) and every request
// cancels a pending exit timer, so a reload — which sends bye then re-requests
// "/" — does not look like a close.
func (s *server) handleIndex(w http.ResponseWriter, req *http.Request) {
	if req.URL.Path != "/" {
		http.NotFound(w, req)
		return
	}

	// A GET / is a live page, so cancel any exit countdown a just-departed page
	// started. This is the reload case: the old page's bye starts the timer and
	// the new page's request stops it, all within a few milliseconds.
	s.cancelTimer()

	// The first GET / claims the monitored page. Only that response carries the
	// marker, so later tabs the user opens by hand are not monitored.
	monitor := s.autoExit && s.claimed.CompareAndSwap(false, true)

	p, err := load(s.dir, s.allDecls)
	if err != nil {
		// A load error here means the directory stopped being a single
		// importable package since start-up (for example all files were
		// removed mid-edit). Report it rather than rendering a blank page.
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	data := p.render(s.b != nil)
	data.AutoExit = s.autoExit
	data.Monitor = monitor
	if err := pageTemplate.Execute(w, data); err != nil {
		// The header may already be written; there is nothing more we can
		// do except surface the error for diagnosis.
		fmt.Fprintf(w, "\n<!-- template error: %s -->\n", template.HTMLEscapeString(err.Error()))
	}
}

// handleBye serves "POST /_doc/bye": the beacon a monitored page sends when it
// is closing. It starts a short exit timer (unless one is already pending),
// which a subsequent GET / cancels. When the timer fires it shuts the server
// down through the same graceful path as SIGINT.
//
// Without a token in the URL the endpoint is protected by request checks: it
// accepts POST only; if an Origin header is present it must match the Host
// (browsers always send Origin on cross-site POSTs, so another site cannot
// trigger it); and the Host must equal the address the server listens on, which
// blocks DNS rebinding. The worst outcome of a bypass is a local documentation
// server exiting early, so this is sufficient.
func (s *server) handleBye(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || !originOK(r) || r.Host != s.host {
		http.NotFound(w, r)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.timer == nil {
		s.timer = time.AfterFunc(exitGrace, s.exit)
	}
}

// cancelTimer stops and clears any pending exit timer. It is a no-op when none
// is pending, so every GET / can call it unconditionally.
func (s *server) cancelTimer() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.timer != nil {
		s.timer.Stop()
		s.timer = nil
	}
}

// exit is run by the expired timer: it announces the shutdown and cancels the
// server context, which drives the same graceful shutdown as SIGINT.
func (s *server) exit() {
	if s.out != nil {
		fmt.Fprintln(s.out, "llcppg: documentation page closed, shutting down")
	}
	if s.cancel != nil {
		s.cancel()
	}
}

// originOK reports whether r's Origin header is acceptable for the bye beacon.
// A missing Origin is accepted (same-origin navigations and some clients omit
// it); when present it must name the same host:port as the request, which is
// what a browser sends for a same-origin POST and what it will not forge for a
// cross-site one.
func originOK(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	// Compare the Origin's authority against the request Host. Origin is
	// "scheme://host[:port]"; strip the scheme and match the remainder.
	if i := strings.Index(origin, "://"); i >= 0 {
		origin = origin[i+3:]
	}
	return origin == r.Host
}
