package fsops

import "os"

// CanRead reports whether id would be able to read an entry with the given
// owner/group/mode, using standard Unix owner/group/other permission bits.
//
// This is used by the search index to filter results without re-running a
// real impersonated syscall per hit (which wouldn't meet the search latency
// target at scale). It's a deliberate approximation: it checks only the
// target entry's own mode bits, not execute permission on every ancestor
// directory, and it doesn't know about POSIX ACLs or other extended
// permission mechanisms. Actually opening or listing a result still goes
// through the real impersonated path in Sandbox, so this can't grant access
// beyond what the kernel would allow — at worst it can show a result that
// then 403s when opened, or hide one an ACL would have permitted.
func CanRead(id *Identity, ownerUID, ownerGID uint32, mode os.FileMode) bool {
	perm := mode.Perm()
	if uint32(id.UID) == ownerUID {
		return perm&0400 != 0
	}
	if uint32(id.GID) == ownerGID {
		return perm&0040 != 0
	}
	for _, g := range id.Groups {
		if uint32(g) == ownerGID {
			return perm&0040 != 0
		}
	}
	return perm&0004 != 0
}
