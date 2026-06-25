package services

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const maxVersionsPerFile = 10

// VersionInfo describes one stored previous version of a file.
type VersionInfo struct {
	ID      string `json:"id"`      // nanosecond timestamp string
	Size    int64  `json:"size"`
	SavedAt int64  `json:"savedAt"` // epoch millis
}

// VersionStore keeps previous copies of files (made on overwrite) under
// <root>/.versions/<sha256(path)>/<nanotime>.bin, retaining the most recent N.
type VersionStore struct {
	versionsDir string
	mu          sync.Mutex
}

func NewVersionStore(storageRoot string) *VersionStore {
	dir := filepath.Join(storageRoot, ".versions")
	os.MkdirAll(dir, 0755)
	return &VersionStore{versionsDir: dir}
}

func (s *VersionStore) keyDir(webPath string) string {
	sum := sha256.Sum256([]byte(filepath.ToSlash(filepath.Clean(webPath))))
	return filepath.Join(s.versionsDir, hex.EncodeToString(sum[:]))
}

func copyFileTo(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

// SaveVersion snapshots the current file at absPath as a version of webPath.
// No-op for directories or missing files.
func (s *VersionStore) SaveVersion(absPath, webPath string) error {
	info, err := os.Stat(absPath)
	if err != nil || info.IsDir() {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	dir := s.keyDir(webPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	id := strconv.FormatInt(time.Now().UnixNano(), 10)
	if err := copyFileTo(absPath, filepath.Join(dir, id+".bin")); err != nil {
		return err
	}
	s.pruneLocked(dir)
	return nil
}

// ListVersions returns stored versions of webPath, newest first.
func (s *VersionStore) ListVersions(webPath string) []VersionInfo {
	s.mu.Lock()
	defer s.mu.Unlock()
	entries, err := os.ReadDir(s.keyDir(webPath))
	if err != nil {
		return nil
	}
	var out []VersionInfo
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".bin") {
			continue
		}
		id := strings.TrimSuffix(e.Name(), ".bin")
		ns, perr := strconv.ParseInt(id, 10, 64)
		if perr != nil {
			continue
		}
		info, ierr := e.Info()
		if ierr != nil {
			continue
		}
		out = append(out, VersionInfo{ID: id, Size: info.Size(), SavedAt: ns / 1e6})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID > out[j].ID })
	return out
}

// versionPath validates id (numeric only — no path traversal) and returns the
// on-disk path for that version of webPath, or an error if it doesn't exist.
func (s *VersionStore) versionPath(webPath, id string) (string, error) {
	if _, err := strconv.ParseInt(id, 10, 64); err != nil {
		return "", fmt.Errorf("invalid version id")
	}
	p := filepath.Join(s.keyDir(webPath), id+".bin")
	if _, err := os.Stat(p); err != nil {
		return "", fmt.Errorf("version not found")
	}
	return p, nil
}

// OpenVersion returns a readable handle to a specific version (caller closes).
func (s *VersionStore) OpenVersion(webPath, id string) (*os.File, int64, int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, err := s.versionPath(webPath, id)
	if err != nil {
		return nil, 0, 0, err
	}
	info, err := os.Stat(p)
	if err != nil {
		return nil, 0, 0, err
	}
	f, err := os.Open(p)
	if err != nil {
		return nil, 0, 0, err
	}
	ns, _ := strconv.ParseInt(id, 10, 64)
	return f, info.Size(), ns, nil
}

// RestoreVersion writes the chosen version back to currentAbsPath. The current
// file is first snapshotted as a new version, so a restore is itself undoable.
func (s *VersionStore) RestoreVersion(webPath, id, currentAbsPath string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	src, err := s.versionPath(webPath, id)
	if err != nil {
		return err
	}
	dir := s.keyDir(webPath)
	if _, err := os.Stat(currentAbsPath); err == nil {
		newID := strconv.FormatInt(time.Now().UnixNano(), 10)
		if err := copyFileTo(currentAbsPath, filepath.Join(dir, newID+".bin")); err == nil {
			s.pruneLocked(dir)
		}
	}
	return copyFileTo(src, currentAbsPath)
}

// pruneLocked keeps only the newest maxVersionsPerFile .bin files in dir.
func (s *VersionStore) pruneLocked(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".bin") {
			names = append(names, e.Name())
		}
	}
	if len(names) <= maxVersionsPerFile {
		return
	}
	sort.Strings(names) // ascending by nanotime → oldest first
	for _, n := range names[:len(names)-maxVersionsPerFile] {
		_ = os.Remove(filepath.Join(dir, n))
	}
}
