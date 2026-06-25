//go:build unix

package handlers

import "syscall"

// fsSpace returns total and available bytes for the filesystem backing path.
// Returns 0, 0 if the syscall fails.
func fsSpace(path string) (total, free int64) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return 0, 0
	}
	return int64(stat.Blocks) * int64(stat.Bsize), int64(stat.Bavail) * int64(stat.Bsize)
}
