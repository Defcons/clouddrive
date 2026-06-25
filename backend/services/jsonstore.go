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

// prunePathKeys removes the entry for path and any descendant key. Returns true
// if anything was removed. Used to drop per-path metadata when a file/folder is
// permanently deleted, so a path recreated later doesn't inherit stale state.
func prunePathKeys[V any](m map[string]V, path string) bool {
	clean := toSlashClean(path)
	var keys []string
	for k := range m {
		if k == clean || strings.HasPrefix(k, clean+"/") {
			keys = append(keys, k)
		}
	}
	for _, k := range keys {
		delete(m, k)
	}
	return len(keys) > 0
}

// saveJSONFile marshals v to path atomically (write temp + rename).
func saveJSONFile(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
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
