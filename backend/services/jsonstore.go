package services

import (
	"encoding/json"
	"log/slog"
	"os"
)

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
