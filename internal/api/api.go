package api

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"log"
	"mime"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"nimbusfs/internal/auth"
	"nimbusfs/internal/config"
	"nimbusfs/internal/fsops"
	"nimbusfs/internal/mimetypes"
	"nimbusfs/internal/search"
	"nimbusfs/internal/store"
	"nimbusfs/internal/thumbnail"
)

type API struct {
	cfg               *config.Config
	sandbox           *fsops.Sandbox
	sessions          *auth.SessionManager
	store             *store.Store
	sshDevices        *auth.SSHDeviceStore
	shareUnlockSecret []byte
	searchIndex       *search.Index
	thumbnails        *thumbnail.Generator
	fileTypeNames     map[string]string
}

func New(cfg *config.Config, sandbox *fsops.Sandbox, sessions *auth.SessionManager, st *store.Store, searchIndex *search.Index, thumbnails *thumbnail.Generator) *API {
	secret := make([]byte, 32)
	_, _ = rand.Read(secret)
	return &API{
		cfg:               cfg,
		sandbox:           sandbox,
		sessions:          sessions,
		store:             st,
		sshDevices:        auth.NewSSHDeviceStore(),
		shareUnlockSecret: secret,
		searchIndex:       searchIndex,
		thumbnails:        thumbnails,
		fileTypeNames:     mimetypes.Load(),
	}
}

// Features reports which optional features are enabled, so the authenticated
// UI can hide controls (e.g. sharing) the operator has turned off.
func (a *API) Features(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]bool{
		"sharing": a.cfg.Sharing.Enabled,
		"search":  a.cfg.Search.Enabled,
	})
}

// FileTypes reports the extension -> human-readable type name map (e.g.
// "py" -> "Python script") derived from the system's shared-mime-info
// database, so the UI can show a friendly type instead of "PY File".
func (a *API) FileTypes(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, a.fileTypeNames)
}

// AuthMethods reports which login methods are enabled, so the login page
// can decide whether to show the SSH-key option without needing auth itself.
func (a *API) AuthMethods(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]bool{
		"pam":       a.cfg.Auth.PAM,
		"sshKeys":   a.cfg.Auth.SSHKeys,
		"proxyAuth": a.cfg.Auth.ProxyAuth,
	})
}

type ctxKey int

const ctxUsername ctxKey = iota

func withUsername(ctx context.Context, username string) context.Context {
	return context.WithValue(ctx, ctxUsername, username)
}

func usernameFrom(ctx context.Context) string {
	username, _ := ctx.Value(ctxUsername).(string)
	return username
}

func clientIP(r *http.Request) string {
	if ip := r.Header.Get("X-Forwarded-For"); ip != "" {
		parts := strings.Split(ip, ",")
		return strings.TrimSpace(parts[0])
	}
	host, _, err := splitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func splitHostPort(addr string) (string, string, error) {
	idx := strings.LastIndex(addr, ":")
	if idx < 0 {
		return addr, "", nil
	}
	return addr[:idx], addr[idx+1:], nil
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// ---- Auth endpoints ----

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Remember bool   `json:"remember"`
}

const maxLoginAttemptsPerMinute = 5

func (a *API) Login(w http.ResponseWriter, r *http.Request) {
	if !a.cfg.Auth.PAM {
		writeError(w, http.StatusNotFound, "password login is disabled")
		return
	}
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	ip := clientIP(r)

	count, err := a.store.RecentFailedAttempts(ip, time.Now().Add(-time.Minute))
	if err == nil && count >= maxLoginAttemptsPerMinute {
		writeError(w, http.StatusTooManyRequests, "too many login attempts, try again later")
		return
	}

	if req.Username == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "username and password are required")
		return
	}

	err = auth.Authenticate(auth.DefaultService, req.Username, req.Password)
	_ = a.store.RecordLoginAttempt(req.Username, ip, err == nil)
	if err != nil {
		_ = a.store.RecordAudit(req.Username, "login_failed", "", ip, "")
		writeError(w, http.StatusUnauthorized, "invalid username or password")
		return
	}

	if _, err := fsops.LookupIdentity(req.Username); err != nil {
		writeError(w, http.StatusUnauthorized, "unknown system user")
		return
	}

	secure := r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
	cookie, err := a.sessions.Create(req.Username, req.Remember, secure)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not create session")
		return
	}
	http.SetCookie(w, cookie)
	setCSRFCookie(w, secure)
	_ = a.store.RecordAudit(req.Username, "login", "", ip, "")
	writeJSON(w, http.StatusOK, map[string]string{"username": req.Username})
}

