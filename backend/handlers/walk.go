package handlers

import (
	"os"
	"path/filepath"
)

// walkFilesNoSymlinks walks root recursively, invoking fn for every regular
// file. Symlinks are skipped entirely — this prevents zip/search/preview
// endpoints from exfiltrating files outside the storage root via symlink
// traversal (e.g. a user placing `link -> /etc/passwd` in their home folder
// and then downloading the parent as a zip).
//
// fn receives (absolute path, FileInfo, path relative to root).
func walkFilesNoSymlinks(root string, fn func(path string, info os.FileInfo, relPath string)) {
	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil {
			return nil
		}
		// Skip symlinks entirely (Walk doesn't follow them, but Lstat-mode
		// files may still appear — skip both dirs and files).
		if info.Mode()&os.ModeSymlink != 0 {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if info.IsDir() {
			return nil
		}
		relPath, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}
		fn(path, info, relPath)
		return nil
	})
}
