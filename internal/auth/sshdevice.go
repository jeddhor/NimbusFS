package auth

import (
	"crypto/rand"
	"encoding/base32"
	"encoding/base64"
	"sync"
	"time"
)

const (
	deviceCodeTTL     = 5 * time.Minute
	maxVerifyAttempts = 5
)

// DeviceCodeTTL reports how long a device-flow code remains valid.
func DeviceCodeTTL() time.Duration { return deviceCodeTTL }

type challengeStatus int

const (
	statusPending challengeStatus = iota
	statusApproved
)

// SSHChallenge is one in-flight "sign in with SSH key" device-flow attempt:
// a browser tab holds PollToken, the CLI run by the user holds UserCode,
// and both refer to the same Nonce that must be signed by an authorized key.
type SSHChallenge struct {
	UserCode  string
	PollToken string
	Username  string
	Nonce     []byte
	Status    challengeStatus
	Attempts  int
	ExpiresAt time.Time
}

// SSHDeviceStore holds in-flight device-flow challenges in memory. They are
// short-lived (minutes) and single-use, so durability across restarts isn't
// needed — unlike sessions, which live in sqlite.
type SSHDeviceStore struct {
	mu      sync.Mutex
	byCode  map[string]*SSHChallenge
	byToken map[string]*SSHChallenge
}

func NewSSHDeviceStore() *SSHDeviceStore {
	return &SSHDeviceStore{
		byCode:  map[string]*SSHChallenge{},
		byToken: map[string]*SSHChallenge{},
	}
}

func randomString(encoding *base32.Encoding, byteLen int) (string, error) {
	b := make([]byte, byteLen)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return encoding.EncodeToString(b), nil
}

// userCodeEncoding produces short, unambiguous, easy-to-type codes (no
// padding, uppercase, excludes visually confusable characters via base32).
var userCodeEncoding = base32.StdEncoding.WithPadding(base32.NoPadding)

func (s *SSHDeviceStore) New(username string) (*SSHChallenge, error) {
	nonce := make([]byte, 32)
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	codeRaw, err := randomString(userCodeEncoding, 5)
	if err != nil {
		return nil, err
	}
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return nil, err
	}

	c := &SSHChallenge{
		UserCode:  codeRaw[:8],
		PollToken: base64.RawURLEncoding.EncodeToString(tokenBytes),
		Username:  username,
		Nonce:     nonce,
		Status:    statusPending,
		ExpiresAt: time.Now().Add(deviceCodeTTL),
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.sweep()
	s.byCode[c.UserCode] = c
	s.byToken[c.PollToken] = c
	return c, nil
}

// Approved reports whether the CLI has successfully signed this challenge.
func (c *SSHChallenge) Approved() bool { return c.Status == statusApproved }

func (s *SSHDeviceStore) ByCode(code string) (*SSHChallenge, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.byCode[code]
	if !ok || time.Now().After(c.ExpiresAt) {
		return nil, false
	}
	return c, true
}

func (s *SSHDeviceStore) ByPollToken(token string) (*SSHChallenge, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.byToken[token]
	if !ok || time.Now().After(c.ExpiresAt) {
		return nil, false
	}
	return c, true
}

// RegisterAttempt increments the failed-verification counter and reports
// whether the challenge has exceeded its retry budget (brute-force guard).
func (s *SSHDeviceStore) RegisterAttempt(code string) (attemptsLeft int, ok bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, found := s.byCode[code]
	if !found {
		return 0, false
	}
	c.Attempts++
	left := maxVerifyAttempts - c.Attempts
	if left <= 0 {
		delete(s.byCode, c.UserCode)
		delete(s.byToken, c.PollToken)
		return 0, false
	}
	return left, true
}

func (s *SSHDeviceStore) Approve(code string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.byCode[code]
	if !ok {
		return false
	}
	c.Status = statusApproved
	return true
}

// Consume removes a challenge once the browser has picked up its approval,
// making the device code and poll token single-use.
func (s *SSHDeviceStore) Consume(pollToken string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.byToken[pollToken]
	if !ok {
		return
	}
	delete(s.byCode, c.UserCode)
	delete(s.byToken, c.PollToken)
}

func (s *SSHDeviceStore) sweep() {
	now := time.Now()
	for code, c := range s.byCode {
		if now.After(c.ExpiresAt) {
			delete(s.byCode, code)
			delete(s.byToken, c.PollToken)
		}
	}
}