func (a *API) Logout(w http.ResponseWriter, r *http.Request) {
	username := usernameFrom(r.Context())
	if c, err := r.Cookie(auth.CookieName); err == nil {
		_ = a.sessions.Destroy(c.Value)
	}
	http.SetCookie(w, &http.Cookie{Name: auth.CookieName, Value: "", Path: "/", MaxAge: -1})
	_ = a.store.RecordAudit(username, "logout", "", clientIP(r), "")
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (a *API) Me(w http.ResponseWriter, r *http.Request) {
	username := usernameFrom(r.Context())
	writeJSON(w, http.StatusOK, map[string]string{"username": username})
}

// ---- CSRF (double-submit cookie) ----

const csrfCookieName = "nimbusfs_csrf"
const csrfHeaderName = "X-CSRF-Token"

func setCSRFCookie(w http.ResponseWriter, secure bool) {
	b := make([]byte, 24)
	_, _ = rand.Read(b)
	token := base64.RawURLEncoding.EncodeToString(b)
	http.SetCookie(w, &http.Cookie{
		Name:     csrfCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: false,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
}

// ensureCSRFCookie sets a CSRF cookie if the request doesn't already carry
// one. Password/SSH login set it explicitly on success; proxy-authenticated
// requests never go through a login endpoint, so it's bootstrapped here on
// the first authenticated request instead.
func ensureCSRFCookie(w http.ResponseWriter, r *http.Request, secure bool) {
	if c, err := r.Cookie(csrfCookieName); err == nil && c.Value != "" {
		return
	}
	setCSRFCookie(w, secure)
}

func (a *API) RequireCSRF(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
			cookie, err := r.Cookie(csrfCookieName)
			if err != nil || cookie.Value == "" {
				writeError(w, http.StatusForbidden, "missing csrf token")
				return
			}
			if r.Header.Get(csrfHeaderName) != cookie.Value {
				writeError(w, http.StatusForbidden, "invalid csrf token")
				return
			}
		}
		next(w, r)
	}
}

// ---- Auth middleware ----

// proxyAuthHeader is the header a trusted reverse proxy (Authelia, Keycloak,
// oauth2-proxy, etc.) sets after it has already authenticated the request.
// nimbusfs only trusts it when auth.proxy_auth is enabled; the operator is
// responsible for making sure their proxy strips any client-supplied copy
// of this header before setting their own — nimbusfs has no way to verify
// a request actually passed through the proxy rather than around it.
const proxyAuthHeader = "X-Remote-User"

func (a *API) RequireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if a.cfg.Auth.ProxyAuth {
			if username := r.Header.Get(proxyAuthHeader); username != "" {
				if _, err := fsops.LookupIdentity(username); err != nil {
					writeError(w, http.StatusUnauthorized, "unknown system user")
					return
				}
				secure := r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
				ensureCSRFCookie(w, r, secure)
				next(w, r.WithContext(withUsername(r.Context(), username)))
				return
			}
		}

		c, err := r.Cookie(auth.CookieName)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "not authenticated")
			return
		}
		username, err := a.sessions.Validate(c.Value)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "session expired")
			return
		}
		// The CSRF cookie is intentionally session-only (cleared when the
		// browser closes), but a "remember me" session cookie can outlive it
		// by weeks. Re-issue it here if missing so a long-lived session
		// doesn't end up permanently unable to pass CSRF checks.
		secure := r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
		ensureCSRFCookie(w, r, secure)
		ctx := r.Context()
		ctx = withUsername(ctx, username)
		next(w, r.WithContext(ctx))
	}
}

