package services

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// BackupTier values:
//   0 = no special backup handling (ZFS snapshots only, default)
//   2 = included in offsite (Hetzner restic)
type BackupTierStore struct {
	filePath string
	tiers    map[string]int // path -> tier
	mu       sync.RWMutex
}

func NewBackupTierStore(storageRoot string) *BackupTierStore {
	store := &BackupTierStore{
		filePath: filepath.Join(storageRoot, ".backup-tiers.json"),
		tiers:    make(map[string]int),
	}
	store.load()
	return store
}

func (s *BackupTierStore) load() {
	loadJSONFile(s.filePath, &s.tiers)
}

func (s *BackupTierStore) save() error {
	data, err := json.MarshalIndent(s.tiers, "", "  ")
	if err != nil {
		return err
	}
	tmpPath := s.filePath + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0600); err != nil {
		return err
	}
	return os.Rename(tmpPath, s.filePath)
}

// GetTier returns the configured tier for a path, walking up parents.
// Returns 0 if no tier is set anywhere in the ancestry.
func (s *BackupTierStore) GetTier(path string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	path = filepath.ToSlash(filepath.Clean(path))
	current := path
	for {
		if tier, ok := s.tiers[current]; ok {
			return tier
		}
		parent := filepath.ToSlash(filepath.Dir(current))
		if parent == current || parent == "." {
			break
		}
		current = parent
	}
	return 0
}

// GetTierExact returns only the explicit tier set on this exact path (no inheritance)
func (s *BackupTierStore) GetTierExact(path string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.tiers[filepath.ToSlash(filepath.Clean(path))]
}

// SetTier assigns a backup tier to a path. Tier 0 removes the entry.
func (s *BackupTierStore) SetTier(path string, tier int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	path = filepath.ToSlash(filepath.Clean(path))
	if tier == 0 {
		delete(s.tiers, path)
	} else {
		s.tiers[path] = tier
	}
	return s.save()
}

// MovePath migrates the backup tier for path (and any descendants) to newPath
// so a renamed/moved folder keeps its tier.
func (s *BackupTierStore) MovePath(oldPath, newPath string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if movePathKeys(s.tiers, oldPath, newPath) {
		return s.save()
	}
	return nil
}

// All returns a copy of the tiers map
func (s *BackupTierStore) All() map[string]int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[string]int, len(s.tiers))
	for k, v := range s.tiers {
		out[k] = v
	}
	return out
}
