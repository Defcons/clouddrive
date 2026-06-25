//go:build unix

package handlers

import (
	"os"
	"syscall"
)

// statCreationMillis returns the inode status-change time (Ctim) in epoch
// milliseconds — the closest proxy to a creation time available on Unix.
// ok is false when the platform stat info isn't present.
func statCreationMillis(info os.FileInfo) (int64, bool) {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, false
	}
	ms := stat.Ctim.Sec*1000 + stat.Ctim.Nsec/1000000
	if ms <= 0 {
		return 0, false
	}
	return ms, true
}