// ---- Filesystem endpoints ----
// Every handler below runs the actual filesystem work via fsops.As so that
// kernel permission checks are evaluated as the authenticated Linux user.

func (a *API) ListFiles(w http.ResponseWriter, r *http.Request) {
	username := usernameFrom(r.Context())
	reqPath := r.URL.Query().Get("path")
	if reqPath == "" {
		reqPath = "/"
	}

	id, err := fsops.LookupIdentity(username)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "user lookup failed")
		return
	}

	var entries []fsops.Entry
	err = fsops.As(id, func() error {
		var e error
		entries, e = a.sandbox.List(reqPath)
		return e
	})
	if err != nil {
		writeFSError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"path": reqPath, "entries": entries})
}

func (a *API) GetFile(w http.ResponseWriter, r *http.Request) {
	username := usernameFrom(r.Context())
	reqPath := r.URL.Query().Get("path")
	if reqPath == "" {
		writeError(w, http.StatusBadRequest, "path is required")
		return
	}
	download := r.URL.Query().Get("download") == "1"

	id, err := fsops.LookupIdentity(username)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "user lookup failed")
		return
	}

	var f *os.File
	var info os.FileInfo
	err = fsops.As(id, func() error {
		var e error
		f, info, e = a.sandbox.Open(reqPath)
		return e
	})
	if err != nil {
		writeFSError(w, err)
		return
	}
	defer f.Close()

	if info.IsDir() {
		writeError(w, http.StatusBadRequest, "cannot download a directory directly")
		return
	}

	w.Header().Set("Content-Type", mimeTypeFor(reqPath))
	if download {
		w.Header().Set("Content-Disposition", "attachment; filename=\""+path.Base(reqPath)+"\"")
	}
	_ = a.store.RecordAudit(username, "download", reqPath, clientIP(r), "")
	http.ServeContent(w, r, info.Name(), info.ModTime(), f)
}

func (a *API) Upload(w http.ResponseWriter, r *http.Request) {
	username := usernameFrom(r.Context())
	destDir := r.URL.Query().Get("path")
	if destDir == "" {
		destDir = "/"
	}

	id, err := fsops.LookupIdentity(username)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "user lookup failed")
		return
	}

	if err := r.ParseMultipartForm(1 << 30); err != nil {
		writeError(w, http.StatusBadRequest, "invalid upload")
		return
	}
	files := r.MultipartForm.File["files"]
	if len(files) == 0 {
		writeError(w, http.StatusBadRequest, "no files provided")
		return
	}

	for _, fh := range files {
		// relPath supports folder uploads where the client sends a webkitdirectory path.
		relPath := fh.Filename
		destPath := path.Join(destDir, relPath)

		src, err := fh.Open()
		if err != nil {
			writeError(w, http.StatusBadRequest, "could not read upload")
			return
		}

		err = fsops.As(id, func() error {
			out, e := a.sandbox.Create(destPath)
			if e != nil {
				return e
			}
			defer out.Close()
			_, e = io.Copy(out, src)
			return e
		})
		src.Close()
		if err != nil {
			writeFSError(w, err)
			return
		}
		_ = a.store.RecordAudit(username, "upload", destPath, clientIP(r), "")
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (a *API) DeleteFile(w http.ResponseWriter, r *http.Request) {
	username := usernameFrom(r.Context())
	reqPath := r.URL.Query().Get("path")
	if reqPath == "" {
		writeError(w, http.StatusBadRequest, "path is required")
		return
	}
	id, err := fsops.LookupIdentity(username)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "user lookup failed")
		return
	}
	err = fsops.As(id, func() error { return a.sandbox.Delete(reqPath) })
	if err != nil {
		writeFSError(w, err)
		return
	}
	_ = a.store.RecordAudit(username, "delete", reqPath, clientIP(r), "")
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

type mkdirRequest struct {
	Path string `json:"path"`
}

