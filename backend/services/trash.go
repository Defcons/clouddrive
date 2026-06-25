package services

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type TrashItem struct {
	ID           string `json:"id"`
	OriginalPath string `json:"originalPath"`
	Name         string `json:"name"`
	IsDir        bool   `json:"isDir"`
	Size         int64  `json:"size"`
	DeletedBy    string `json:"deletedBy"`
	DeletedAt    int64  `json:"deletedAt"`
	TrashPath    string `json:"trashPath"` // actual path in .trash/
}

type TrashStore struct {
	root     string
	trashDir string
	manifest []TrashItem
	mu       sync.Mutex
}

func NewTrashStore(storageRoot string) *TrashStore {
	trashDir := filepath.Join(storageRoot, ".trash")
	os.MkdirAll(trashDir, 0755)

	store := &TrashStore{
		root:     storageRoot,
		trashDir: trashDir,
		manifest: make([]TrashItem, 0),
	}
	store.load()
	return store
}

func (s *TrashStore) load() {
	data, err := os.ReadFile(filepath.Join(s.trashDir, "manifest.json"))
	if err != nil {
		return
	}
	json.Unmarshal(data, &s.manifest)
}

func (s *TrashStore) save() error {
	data, err := json.MarshalIndent(s.manifest, "", "  ")
	if err != nil {
		return err
	}
	tmpPath := filepath.Join(s.trashDir, "manifest.json.tmp")
	if err := os.WriteFile(tmpPath, data, 0600); err != nil {
		return err
	}
	return os.Rename(tmpPath, filepath.Join(s.trashDir, "manifest.json"))
}

// MoveToTrash moves a file/folder to the trash directory
func (s *TrashStore) MoveToTrash(absPath, originalPath, username string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	info, err := os.Stat(absPath)
	if err != nil {
		return err
	}

	// A random suffix guarantees a unique id/trashPath even when two same-named
	// files are deleted within the same millisecond (otherwise the second move
	// would clobber the first's trashed bytes).
	id := fmt.Sprintf("%d_%s_%s", time.Now().UnixMilli(), randHex(4), filepath.Base(originalPath))
	trashPath := filepath.Join(s.trashDir, id)

	if err := os.Rename(absPath, trashPath); err != nil {
		return fmt.Errorf("failed to move to trash: %w", err)
	}

	var size int64
	if info.IsDir() {
		filepath.Walk(trashPath, func(_ string, fi os.FileInfo, _ error) error {
			if fi != nil && !fi.IsDir() {
				size += fi.Size()
			}
			return nil
		})
	} else {
		size = info.Size()
	}

	s.manifest = append(s.manifest, TrashItem{
		ID:           id,
		OriginalPath: originalPath,
		Name:         filepath.Base(originalPath),
		IsDir:        info.IsDir(),
		Size:         size,
		DeletedBy:    username,
		DeletedAt:    time.Now().UnixMilli(),
		TrashPath:    trashPath,
	})

	return s.save()
}

// List returns all trashed items, optionally filtered by username
func (s *TrashStore) List(username, role string) []TrashItem {
	s.mu.Lock()
	defer s.mu.Unlock()

	if role == "admin" {
		// Return a copy: the caller (handler) JSON-encodes after our lock is
		// released, and a concurrent mutator reslices s.manifest in place.
		out := make([]TrashItem, len(s.manifest))
		copy(out, s.manifest)
		return out
	}

	var items []TrashItem
	for _, item := range s.manifest {
		if item.DeletedBy == username {
			items = append(items, item)
		}
	}
	return items
}

