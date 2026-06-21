package fsops

import "golang.org/x/sys/unix"

// setfsuid/setfsgid return the previous fsuid/fsgid like the underlying
// syscalls so callers can detect failure (e.g. missing CAP_SETUID/CAP_SETGID).
func setfsuid(uid int) (int, error) {
	return unix.SetfsuidRetUid(uid)
}

func setfsgid(gid int) (int, error) {
	return unix.SetfsgidRetGid(gid)
}
