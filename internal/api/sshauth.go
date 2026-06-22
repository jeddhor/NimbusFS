package api

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"time"

	"golang.org/x/crypto/ssh"

	"nimbusfs/internal/auth"
	"nimbusfs/internal/fsops"
)

type sshStartRequest struct {
	Username string `json:"username"`
}

// SSHStart begins a device-flow SSH key login: it verifies the username
// exists and has at least one usable authorized key, then hands the
// browser a short user-facing code plus a private poll token.
func (a *API) SSHStart(w http.ResponseWriter, r *http.Request) {
	if !a.cfg.Auth.SSHKeys {
		writeError(w, http.StatusNotFound, "ssh key auth is disabled")
		return
	}
	var req sshStartRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Username == "" {
		writeError(w, http.StatusBadRequest, "username is required")
		return
	}

	ip := clientIP(r)
	count, err := a.store.RecentFailedAttempts(ip, time.Now().Add(-time.Minute))
	if err == nil && count >= maxLoginAttemptsPerMinute {
		writeError(w, http.StatusTooManyRequests, "too many login attempts, try again later")
		return
	}

	if _, err := fsops.LookupIdentity(req.Username); err != nil {
		writeError(w, http.StatusBadRequest, "unknown system user")
		return
	}
	if _, err := auth.LoadAuthorizedKeys(req.Username); err != nil {
		writeError(w, http.StatusBadRequest, "no usable SSH keys configured for this account")
		return
	}

	challenge, err := a.sshDevices.New(req.Username)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not start ssh login")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"code":      challenge.UserCode,
		"pollToken": challenge.PollToken,
		"expiresIn": int(auth.DeviceCodeTTL().Seconds()),
	})
}

// SSHPoll is long-polled by the browser tab that started the flow. Once the
// CLI has successfully signed the challenge, this is what actually creates
// the session and sets the cookie — on the browser's own connection.
func (a *API) SSHPoll(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("pollToken")
	if token == "" {
		writeError(w, http.StatusBadRequest, "pollToken is required")
		return
	}
	challenge, ok := a.sshDevices.ByPollToken(token)
	if !ok {
		writeError(w, http.StatusNotFound, "code expired or unknown")
		return
	}
	if !challenge.Approved() {
		writeJSON(w, http.StatusOK, map[string]string{"status": "pending"})
		return
	}

	secure := r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https"
	cookie, err := a.sessions.Create(challenge.Username, false, secure)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not create session")
		return
	}
	http.SetCookie(w, cookie)
	setCSRFCookie(w, secure)
	a.sshDevices.Consume(token)
	_ = a.store.RecordAudit(challenge.Username, "login_ssh", "", clientIP(r), "")
	writeJSON(w, http.StatusOK, map[string]string{"status": "approved", "username": challenge.Username})
}

// SSHDeviceInfo is fetched by the CLI: it exchanges the short user code for
// the actual nonce to sign and the username/fingerprints it should sign with.
func (a *API) SSHDeviceInfo(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	challenge, ok := a.sshDevices.ByCode(code)
	if !ok {
		writeError(w, http.StatusNotFound, "code expired or unknown")
		return
	}
	keys, err := auth.LoadAuthorizedKeys(challenge.Username)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load authorized keys")
		return
	}
	fingerprints := make([]string, 0, len(keys))
	for _, k := range keys {
		fingerprints = append(fingerprints, auth.FingerprintSHA256(k))
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"username":     challenge.Username,
		"nonce":        base64.StdEncoding.EncodeToString(challenge.Nonce),
		"fingerprints": fingerprints,
	})
}

type sshRespondRequest struct {
	Code      string `json:"code"`
	PublicKey string `json:"publicKey"` // base64 of the SSH wire-format public key blob
	SigFormat string `json:"sigFormat"`
	Signature string `json:"signature"` // base64 of the raw signature blob
}

// SSHRespond is called by the CLI with a signature over the challenge nonce.
// On success the challenge is marked approved for the waiting browser poll.
func (a *API) SSHRespond(w http.ResponseWriter, r *http.Request) {
	var req sshRespondRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	challenge, ok := a.sshDevices.ByCode(req.Code)
	if !ok {
		writeError(w, http.StatusNotFound, "code expired or unknown")
		return
	}

	pubKeyBlob, err := base64.StdEncoding.DecodeString(req.PublicKey)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid public key encoding")
		return
	}
	sigBlob, err := base64.StdEncoding.DecodeString(req.Signature)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid signature encoding")
		return
	}

	candidate, err := ssh.ParsePublicKey(pubKeyBlob)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid public key")
		return
	}

	authorized, err := auth.LoadAuthorizedKeys(challenge.Username)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load authorized keys")
		return
	}
	if !keyIsAuthorized(candidate, authorized) {
		registerFailedAttempt(w, a, req.Code)
		return
	}

	sig := &ssh.Signature{Format: req.SigFormat, Blob: sigBlob}
	if err := candidate.Verify(challenge.Nonce, sig); err != nil {
		registerFailedAttempt(w, a, req.Code)
		return
	}

	a.sshDevices.Approve(req.Code)
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "username": challenge.Username})
}

func registerFailedAttempt(w http.ResponseWriter, a *API, code string) {
	if _, ok := a.sshDevices.RegisterAttempt(code); !ok {
		writeError(w, http.StatusTooManyRequests, "too many failed attempts, request a new code")
		return
	}
	writeError(w, http.StatusUnauthorized, "signature verification failed")
}

func keyIsAuthorized(candidate ssh.PublicKey, authorized []ssh.PublicKey) bool {
	candidateBytes := candidate.Marshal()
	for _, k := range authorized {
		if string(k.Marshal()) == string(candidateBytes) {
			return true
		}
	}
	return false
}