// Restore moves a trashed item back to its original location
func (s *TrashStore) Restore(id, username, role string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, item := range s.manifest {
		if item.ID == id {
			if role != "admin" && item.DeletedBy != username {
				return fmt.Errorf("permission denied")
			}

			// Re-validate the destination stays within the storage root. The
			// delete path validated OriginalPath, but the manifest is the only
			// input here and could be corrupt/hand-edited — don't restore out
			// of the root.
			destPath, err := s.resolveRestoreDest(item.OriginalPath)
			if err != nil {
				return err
			}

			if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
				return fmt.Errorf("failed to create restore directory: %w", err)
			}

			// Never silently overwrite a file created at the original location
			// since the delete — restore alongside it under a unique name.
			destPath = uniqueDest(destPath)
			if err := os.Rename(item.TrashPath, destPath); err != nil {
				return fmt.Errorf("failed to restore: %w", err)
			}

			s.manifest = append(s.manifest[:i], s.manifest[i+1:]...)
			return s.save()
		}
	}
	return fmt.Errorf("item not found in trash")
}

// PermanentDelete removes an item from trash permanently
func (s *TrashStore) PermanentDelete(id, username, role string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, item := range s.manifest {
		if item.ID == id {
			if role != "admin" && item.DeletedBy != username {
				return fmt.Errorf("permission denied")
			}

			os.RemoveAll(item.TrashPath)
			s.manifest = append(s.manifest[:i], s.manifest[i+1:]...)
			return s.save()
		}
	}
	return fmt.Errorf("item not found in trash")
}

// EmptyTrash removes all items (admin only or user's own items)
func (s *TrashStore) EmptyTrash(username, role string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var remaining []TrashItem
	for _, item := range s.manifest {
		if role == "admin" || item.DeletedBy == username {
			os.RemoveAll(item.TrashPath)
		} else {
			remaining = append(remaining, item)
		}
	}

	s.manifest = remaining
	if s.manifest == nil {
		s.manifest = make([]TrashItem, 0)
	}
	return s.save()
}

// randHex returns n random bytes as a hex string, or a zero fallback if the
// system RNG fails (the timestamp prefix still keeps ids mostly unique).
func randHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return strings.Repeat("0", n*2)
	}
	return hex.EncodeToString(b)
}

// resolveRestoreDest returns the absolute restore destination for originalPath,
// rejecting any path that escapes the storage root.
func (s *TrashStore) resolveRestoreDest(originalPath string) (string, error) {
	dest := filepath.Join(s.root, filepath.Clean(originalPath))
	abs, err := filepath.Abs(dest)
	if err != nil {
		return "", fmt.Errorf("invalid restore path")
	}
	rootAbs, err := filepath.Abs(s.root)
	if err != nil {
		return "", fmt.Errorf("invalid root")
	}
	if abs != rootAbs && !strings.HasPrefix(abs+string(filepath.Separator), rootAbs+string(filepath.Separator)) {
		return "", fmt.Errorf("restore path escapes storage root")
	}
	return abs, nil
}

// uniqueDest returns dest unchanged if nothing exists there, otherwise inserts
// " (restored)" (with a counter if needed) before the extension so a restore
// never overwrites a file created since the delete.
func uniqueDest(dest string) string {
	if _, err := os.Stat(dest); os.IsNotExist(err) {
		return dest
	}
	dir := filepath.Dir(dest)
	base := filepath.Base(dest)
	ext := filepath.Ext(base)
	stem := strings.TrimSuffix(base, ext)
	for i := 0; ; i++ {
		suffix := " (restored)"
		if i > 0 {
			suffix = fmt.Sprintf(" (restored %d)", i+1)
		}
		candidate := filepath.Join(dir, stem+suffix+ext)
		if _, err := os.Stat(candidate); os.IsNotExist(err) {
			return candidate
		}
	}
}

// CleanExpired removes items older than 30 days
func (s *TrashStore) CleanExpired() {
	s.mu.Lock()
	defer s.mu.Unlock()

	cutoff := time.Now().Add(-30 * 24 * time.Hour).UnixMilli()
	var remaining []TrashItem
	for _, item := range s.manifest {
		if item.DeletedAt < cutoff {
			os.RemoveAll(item.TrashPath)
		} else {
			remaining = append(remaining, item)
		}
	}

	if len(remaining) != len(s.manifest) {
		s.manifest = remaining
		if s.manifest == nil {
			s.manifest = make([]TrashItem, 0)
		}
		s.save()
	}
}
