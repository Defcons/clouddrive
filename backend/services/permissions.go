package services

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type FolderPermission struct {
	Owner        string   `json:"owner"`
	AllowedUsers []string `json:"allowedUsers"`
}

type PermissionStore struct {
	configPath  string
	permissions map[string]*FolderPermission
	mu          sync.RWMutex
}

func NewPermissionStore(storageRoot string) *PermissionStore {
	configPath := filepath.Join(storageRoot, ".permissions.json")
	store := &PermissionStore{
		configPath:  configPath,
		permissions: make(map[string]*FolderPermission),
	}
	store.load()
	return store
}

func (s *PermissionStore) load() {
	data, err := os.ReadFile(s.configPath)
	if err != nil {
		return // file doesn't exist yet, that's fine
	}
	json.Unmarshal(data, &s.permissions)
}

func (s *PermissionStore) save() error {
	data, err := json.MarshalIndent(s.permissions, "", "  ")
	if err != nil {
		return err
	}
	tmpPath := s.configPath + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0600); err != nil {
		return err
	}
	return os.Rename(tmpPath, s.configPath)
}

// CanAccess checks if a user can access a given path.
// Admins always have access. Walks up from the path to root checking ancestors.
func (s *PermissionStore) CanAccess(folderPath, username, role string) bool {
	if role == "admin" {
		return true
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	// Normalize path
	folderPath = filepath.ToSlash(filepath.Clean(folderPath))

	// Check the path and all its ancestors
	current := folderPath
	for {
		if perm, exists := s.permissions[current]; exists {
			for _, u := range perm.AllowedUsers {
				if u == username {
					return true
				}
			}
			return false // path is restricted and user is not in allowed list
		}

		// Move to parent
		parent := filepath.ToSlash(filepath.Dir(current))
		if parent == current || parent == "." {
			break
		}
		current = parent
	}

	return true // no restrictions found
}

// IsPrivate checks if a specific path has a permission entry
func (s *PermissionStore) IsPrivate(folderPath string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	folderPath = filepath.ToSlash(filepath.Clean(folderPath))
	_, exists := s.permissions[folderPath]
	return exists
}

// GetPermission returns the permission entry for a path, or nil
func (s *PermissionStore) GetPermission(folderPath string) *FolderPermission {
	s.mu.RLock()
	defer s.mu.RUnlock()

	folderPath = filepath.ToSlash(filepath.Clean(folderPath))
	return s.permissions[folderPath]
}

// SetPrivate marks a folder as private to specific users
func (s *PermissionStore) SetPrivate(folderPath, owner string, allowedUsers []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	folderPath = filepath.ToSlash(filepath.Clean(folderPath))

	// Ensure owner is in allowed users
	hasOwner := false
	for _, u := range allowedUsers {
		if u == owner {
			hasOwner = true
			break
		}
	}
	if !hasOwner {
		allowedUsers = append([]string{owner}, allowedUsers...)
	}

	s.permissions[folderPath] = &FolderPermission{
		Owner:        owner,
		AllowedUsers: allowedUsers,
	}

	return s.save()
}

// RemovePrivate removes the permission restriction on a folder
func (s *PermissionStore) RemovePrivate(folderPath string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	folderPath = filepath.ToSlash(filepath.Clean(folderPath))
	delete(s.permissions, folderPath)

	return s.save()
}

// FilterPaths filters a list of paths to only those the user can access
func (s *PermissionStore) FilterPaths(paths []string, username, role string) []string {
	if role == "admin" {
		return paths
	}

	filtered := make([]string, 0, len(paths))
	for _, p := range paths {
		if s.CanAccess(p, username, role) {
			filtered = append(filtered, p)
		}
	}
	return filtered
}

// IsAncestorPrivate checks if any ancestor of a path is private and the user doesn't have access
func (s *PermissionStore) IsAncestorPrivate(folderPath, username, role string) bool {
	if role == "admin" {
		return false
	}
	return !s.CanAccess(folderPath, username, role)
}

// ListPrivatePaths returns all paths that have permission entries (for checking subpaths)
func (s *PermissionStore) ListPrivatePaths() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	paths := make([]string, 0, len(s.permissions))
	for p := range s.permissions {
		paths = append(paths, p)
	}
	return paths
}

// HasPrefix checks if path starts with the given prefix (for subpath matching)
func HasPathPrefix(path, prefix string) bool {
	path = filepath.ToSlash(filepath.Clean(path))
	prefix = filepath.ToSlash(filepath.Clean(prefix))
	return path == prefix || strings.HasPrefix(path, prefix+"/")
}
