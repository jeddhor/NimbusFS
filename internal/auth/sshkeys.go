package auth

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"syscall"

	"golang.org/x/crypto/ssh"
)

// LoadAuthorizedKeys reads and parses ~<username>/.ssh/authorized_keys.
//
// The server runs as root, so it can read any user's file regardless of
// permissions — but that's exactly why it replicates sshd's StrictModes
// check: the user's home directory, .ssh directory, and authorized_keys
// file must all be owned by that user or root and not group/world-writable.
// Without this, anyone who can write into another user's home directory
// (e.g. a misconfigured shared mount) could plant a key and impersonate them.
func LoadAuthorizedKeys(username string) ([]ssh.PublicKey, error) {
	u, err := user.Lookup(username)
	if err != nil {
		return nil, fmt.Errorf("lookup user %q: %w", username, err)
	}

	sshDir := filepath.Join(u.HomeDir, ".ssh")
	keysPath := filepath.Join(sshDir, "authorized_keys")

	if err := checkStrictPerms(u.HomeDir, username); err != nil {
		return nil, err
	}
	if err := checkStrictPerms(sshDir, username); err != nil {
		return nil, err
	}
	if err := checkStrictPerms(keysPath, username); err != nil {
		return nil, err
	}

	data, err := os.ReadFile(keysPath)
	if err != nil {
		return nil, err
	}

	var keys []ssh.PublicKey
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 || line[0] == '#' {
			continue
		}
		pk, _, _, _, err := ssh.ParseAuthorizedKey(line)
		if err != nil {
			continue // skip malformed/unsupported lines rather than failing the whole file
		}
		keys = append(keys, pk)
	}
	if len(keys) == 0 {
		return nil, fmt.Errorf("no usable keys in %s", keysPath)
	}
	return keys, nil
}

// checkStrictPerms mirrors sshd's StrictModes: the path must be owned by
// root or the target user, and must not be writable by group or other.
func checkStrictPerms(path, username string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return nil
	}
	if stat.Uid != 0 {
		u, err := user.LookupId(fmt.Sprint(stat.Uid))
		if err != nil || u.Username != username {
			return fmt.Errorf("%s must be owned by %q or root (StrictModes)", path, username)
		}
	}
	if info.Mode()&0022 != 0 {
		return fmt.Errorf("%s must not be group- or world-writable (StrictModes)", path)
	}
	return nil
}

// FingerprintSHA256 is exposed so the CLI/API can advertise which local
// keys would be accepted without leaking the full authorized_keys contents.
func FingerprintSHA256(pk ssh.PublicKey) string {
	return ssh.FingerprintSHA256(pk)
}
