package fsops

import (
	"os"
	"os/user"
	"strconv"
	"sync"
	"syscall"
)

var (
	userCacheMu sync.Mutex
	userCache   = map[uint32]string{}
	groupCache  = map[uint32]string{}
)

// ownerGroup resolves the numeric uid/gid embedded in a FileInfo to names,
// caching lookups since the same handful of owners repeat across a listing.
func ownerGroup(info os.FileInfo) (owner, group string) {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return "", ""
	}
	return LookupUserName(stat.Uid), LookupGroupName(stat.Gid)
}

// StatT extracts the raw uid/gid/mode from a FileInfo, for callers (like the
// search indexer) that need them without going through the Entry/JSON path.
func StatT(info os.FileInfo) (uid, gid uint32, ok bool) {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, 0, false
	}
	return stat.Uid, stat.Gid, true
}

// LookupUserName resolves a uid to a username, caching lookups.
func LookupUserName(uid uint32) string {
	userCacheMu.Lock()
	defer userCacheMu.Unlock()
	if name, ok := userCache[uid]; ok {
		return name
	}
	name := strconv.FormatUint(uint64(uid), 10)
	if u, err := user.LookupId(name); err == nil {
		name = u.Username
	}
	userCache[uid] = name
	return name
}

// LookupGroupName resolves a gid to a group name, caching lookups.
func LookupGroupName(gid uint32) string {
	userCacheMu.Lock()
	defer userCacheMu.Unlock()
	if name, ok := groupCache[gid]; ok {
		return name
	}
	name := strconv.FormatUint(uint64(gid), 10)
	if g, err := user.LookupGroupId(name); err == nil {
		name = g.Name
	}
	groupCache[gid] = name
	return name
}
