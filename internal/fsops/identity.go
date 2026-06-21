package fsops

import (
	"fmt"
	"os/user"
	"runtime"
	"strconv"
	"sync"
	"syscall"
)

// Identity is the Linux account a request should be executed as, so that
// every filesystem syscall is subject to real Linux permission checks.
type Identity struct {
	Username string
	UID      int
	GID      int
	Groups   []int
}

// LookupIdentity resolves a Linux username to the identity used for impersonation.
func LookupIdentity(username string) (*Identity, error) {
	u, err := user.Lookup(username)
	if err != nil {
		return nil, fmt.Errorf("lookup user %q: %w", username, err)
	}
	uid, err := strconv.Atoi(u.Uid)
	if err != nil {
		return nil, fmt.Errorf("invalid uid for %q: %w", username, err)
	}
	gid, err := strconv.Atoi(u.Gid)
	if err != nil {
		return nil, fmt.Errorf("invalid gid for %q: %w", username, err)
	}
	groupIDs, err := u.GroupIds()
	if err != nil {
		return nil, fmt.Errorf("group lookup for %q: %w", username, err)
	}
	groups := make([]int, 0, len(groupIDs))
	for _, g := range groupIDs {
		n, err := strconv.Atoi(g)
		if err != nil {
			continue
		}
		groups = append(groups, n)
	}
	return &Identity{Username: username, UID: uid, GID: gid, Groups: groups}, nil
}

var (
	rootGroupsOnce sync.Once
	rootGroups     []int
)

// captureRootGroups records the process's original supplementary groups
// (captured once, before any impersonation) so each impersonated thread can
// be restored to a known-safe state before it's returned to the goroutine pool.
func captureRootGroups() []int {
	rootGroupsOnce.Do(func() {
		groups, err := syscall.Getgroups()
		if err == nil {
			rootGroups = groups
		}
	})
	return rootGroups
}

// As runs fn with the current OS thread's filesystem credentials (fsuid,
// fsgid, supplementary groups) switched to id, so kernel permission checks
// for every syscall fn performs are evaluated as that Linux user — not root.
//
// This relies on Go's syscall.Setfsuid/Setfsgid/Setgroups issuing the raw
// Linux syscall directly rather than glibc's NPTL wrapper, which means the
// change is scoped to the calling OS thread only. Combined with
// runtime.LockOSThread, that keeps the privilege change from leaking to any
// other goroutine that might later reuse this thread, and from affecting
// goroutines running concurrently on other threads.
func As(id *Identity, fn func() error) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	captureRootGroups()

	if err := syscall.Setgroups(id.Groups); err != nil {
		return fmt.Errorf("setgroups: %w", err)
	}
	if _, err := setfsgid(id.GID); err != nil {
		syscall.Setgroups(rootGroups)
		return fmt.Errorf("setfsgid: %w", err)
	}
	if _, err := setfsuid(id.UID); err != nil {
		setfsgid(0)
		syscall.Setgroups(rootGroups)
		return fmt.Errorf("setfsuid: %w", err)
	}

	defer func() {
		setfsuid(0)
		setfsgid(0)
		syscall.Setgroups(rootGroups)
	}()

	return fn()
}
