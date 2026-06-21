package auth

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"net/http"
	"time"

	"nimbusfs/internal/store"
)

const (
	CookieName         = "nimbusfs_session"
	IdleTimeout        = 30 * time.Minute
	RememberMeDuration = 30 * 24 * time.Hour
)

var ErrSessionInvalid = errors.New("session invalid or expired")

type SessionManager struct {
	store *store.Store
}

func NewSessionManager(st *store.Store) *SessionManager {
	return &SessionManager{store: st}
}

func generateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// Create starts a new session for username and returns the cookie to set.
func (m *SessionManager) Create(username string, remember, secure bool) (*http.Cookie, error) {
	token, err := generateToken()
	if err != nil {
		return nil, err
	}
	now := time.Now()
	expires := now.Add(IdleTimeout)
	if remember {
		expires = now.Add(RememberMeDuration)
	}
	sess := store.Session{
		ID:         token,
		Username:   username,
		CreatedAt:  now,
		LastSeenAt: now,
		ExpiresAt:  expires,
		Remember:   remember,
	}
	if err := m.store.CreateSession(sess); err != nil {
		return nil, err
	}
	cookie := &http.Cookie{
		Name:     CookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	}
	if remember {
		cookie.Expires = expires
	}
	return cookie, nil
}

// Validate looks up the session for an incoming request cookie, enforcing
// idle timeout and absolute expiry, and refreshes last-seen/idle expiry on success.
func (m *SessionManager) Validate(token string) (username string, err error) {
	sess, err := m.store.GetSession(token)
	if err != nil {
		return "", ErrSessionInvalid
	}
	now := time.Now()
	if now.After(sess.ExpiresAt) {
		_ = m.store.DeleteSession(token)
		return "", ErrSessionInvalid
	}
	if !sess.Remember && now.Sub(sess.LastSeenAt) > IdleTimeout {
		_ = m.store.DeleteSession(token)
		return "", ErrSessionInvalid
	}
	newExpiry := now.Add(IdleTimeout)
	if sess.Remember {
		newExpiry = sess.ExpiresAt // remember-me sessions use absolute expiry, not idle-refreshed
	}
	_ = m.store.TouchSession(token, now, newExpiry)
	return sess.Username, nil
}

func (m *SessionManager) Destroy(token string) error {
	return m.store.DeleteSession(token)
}

func (m *SessionManager) DestroyAllForUser(username string) error {
	return m.store.DeleteSessionsForUser(username)
}
