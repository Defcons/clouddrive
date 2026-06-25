package services

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

func toSlashClean(p string) string {
	return filepath.ToSlash(filepath.Clean(p))
}

// movePathKeys re-keys m so oldPath and any descendant key (oldPath + "/...")
// move to the equivalent key under newPath. Returns true if anything changed.
// Used to keep per-path metadata (permissions, tags, backup tiers) attached to
// a file/folder when it is renamed or moved.
func movePathKeys[V any](m map[string]V, oldPath, newPath string) bool {
	oldClean := toSlashClean(oldPath)
	newClean := toSlashClean(newPath)
	if oldClean == newClean {
		return false
	}
	moved := map[string]V{}
	for k, v := range m {
		if k == oldClean || strings.HasPrefix(k, oldClean+"/") {
			moved[k] = v
		}
	}
	if len(moved) == 0 {
		return false
	}
	for k, v := range moved {
		delete(m, k)
		m[newClean+strings.TrimPrefix(k, oldClean)] = v
	}
	return true
}

// loadJSONFile reads path and unmarshals it into v. A missing file is a no-op
// (fresh store). If the file exists but is corrupt, it is preserved as
// <path>.corrupt and the error is logged, so the next save() doesn't silently
// overwrite recoverable data with an empty store.
func loadJSONFile(path string, v any) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	if err := json.Unmarshal(data, v); err != nil {
		slog.Error("corrupt JSON store; preserving as .corrupt and starting empty", "path", path, "err", err)
		if rerr := os.Rename(path, path+".corrupt"); rerr != nil {
			slog.Error("failed to preserve corrupt store", "path", path, "err", rerr)
		}
	}
}