func (a *API) Mkdir(w http.ResponseWriter, r *http.Request) {
	username := usernameFrom(r.Context())
	var req mkdirRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Path == "" {
		writeError(w, http.StatusBadRequest, "path is required")
		return
	}
	id, err := fsops.LookupIdentity(username)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "user lookup failed")
		return
	}
	err = fsops.As(id, func() error { return a.sandbox.Mkdir(req.Path) })
	if err != nil {
		writeFSError(w, err)
		return
	}
	_ = a.store.RecordAudit(username, "mkdir", req.Path, clientIP(r), "")
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (a *API) CreateFile(w http.ResponseWriter, r *http.Request) {
	username := usernameFrom(r.Context())
	var req mkdirRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Path == "" {
		writeError(w, http.StatusBadRequest, "path is required")
		return
	}
	id, err := fsops.LookupIdentity(username)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "user lookup failed")
		return
	}
	err = fsops.As(id, func() error { return a.sandbox.CreateFile(req.Path) })
	if err != nil {
		writeFSError(w, err)
		return
	}
	_ = a.store.RecordAudit(username, "create_file", req.Path, clientIP(r), "")
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

type renameRequest struct {
	Path    string `json:"path"`
	NewName string `json:"newName"`
}

func (a *API) Rename(w http.ResponseWriter, r *http.Request) {
	username := usernameFrom(r.Context())
	var req renameRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Path == "" || req.NewName == "" {
		writeError(w, http.StatusBadRequest, "path and newName are required")
		return
	}
	if strings.ContainsAny(req.NewName, "/\\") {
		writeError(w, http.StatusBadRequest, "newName must not contain path separators")
		return
	}
	id, err := fsops.LookupIdentity(username)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "user lookup failed")
		return
	}
	var newPath string
	err = fsops.As(id, func() error {
		var e error
		newPath, e = a.sandbox.Rename(req.Path, req.NewName)
		return e
	})
	if err != nil {
		writeFSError(w, err)
		return
	}
	_ = a.store.RecordAudit(username, "rename", req.Path, clientIP(r), newPath)
	writeJSON(w, http.StatusOK, map[string]string{"path": newPath})
}

type srcDestRequest struct {
	Src  string `json:"src"`
	Dest string `json:"dest"`
}

func (a *API) Move(w http.ResponseWriter, r *http.Request) {
	a.handleSrcDest(w, r, "move", func(s *fsops.Sandbox, req srcDestRequest) error {
		return s.Move(req.Src, req.Dest)
	})
}

func (a *API) Copy(w http.ResponseWriter, r *http.Request) {
	a.handleSrcDest(w, r, "copy", func(s *fsops.Sandbox, req srcDestRequest) error {
		return s.Copy(req.Src, req.Dest)
	})
}

func (a *API) handleSrcDest(w http.ResponseWriter, r *http.Request, action string, fn func(*fsops.Sandbox, srcDestRequest) error) {
	username := usernameFrom(r.Context())
	var req srcDestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Src == "" || req.Dest == "" {
		writeError(w, http.StatusBadRequest, "src and dest are required")
		return
	}
	id, err := fsops.LookupIdentity(username)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "user lookup failed")
		return
	}
	err = fsops.As(id, func() error { return fn(a.sandbox, req) })
	if err != nil {
		writeFSError(w, err)
		return
	}
	_ = a.store.RecordAudit(username, action, req.Src, clientIP(r), req.Dest)
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func mimeTypeFor(name string) string {
	ctype := mime.TypeByExtension(filepath.Ext(name))
	if ctype == "" {
		return "application/octet-stream"
	}
	return ctype
}

func writeFSError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, fsops.ErrEscape):
		writeError(w, http.StatusBadRequest, "invalid path")
	case errors.Is(err, os.ErrNotExist):
		writeError(w, http.StatusNotFound, "not found")
	case errors.Is(err, os.ErrPermission):
		writeError(w, http.StatusForbidden, "permission denied")
	default:
		log.Printf("fsops error: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error")
	}
}
