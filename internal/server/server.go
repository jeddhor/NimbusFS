package server

import (
	"io/fs"
	"log"
	"net/http"
	"strings"

	"nimbusfs/internal/api"
	"nimbusfs/internal/auth"
	"nimbusfs/internal/config"
	"nimbusfs/internal/fsops"
	"nimbusfs/internal/store"
)

// New builds the full HTTP handler: API routes plus the embedded SPA.
func New(cfg *config.Config, sandbox *fsops.Sandbox, sessions *auth.SessionManager, st *store.Store, frontend fs.FS) http.Handler {
	a := api.New(cfg, sandbox, sessions, st)
	mux := http.NewServeMux()

	// Login is exempt from CSRF since no session/csrf cookie exists yet;
	// it's protected instead by per-IP rate limiting in the handler.
	mux.HandleFunc("POST /api/login", a.Login)
	mux.HandleFunc("POST /api/logout", a.RequireAuth(a.RequireCSRF(a.Logout)))
	mux.HandleFunc("GET /api/me", a.RequireAuth(a.Me))

	mux.HandleFunc("GET /api/files", a.RequireAuth(a.ListFiles))
	mux.HandleFunc("GET /api/file", a.RequireAuth(a.GetFile))
	mux.HandleFunc("POST /api/upload", a.RequireAuth(a.RequireCSRF(a.Upload)))
	mux.HandleFunc("DELETE /api/file", a.RequireAuth(a.RequireCSRF(a.DeleteFile)))
	mux.HandleFunc("POST /api/mkdir", a.RequireAuth(a.RequireCSRF(a.Mkdir)))
	mux.HandleFunc("POST /api/create-file", a.RequireAuth(a.RequireCSRF(a.CreateFile)))
	mux.HandleFunc("POST /api/rename", a.RequireAuth(a.RequireCSRF(a.Rename)))
	mux.HandleFunc("POST /api/move", a.RequireAuth(a.RequireCSRF(a.Move)))
	mux.HandleFunc("POST /api/copy", a.RequireAuth(a.RequireCSRF(a.Copy)))

	mux.Handle("/", spaHandler(frontend))

	return withSecurityHeaders(withProxyAwareness(cfg, mux))
}

// spaHandler serves the embedded SPA build, falling back to index.html for
// any path that isn't a real asset (client-side routing).
func spaHandler(frontend fs.FS) http.Handler {
	fileServer := http.FileServer(http.FS(frontend))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}
		if _, err := fs.Stat(frontend, path); err != nil {
			r2 := new(http.Request)
			*r2 = *r
			r2.URL.Path = "/index.html"
			fileServer.ServeHTTP(w, r2)
			return
		}
		fileServer.ServeHTTP(w, r)
	})
}

func withSecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "same-origin")
		h.Set("Content-Security-Policy", "default-src 'self'; img-src 'self' data: blob:; media-src 'self' blob:; style-src 'self' 'unsafe-inline'")
		next.ServeHTTP(w, r)
	})
}

// withProxyAwareness logs a one-time warning if behind_proxy is set but the
// request doesn't carry forwarding headers, which usually means misconfiguration.
func withProxyAwareness(cfg *config.Config, next http.Handler) http.Handler {
	warned := false
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if cfg.Server.BehindProxy && !warned && r.Header.Get("X-Forwarded-For") == "" {
			log.Printf("warning: behind_proxy is enabled but request from %s had no X-Forwarded-For header", r.RemoteAddr)
			warned = true
		}
		next.ServeHTTP(w, r)
	})
}
