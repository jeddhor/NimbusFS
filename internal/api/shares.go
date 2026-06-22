package api

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"nimbusfs/internal/fsops"
	"nimbusfs/internal/store"
)

// ---- Owner-facing share management (authenticated) ----

type createShareRequest struct {
	Path           string `json:"path"`
	Mode           string `json:"mode"` // "both" | "view_only" | "download_only"
	Password       string `json:"password"`
	ExpiresInHours int    `json:"expiresInHours"` // 0 = never
}

func (a *API) CreateShare(w http.ResponseWriter, r *http.Request) {
	if !a.cfg.Sharing.Enabled {
		writeError(w, http.StatusNotFound, "sharing is disabled")
		return
	}
	username := usernameFrom(r.Context())
	var req createShareRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Path == "" {
		writeError(w, http.StatusBadRequest, "path is required")
		return
	}
	switch req.Mode {
	case "":
		req.Mode = "both"
	case "both", "view_only", "download_only":
	default:
		writeError(w, http.StatusBadRequest, "invalid mode")
		return
	}

	id, err := fsops.LookupIdentity(username)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "user lookup failed")
		return
	}
	var entry fsops.Entry
	err = fsops.As(id, func() error {
		var e error
		entry, e = a.sandbox.Stat(req.Path)
		return e
	})
	if err != nil {
		writeFSError(w, err)
		return
	}

	token, err := generateShareToken()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not create share")
		return
	}

	var passwordHash string
	if req.Password != "" {
		hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not hash password")
			return
		}
		passwordHash = string(hash)
	}

	var expiresAt time.Time
	if req.ExpiresInHours > 0 {
		expiresAt = time.Now().Add(time.Duration(req.ExpiresInHours) * time.Hour)
	}

	share := store.Share{
		Token:        token,
		Username:     username,
		Path:         req.Path,
		Mode:         req.Mode,
		PasswordHash: passwordHash,
		CreatedAt:    time.Now(),
		ExpiresAt:    expiresAt,
	}
	if err := a.store.CreateShare(share); err != nil {
		writeError(w, http.StatusInternalServerError, "could not create share")
		return
	}
	_ = a.store.RecordAudit(username, "share_create", req.Path, clientIP(r), token)
	writeJSON(w, http.StatusOK, shareToJSON(share, entry.IsDir, entry.Name))
}

func (a *API) ListShares(w http.ResponseWriter, r *http.Request) {
	username := usernameFrom(r.Context())
	shares, err := a.store.ListSharesForUser(username)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not list shares")
		return
	}

	id, err := fsops.LookupIdentity(username)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "user lookup failed")
		return
	}

	out := make([]map[string]any, 0, len(shares))
	for _, sh := range shares {
		name := filepath.Base(sh.Path)
		isDir := false
		err = fsops.As(id, func() error {
			entry, e := a.sandbox.Stat(sh.Path)
			if e != nil {
				return e
			}
			name, isDir = entry.Name, entry.IsDir
			return nil
		})
		// A share whose target no longer exists/is inaccessible is still
		// listed (so it can be revoked), just with whatever we last knew.
		out = append(out, shareToJSON(sh, isDir, name))
	}
	writeJSON(w, http.StatusOK, map[string]any{"shares": out})
}

func (a *API) RevokeShare(w http.ResponseWriter, r *http.Request) {
	username := usernameFrom(r.Context())
	token := r.PathValue("token")
	if err := a.store.DeleteShare(token, username); err != nil {
		writeError(w, http.StatusInternalServerError, "could not revoke share")
		return
	}
	_ = a.store.RecordAudit(username, "share_revoke", "", clientIP(r), token)
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func shareToJSON(sh store.Share, isDir bool, name string) map[string]any {
	out := map[string]any{
		"token":       sh.Token,
		"path":        sh.Path,
		"name":        name,
		"isDir":       isDir,
		"mode":        sh.Mode,
		"hasPassword": sh.PasswordHash != "",
		"createdAt":   sh.CreatedAt,
		"url":         "/share/" + sh.Token,
	}
	if !sh.ExpiresAt.IsZero() {
		out["expiresAt"] = sh.ExpiresAt
	}
	return out
}

func generateShareToken() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// ---- Public, anonymous share serving ----

var errShareExpired = errors.New("share expired")
var errSharePasswordRequired = errors.New("password required")

// loadShare looks up a share by token and rejects it if expired.
func (a *API) loadShare(token string) (*store.Share, error) {
	sh, err := a.store.GetShare(token)
	if err != nil {
		return nil, err
	}
	if !sh.ExpiresAt.IsZero() && time.Now().After(sh.ExpiresAt) {
		return nil, errShareExpired
	}
	return sh, nil
}

func (a *API) ShareInfo(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	sh, err := a.loadShare(token)
	if err != nil {
		writeShareError(w, err)
		return
	}

	id, err := fsops.LookupIdentity(sh.Username)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "share owner no longer exists")
		return
	}
	var entry fsops.Entry
	err = fsops.As(id, func() error {
		var e error
		entry, e = a.sandbox.Stat(sh.Path)
		return e
	})
	if err != nil {
		writeFSError(w, err)
		return
	}

	unlocked := sh.PasswordHash == "" || a.shareUnlocked(r, sh.Token)
	writeJSON(w, http.StatusOK, map[string]any{
		"name":             entry.Name,
		"isDir":            entry.IsDir,
		"mode":             sh.Mode,
		"requiresPassword": sh.PasswordHash != "" && !unlocked,
	})
}

