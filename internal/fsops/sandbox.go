package fsops

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ErrEscape is returned whenever a request path would resolve outside the
// configured root directory, including via a symlink.
var ErrEscape = errors.New("path escapes sandbox root")

// Sandbox roots all filesystem operations under a single directory and
// guarantees no resolved path — including through symlinks — can leave it.
type Sandbox struct {
	root string
}

func NewSandbox(root string) (*Sandbox, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return nil, fmt.Errorf("resolving root %q: %w", root, err)
	}
	return &Sandbox{root: resolved}, nil
}

func (s *Sandbox) Root() string { return s.root }

// Resolve maps a request path like "/movies/file.mp4" to an absolute path on
// disk, guaranteeing the result is contained within the sandbox root.
//
// It is intentionally strict: if any path component is a symlink whose
// target escapes the root, resolution fails rather than silently clamping,
// since clamping a symlink target can itself be exploited.
func (s *Sandbox) Resolve(requestPath string) (string, error) {
	cleaned := filepath.Clean("/" + requestPath)
	joined := filepath.Join(s.root, cleaned)

	if !s.within(joined) {
		return "", ErrEscape
	}

	// Resolve symlinks on the deepest existing ancestor so a non-existent
	// leaf (e.g. a file about to be created) can still be validated.
	existing, rest := deepestExisting(joined)
	resolved, err := filepath.EvalSymlinks(existing)
	if err != nil {
		return "", fmt.Errorf("resolving path: %w", err)
	}
	if !s.within(resolved) {
		return "", ErrEscape
	}

	full := filepath.Join(resolved, rest)
	if !s.within(full) {
		return "", ErrEscape
	}
	return full, nil
}

func (s *Sandbox) within(p string) bool {
	if p == s.root {
		return true
	}
	return strings.HasPrefix(p, s.root+string(os.PathSeparator))
}

// RelPath converts an absolute on-disk path back to a request-style path
// rooted at "/". Used when reporting paths back to the API/UI.
func (s *Sandbox) RelPath(absPath string) (string, error) {
	rel, err := filepath.Rel(s.root, absPath)
	if err != nil {
		return "", err
	}
	if rel == "." {
		return "/", nil
	}
	return "/" + rel, nil
}

func deepestExisting(p string) (existing string, rest string) {
	cur := p
	var parts []string
	for {
		if _, err := os.Lstat(cur); err == nil {
			break
		}
		parts = append([]string{filepath.Base(cur)}, parts...)
		parent := filepath.Dir(cur)
		if parent == cur {
			// Reached filesystem root without finding anything that exists.
			return parent, filepath.Join(parts...)
		}
		cur = parent
	}
	return cur, filepath.Join(parts...)
}
