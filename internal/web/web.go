// Package web builds the HTTP surface: the mux and its middleware.
package web

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/altalune-id/othra/apperror"
	"github.com/altalune-id/othra/internal/web/i18n"
	"github.com/altalune-id/othra/internal/web/layout"
	"github.com/altalune-id/othra/internal/web/middleware"
)

// Registrar mounts a domain's server-rendered routes; each domain's web.go implements it.
type Registrar interface {
	Register(mux *http.ServeMux)
}

// ServerOpts configures the web server handler.
type ServerOpts struct {
	APIHandler http.Handler // optional Connect-RPC surface mounted at BasePath+"/api"
	MCPHandler http.Handler // optional MCP surface mounted at BasePath+"/mcp"

	ResourceMetadata map[string]http.Handler // optional RFC 9728 documents by path, mounted like Probes

	Console      http.Handler // optional console SPA mounted at BasePath+"/console"
	PortalAssets http.Handler // optional end-user portal build files mounted at BasePath+"/portal/"
	Routes       []Registrar  // optional server-rendered routes; their mux also answers every unknown path
	PublicRoutes []Registrar  // optional cookie-less public documents: no session, CSRF check or no-store
	NotFound     http.Handler // optional page for paths no route claims; a wrong method on a real route keeps its 405
	HostedPages  []Registrar  // optional end-user pages under BasePath+"/p/": never the admin Session or SessionGate
	Probes       Registrar    // optional /healthz and /readyz, mounted with no gate, session or CSRF check

	Gate        func(http.Handler) http.Handler // optional; wraps every route, probes included
	Session     func(http.Handler) http.Handler // optional; puts the cookie's Principal on routes and the console
	SessionGate func(http.Handler) http.Handler // optional; runs after Session on routes and the console
	ConsoleGate func(http.Handler) http.Handler // optional; runs after SessionGate on the console only
	HostedGate  func(http.Handler) http.Handler // optional; wraps the hosted pages only, inside their headers and CSRF check

	HostedImageBases []string // optional; base URLs the hosted pages may load images from (the storage public URL)

	CrossOrigin *http.CrossOriginProtection // guards routes and /api, not /mcp; nil uses the stdlib default
	BasePath    string
	Log         *slog.Logger
	Unexpected  apperror.UnexpectedFunc
}

// NewServer builds the root http.Handler.
func NewServer(opts ServerOpts) http.Handler {
	cop := opts.CrossOrigin
	if cop == nil {
		cop = http.NewCrossOriginProtection()
	}
	outer := http.NewServeMux()
	if opts.Probes != nil {
		opts.Probes.Register(outer)
	}
	for path, h := range opts.ResourceMetadata {
		outer.Handle("GET "+path, h)
	}
	for _, r := range opts.PublicRoutes {
		r.Register(outer)
	}
	if opts.APIHandler != nil {
		outer.Handle(layout.Path(opts.BasePath, "/api")+"/", cop.Handler(opts.APIHandler))
	}
	if opts.MCPHandler != nil {
		mcp := layout.Path(opts.BasePath, "/mcp")
		outer.Handle(mcp, opts.MCPHandler)
		outer.Handle(mcp+"/", opts.MCPHandler)
	}
	withSession := chain(opts.Session, opts.SessionGate)
	if opts.Console != nil {
		console := layout.Path(opts.BasePath, "/console")
		outer.Handle(console, http.RedirectHandler(console+"/", http.StatusMovedPermanently))
		spa := http.StripPrefix(console, opts.Console)
		if opts.ConsoleGate != nil {
			spa = opts.ConsoleGate(spa)
		}
		outer.Handle(console+"/", withSession(spa))
	}
	if opts.PortalAssets != nil {
		assets := layout.Path(opts.BasePath, "/portal") + "/"
		outer.Handle("GET "+assets, http.StripPrefix(assets, opts.PortalAssets))
	}
	if len(opts.HostedPages) > 0 {
		outer.Handle(layout.Path(opts.BasePath, "/p")+"/", hostedHandler(opts, cop))
	}
	static := layout.Path(opts.BasePath, "/static") + "/"
	outer.Handle("GET "+static, staticHandler(static, staticFiles))
	outer.Handle("GET "+layout.Path(opts.BasePath, "/favicon.ico"), http.RedirectHandler(static+"favicon.ico", http.StatusFound))
	toConsole := http.RedirectHandler(layout.Path(opts.BasePath, "/console")+"/", http.StatusFound)
	outer.Handle("GET "+strings.TrimRight(opts.BasePath, "/")+"/{$}", toConsole)
	if bp := strings.TrimRight(opts.BasePath, "/"); bp != "" {
		outer.Handle("GET "+bp, toConsole)
	}
	if len(opts.Routes) > 0 {
		routes := http.NewServeMux()
		for _, r := range opts.Routes {
			r.Register(routes)
		}
		outer.Handle("/", middleware.NoStore(cop.Handler(withSession(withNotFound(routes, opts.NotFound)))))
	} else {
		outer.Handle("/", http.NotFoundHandler())
	}
	mcpPrefix := layout.Path(opts.BasePath, "/mcp")
	secured := middleware.SecurityHeaders(outer)
	var root http.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == mcpPrefix || strings.HasPrefix(r.URL.Path, mcpPrefix+"/") {
			outer.ServeHTTP(w, r)
			return
		}
		secured.ServeHTTP(w, r)
	})
	if opts.Gate != nil {
		root = opts.Gate(root)
	}
	return middleware.Recoverer(opts.Log, opts.Unexpected, middleware.RequestID(root))
}

// SECURITY: end-user pages get their own stack with no admin session, so an admin cookie never reaches them.
func hostedHandler(opts ServerOpts, cop *http.CrossOriginProtection) http.Handler {
	mux := http.NewServeMux()
	for _, r := range opts.HostedPages {
		r.Register(mux)
	}
	var h http.Handler = mux
	if opts.HostedGate != nil {
		h = opts.HostedGate(h)
	}
	return middleware.NoStore(middleware.NoReferrer(middleware.HostedCSP(opts.HostedImageBases...)(cop.Handler(i18n.Middleware(h)))))
}

func chain(outer, inner func(http.Handler) http.Handler) func(http.Handler) http.Handler {
	return func(h http.Handler) http.Handler {
		if inner != nil {
			h = inner(h)
		}
		if outer != nil {
			h = outer(h)
		}
		return h
	}
}

// NOTE: only the mux's own 404 becomes the branded page; its 405 with Allow and every handler's answer pass through.
func withNotFound(mux *http.ServeMux, notFound http.Handler) http.Handler {
	if notFound == nil {
		return mux
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, pattern := mux.Handler(r); pattern != "" {
			mux.ServeHTTP(w, r)
			return
		}
		probe := &notFoundProbe{ResponseWriter: w}
		mux.ServeHTTP(probe, r)
		if probe.missed {
			notFound.ServeHTTP(w, r)
		}
	})
}

type notFoundProbe struct {
	http.ResponseWriter
	missed, wrote bool
}

func (p *notFoundProbe) WriteHeader(code int) {
	if !p.wrote && code == http.StatusNotFound {
		p.missed, p.wrote = true, true
		return
	}
	p.wrote = true
	p.ResponseWriter.WriteHeader(code)
}

func (p *notFoundProbe) Write(b []byte) (int, error) {
	if !p.wrote {
		p.WriteHeader(http.StatusOK)
	}
	if p.missed {
		return len(b), nil
	}
	return p.ResponseWriter.Write(b)
}