type unlockRequest struct {
	Password string `json:"password"`
}

func (a *API) ShareUnlock(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	sh, err := a.loadShare(token)
	if err != nil {
		writeShareError(w, err)
		return
	}
	if sh.PasswordHash == "" {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
		return
	}

	var req unlockRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(sh.PasswordHash), []byte(req.Password)) != nil {
		writeError(w, http.StatusUnauthorized, "incorrect password")
		return
	}

	secure := r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
	http.SetCookie(w, &http.Cookie{
		Name:     shareUnlockCookieName(sh.Token),
		Value:    a.shareUnlockMAC(sh.Token),
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (a *API) ShareList(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	sh, err := a.loadShare(token)
	if err != nil {
		writeShareError(w, err)
		return
	}
	if sh.Mode == "download_only" {
		writeError(w, http.StatusForbidden, "browsing is disabled for this share")
		return
	}
	if err := a.requireShareUnlocked(r, sh); err != nil {
		writeShareError(w, err)
		return
	}

	subPath := r.URL.Query().Get("path")
	id, err := fsops.LookupIdentity(sh.Username)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "share owner no longer exists")
		return
	}
	var entries []fsops.Entry
	err = fsops.As(id, func() error {
		var e error
		entries, e = a.sandbox.ListWithin(sh.Path, subPath)
		return e
	})
	if err != nil {
		writeFSError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"entries": entries})
}

func (a *API) ShareFile(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	sh, err := a.loadShare(token)
	if err != nil {
		writeShareError(w, err)
		return
	}
	download := r.URL.Query().Get("download") == "1"
	if sh.Mode == "view_only" && download {
		writeError(w, http.StatusForbidden, "downloads are disabled for this share")
		return
	}
	if err := a.requireShareUnlocked(r, sh); err != nil {
		writeShareError(w, err)
		return
	}

	subPath := r.URL.Query().Get("path")
	id, err := fsops.LookupIdentity(sh.Username)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "share owner no longer exists")
		return
	}
	var f *os.File
	var info os.FileInfo
	err = fsops.As(id, func() error {
		var e error
		f, info, e = a.sandbox.OpenWithin(sh.Path, subPath)
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

	ctype := mimeTypeFor(info.Name())
	w.Header().Set("Content-Type", ctype)
	if download {
		w.Header().Set("Content-Disposition", "attachment; filename=\""+info.Name()+"\"")
	}
	_ = a.store.RecordAudit(sh.Username, "share_download", sh.Path, clientIP(r), token)
	http.ServeContent(w, r, info.Name(), info.ModTime(), f)
}

func (a *API) requireShareUnlocked(r *http.Request, sh *store.Share) error {
	if sh.PasswordHash == "" {
		return nil
	}
	if !a.shareUnlocked(r, sh.Token) {
		return errSharePasswordRequired
	}
	return nil
}

func (a *API) shareUnlocked(r *http.Request, token string) bool {
	c, err := r.Cookie(shareUnlockCookieName(token))
	if err != nil {
		return false
	}
	expected := a.shareUnlockMAC(token)
	return subtle.ConstantTimeCompare([]byte(c.Value), []byte(expected)) == 1
}

func (a *API) shareUnlockMAC(token string) string {
	mac := hmac.New(sha256.New, a.shareUnlockSecret)
	mac.Write([]byte(token))
	return hex.EncodeToString(mac.Sum(nil))
}

func shareUnlockCookieName(token string) string {
	return "nimbusfs_share_" + token
}

func writeShareError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, errShareExpired):
		writeError(w, http.StatusGone, "this share link has expired")
	case errors.Is(err, errSharePasswordRequired):
		writeError(w, http.StatusUnauthorized, "password required")
	case errors.Is(err, fsops.ErrEscape):
		writeError(w, http.StatusBadRequest, "invalid path")
	default:
		writeError(w, http.StatusNotFound, "share not found")
	}
}
