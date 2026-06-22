package fsops

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

var ErrNotExist = os.ErrNotExist

type Entry struct {
	Name     string    `json:"name"`
	Path     string    `json:"path"`
	IsDir    bool      `json:"isDir"`
	Size     int64     `json:"size"`
	Modified time.Time `json:"modified"`
	Mode     string    `json:"mode"`
	Owner    string    `json:"owner"`
	Group    string    `json:"group"`
}

// List returns the entries of a directory. Callers must invoke this inside
// fsops.As so the listing reflects the requesting user's real permissions.
func (s *Sandbox) List(reqPath string) ([]Entry, error) {
	abs, err := s.Resolve(reqPath)
	if err != nil {
		return nil, err
	}
	return s.listAbs(abs)
}

// ListWithin lists a directory like List, but additionally requires the
// result to stay within scopeReq — used to serve share links scoped to a
// single subtree rather than the whole sandbox.
func (s *Sandbox) ListWithin(scopeReq, subPath string) ([]Entry, error) {
	abs, err := s.ResolveWithin(scopeReq, subPath)
	if err != nil {
		return nil, err
	}
	return s.listAbs(abs)
}

func (s *Sandbox) listAbs(abs string) ([]Entry, error) {
	dirEntries, err := os.ReadDir(abs)
	if err != nil {
		return nil, err
	}
	entries := make([]Entry, 0, len(dirEntries))
	for _, de := range dirEntries {
		info, err := de.Info()
		if err != nil {
			// Skip entries we can't stat (e.g. broken symlink, race with deletion).
			continue
		}
		childAbs := filepath.Join(abs, de.Name())
		childRel, err := s.RelPath(childAbs)
		if err != nil {
			continue
		}
		entries = append(entries, entryFromInfo(de.Name(), childRel, info))
	}
	return entries, nil
}

// Stat returns metadata for a single path.
func (s *Sandbox) Stat(reqPath string) (Entry, error) {
	abs, err := s.Resolve(reqPath)
	if err != nil {
		return Entry{}, err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return Entry{}, err
	}
	return entryFromInfo(info.Name(), reqPath, info), nil
}

// StatWithin is the share-scoped equivalent of Stat.
func (s *Sandbox) StatWithin(scopeReq, subPath string) (Entry, error) {
	abs, err := s.ResolveWithin(scopeReq, subPath)
	if err != nil {
		return Entry{}, err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return Entry{}, err
	}
	rel, err := s.RelPath(abs)
	if err != nil {
		return Entry{}, err
	}
	return entryFromInfo(info.Name(), rel, info), nil
}

// AbsPath resolves a request path to an absolute on-disk path without
// performing any I/O beyond what Resolve needs for symlink safety.
func (s *Sandbox) AbsPath(reqPath string) (string, error) {
	return s.Resolve(reqPath)
}

func (s *Sandbox) Mkdir(reqPath string) error {
	abs, err := s.Resolve(reqPath)
	if err != nil {
		return err
	}
	return os.Mkdir(abs, 0755)
}

func (s *Sandbox) CreateFile(reqPath string) error {
	abs, err := s.Resolve(reqPath)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(abs, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	return f.Close()
}

func (s *Sandbox) Delete(reqPath string) error {
	abs, err := s.Resolve(reqPath)
	if err != nil {
		return err
	}
	if abs == s.root {
		return errors.New("cannot delete root directory")
	}
	return os.RemoveAll(abs)
}

func (s *Sandbox) Rename(reqPath, newName string) (string, error) {
	abs, err := s.Resolve(reqPath)
	if err != nil {
		return "", err
	}
	if abs == s.root {
		return "", errors.New("cannot rename root directory")
	}
	dest := filepath.Join(filepath.Dir(abs), newName)
	if !s.within(dest) {
		return "", ErrEscape
	}
	if err := os.Rename(abs, dest); err != nil {
		return "", err
	}
	return s.RelPath(dest)
}

func (s *Sandbox) Move(srcReq, destReq string) error {
	src, err := s.Resolve(srcReq)
	if err != nil {
		return err
	}
	if src == s.root {
		return errors.New("cannot move root directory")
	}
	dest, err := s.Resolve(destReq)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return err
	}
	return os.Rename(src, dest)
}

func (s *Sandbox) Copy(srcReq, destReq string) error {
	src, err := s.Resolve(srcReq)
	if err != nil {
		return err
	}
	dest, err := s.Resolve(destReq)
	if err != nil {
		return err
	}
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return copyDir(src, dest)
	}
	return copyFile(src, dest, info.Mode())
}

// Open opens a file for reading (downloads, previews). Caller is responsible
// for closing it, and for invoking this inside fsops.As.
func (s *Sandbox) Open(reqPath string) (*os.File, os.FileInfo, error) {
	abs, err := s.Resolve(reqPath)
	if err != nil {
		return nil, nil, err
	}
	return openAbs(abs)
}

// OpenWithin is the share-scoped equivalent of Open.
func (s *Sandbox) OpenWithin(scopeReq, subPath string) (*os.File, os.FileInfo, error) {
	abs, err := s.ResolveWithin(scopeReq, subPath)
	if err != nil {
		return nil, nil, err
	}
	return openAbs(abs)
}

func openAbs(abs string) (*os.File, os.FileInfo, error) {
	f, err := os.Open(abs)
	if err != nil {
		return nil, nil, err
	}
	info, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, nil, err
	}
	return f, info, nil
}

// Create opens a destination file for writing (uploads). Caller closes it.
func (s *Sandbox) Create(reqPath string) (*os.File, error) {
	abs, err := s.Resolve(reqPath)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(abs), 0755); err != nil {
		return nil, err
	}
	return os.OpenFile(abs, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
}

func copyFile(src, dest string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dest, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return nil
}

func copyDir(src, dest string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dest, info.Mode()); err != nil {
		return err
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, e := range entries {
		srcPath := filepath.Join(src, e.Name())
		destPath := filepath.Join(dest, e.Name())
		if e.IsDir() {
			if err := copyDir(srcPath, destPath); err != nil {
				return err
			}
			continue
		}
		info, err := e.Info()
		if err != nil {
			return err
		}
		if err := copyFile(srcPath, destPath, info.Mode()); err != nil {
			return fmt.Errorf("copying %s: %w", srcPath, err)
		}
	}
	return nil
}

func entryFromInfo(name, relPath string, info os.FileInfo) Entry {
	owner, group := ownerGroup(info)
	return Entry{
		Name:     name,
		Path:     relPath,
		IsDir:    info.IsDir(),
		Size:     info.Size(),
		Modified: info.ModTime(),
		Mode:     info.Mode().String(),
		Owner:    owner,
		Group:    group,
	}
}
