package services

import (
	"path/filepath"
	"sync"
)

// Settings holds instance-wide configuration an admin can change at runtime,
// persisted to .settings.json at the storage root. Only non-sensitive,
// feature/UX-shaping options live here — security-critical config (JWT_SECRET,
// TRUSTED_PROXIES, ALLOWED_ORIGINS) stays in the environment.
type Settings struct {
	InstanceName         string `json:"instanceName"`
	OffsiteBackupEnabled bool   `json:"offsiteBackupEnabled"`
	SharingEnabled       bool   `json:"sharingEnabled"`
}

// defaultSettings is the config a fresh install starts with. Loading a stored
// file overlays it, so a file written by an older version keeps sane defaults
// for any field it doesn't contain.
func defaultSettings() Settings {
	return Settings{
		InstanceName:         "CloudDrive",
		OffsiteBackupEnabled: true,
		SharingEnabled:       true,
	}
}

type SettingsStore struct {
	filePath string
	settings Settings
	mu       sync.RWMutex
}

func NewSettingsStore(storageRoot string) *SettingsStore {
	s := &SettingsStore{
		filePath: filepath.Join(storageRoot, ".settings.json"),
		settings: defaultSettings(),
	}
	s.load()
	return s
}

// load overlays the stored file onto the defaults (unmarshal leaves absent
// fields at their default value).
func (s *SettingsStore) load() {
	loadJSONFile(s.filePath, &s.settings)
}

// Get returns a copy of the current settings.
func (s *SettingsStore) Get() Settings {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.settings
}

// Update replaces the settings and persists them atomically. An empty instance
// name falls back to the default rather than showing a blank title.
func (s *SettingsStore) Update(next Settings) error {
	if next.InstanceName == "" {
		next.InstanceName = "CloudDrive"
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.settings = next
	return saveJSONFile(s.filePath, s.settings)
}
