package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type Store struct {
	db *sql.DB
}

func Open(dataDir string) (*Store, error) {
	if err := os.MkdirAll(dataDir, 0750); err != nil {
		return nil, fmt.Errorf("creating data dir: %w", err)
	}
	dbPath := filepath.Join(dataDir, "nimbusfs.db")
	db, err := sql.Open("sqlite3", dbPath+"?_foreign_keys=on")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1) // sqlite3 driver is not safe for concurrent writers across connections
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) migrate() error {
	schema := `
CREATE TABLE IF NOT EXISTS sessions (
	id TEXT PRIMARY KEY,
	username TEXT NOT NULL,
	created_at INTEGER NOT NULL,
	last_seen_at INTEGER NOT NULL,
	expires_at INTEGER NOT NULL,
	remember INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS audit_log (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	ts INTEGER NOT NULL,
	username TEXT NOT NULL,
	action TEXT NOT NULL,
	path TEXT NOT NULL DEFAULT '',
	remote_addr TEXT NOT NULL DEFAULT '',
	detail TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS login_attempts (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	username TEXT NOT NULL,
	remote_addr TEXT NOT NULL,
	ts INTEGER NOT NULL,
	success INTEGER NOT NULL
);
`
	_, err := s.db.Exec(schema)
	return err
}

type Session struct {
	ID         string
	Username   string
	CreatedAt  time.Time
	LastSeenAt time.Time
	ExpiresAt  time.Time
	Remember   bool
}

func (s *Store) CreateSession(sess Session) error {
	_, err := s.db.Exec(
		`INSERT INTO sessions (id, username, created_at, last_seen_at, expires_at, remember) VALUES (?, ?, ?, ?, ?, ?)`,
		sess.ID, sess.Username, sess.CreatedAt.Unix(), sess.LastSeenAt.Unix(), sess.ExpiresAt.Unix(), boolToInt(sess.Remember),
	)
	return err
}

func (s *Store) GetSession(id string) (*Session, error) {
	row := s.db.QueryRow(`SELECT id, username, created_at, last_seen_at, expires_at, remember FROM sessions WHERE id = ?`, id)
	var sess Session
	var created, lastSeen, expires int64
	var remember int
	if err := row.Scan(&sess.ID, &sess.Username, &created, &lastSeen, &expires, &remember); err != nil {
		return nil, err
	}
	sess.CreatedAt = time.Unix(created, 0)
	sess.LastSeenAt = time.Unix(lastSeen, 0)
	sess.ExpiresAt = time.Unix(expires, 0)
	sess.Remember = remember != 0
	return &sess, nil
}

func (s *Store) TouchSession(id string, lastSeen, expires time.Time) error {
	_, err := s.db.Exec(`UPDATE sessions SET last_seen_at = ?, expires_at = ? WHERE id = ?`, lastSeen.Unix(), expires.Unix(), id)
	return err
}

func (s *Store) DeleteSession(id string) error {
	_, err := s.db.Exec(`DELETE FROM sessions WHERE id = ?`, id)
	return err
}

func (s *Store) DeleteSessionsForUser(username string) error {
	_, err := s.db.Exec(`DELETE FROM sessions WHERE username = ?`, username)
	return err
}

func (s *Store) DeleteExpiredSessions(now time.Time) error {
	_, err := s.db.Exec(`DELETE FROM sessions WHERE expires_at < ?`, now.Unix())
	return err
}

func (s *Store) RecordAudit(username, action, path, remoteAddr, detail string) error {
	_, err := s.db.Exec(
		`INSERT INTO audit_log (ts, username, action, path, remote_addr, detail) VALUES (?, ?, ?, ?, ?, ?)`,
		time.Now().Unix(), username, action, path, remoteAddr, detail,
	)
	return err
}

func (s *Store) RecordLoginAttempt(username, remoteAddr string, success bool) error {
	_, err := s.db.Exec(
		`INSERT INTO login_attempts (username, remote_addr, ts, success) VALUES (?, ?, ?, ?)`,
		username, remoteAddr, time.Now().Unix(), boolToInt(success),
	)
	return err
}

// RecentFailedAttempts counts failed login attempts for an identifier
// (username or remote address) within the given window, for rate limiting.
func (s *Store) RecentFailedAttempts(remoteAddr string, since time.Time) (int, error) {
	row := s.db.QueryRow(
		`SELECT COUNT(*) FROM login_attempts WHERE remote_addr = ? AND ts >= ? AND success = 0`,
		remoteAddr, since.Unix(),
	)
	var count int
	if err := row.Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
