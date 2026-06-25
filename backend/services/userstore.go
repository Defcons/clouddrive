package services

import (
	"clouddrive/models"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"

	"golang.org/x/crypto/bcrypt"
)

// AdminUserInfo is the admin-facing view of a user (no secrets).
type AdminUserInfo struct {
	Username   string `json:"username"`
	HomeFolder string `json:"homeFolder"`
	Role       string `json:"role"`
	Quota      int64  `json:"quota"`
	MfaEnabled bool   `json:"mfaEnabled"`
}

type UserStore struct {
	configPath string
	users      []models.User
	mu         sync.RWMutex
}

func NewUserStore(configPath string) (*UserStore, error) {
	store := &UserStore{configPath: configPath}
	if err := store.load(); err != nil {
		return nil, err
	}
	return store, nil
}

// InitFromEnv creates a users.json from legacy env vars if no config exists
func InitFromEnv(configPath, username, password string) error {
	if _, err := os.Stat(configPath); err == nil {
		return nil // file exists, no migration needed
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	config := models.UsersConfig{
		Users: []models.User{
			{
				Username:   username,
				Password:   string(hash),
				HomeFolder: "/",
				Role:       "admin",
			},
		},
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	log.Printf("Created users.json from env vars (user: %s)", username)
	return nil
}

func (s *UserStore) load() error {
	data, err := os.ReadFile(s.configPath)
	if err != nil {
		return fmt.Errorf("failed to read users config: %w", err)
	}

	var config models.UsersConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("failed to parse users config: %w", err)
	}

	s.mu.Lock()
	s.users = config.Users
	s.mu.Unlock()

	log.Printf("Loaded %d user(s) from %s", len(config.Users), s.configPath)
	return nil
}

func (s *UserStore) save() error {
	s.mu.RLock()
	config := models.UsersConfig{Users: s.users}
	s.mu.RUnlock()

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	tmpPath := s.configPath + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0600); err != nil {
		return err
	}
	return os.Rename(tmpPath, s.configPath)
}

// dummyHash is a valid bcrypt hash (at DefaultCost) of a random string. It's
// compared against when the username doesn't exist so the login response time
// matches the user-exists path, preventing username enumeration via timing.
var dummyHash = func() []byte {
	h, _ := bcrypt.GenerateFromPassword([]byte("nonexistent-user-timing-equalizer"), bcrypt.DefaultCost)
	return h
}()

func (s *UserStore) Authenticate(username, password string) (*models.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, u := range s.users {
		if u.Username == username {
			if err := bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(password)); err != nil {
				return nil, fmt.Errorf("invalid credentials")
			}
			user := u
			return &user, nil
		}
	}
	// Unknown user: still run a bcrypt comparison so timing doesn't reveal
	// whether the username exists.
	_ = bcrypt.CompareHashAndPassword(dummyHash, []byte(password))
	return nil, fmt.Errorf("invalid credentials")
}

func (s *UserStore) GetUser(username string) *models.User {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, u := range s.users {
		if u.Username == username {
			return &u
		}
	}
	return nil
}

func (s *UserStore) ChangePassword(username, currentPassword, newPassword string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, u := range s.users {
		if u.Username == username {
			if err := bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(currentPassword)); err != nil {
				return fmt.Errorf("current password is incorrect")
			}
			hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
			if err != nil {
				return fmt.Errorf("failed to hash password: %w", err)
			}
			s.users[i].Password = string(hash)
			s.users[i].PwVersion++
			// Save under the same lock to avoid a race where another goroutine
			// mutates s.users between the in-memory update and the disk write.
			return s.saveLocked()
		}
	}
	return fmt.Errorf("user not found")
}

// saveLocked writes users.json. Caller must hold s.mu (write-locked).
func (s *UserStore) saveLocked() error {
	config := models.UsersConfig{Users: s.users}
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	tmpPath := s.configPath + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0600); err != nil {
		return err
	}
	return os.Rename(tmpPath, s.configPath)
}

// GetQuota returns the user's storage quota in bytes (0 = unlimited / unknown).
func (s *UserStore) GetQuota(username string) int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, u := range s.users {
		if u.Username == username {
			return u.Quota
		}
	}
	return 0
}

func (s *UserStore) GetPwVersion(username string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, u := range s.users {
		if u.Username == username {
			return u.PwVersion
		}
	}
	return 0
}

// ---- Admin user management ----

func (s *UserStore) countAdminsLocked() int {
	n := 0
	for _, u := range s.users {
		if u.Role == "admin" {
			n++
		}
	}
	return n
}

// ListUsers returns the admin-facing view of all users (no secrets).
func (s *UserStore) ListUsers() []AdminUserInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]AdminUserInfo, 0, len(s.users))
	for _, u := range s.users {
		out = append(out, AdminUserInfo{
			Username:   u.Username,
			HomeFolder: u.HomeFolder,
			Role:       u.Role,
			Quota:      u.Quota,
			MfaEnabled: u.MfaEnabled,
		})
	}
	return out
}

// CreateUser adds a new user. Errors on duplicate, weak password, or bad role.
func (s *UserStore) CreateUser(username, password, homeFolder, role string, quota int64) error {
	username = strings.TrimSpace(username)
	if username == "" {
		return fmt.Errorf("username is required")
	}
	if len(password) < 8 {
		return fmt.Errorf("password must be at least 8 characters")
	}
	if role != "admin" && role != "user" {
		return fmt.Errorf("role must be 'admin' or 'user'")
	}
	if homeFolder == "" {
		homeFolder = "/"
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	for _, u := range s.users {
		if u.Username == username {
			return fmt.Errorf("user already exists")
		}
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	s.users = append(s.users, models.User{
		Username:   username,
		Password:   string(hash),
		HomeFolder: homeFolder,
		Role:       role,
		Quota:      quota,
	})
	return s.saveLocked()
}

// UpdateUser changes a user's home folder, role, quota, and (optionally)
// password. An empty newPassword leaves the password unchanged. Refuses to
// demote the last admin. A password change bumps PwVersion (invalidating
// existing sessions).
func (s *UserStore) UpdateUser(username, homeFolder, role string, quota int64, newPassword string) error {
	if role != "admin" && role != "user" {
		return fmt.Errorf("role must be 'admin' or 'user'")
	}
	if newPassword != "" && len(newPassword) < 8 {
		return fmt.Errorf("password must be at least 8 characters")
	}
	if homeFolder == "" {
		homeFolder = "/"
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	idx := -1
	for i, u := range s.users {
		if u.Username == username {
			idx = i
			break
		}
	}
	if idx == -1 {
		return fmt.Errorf("user not found")
	}
	if s.users[idx].Role == "admin" && role != "admin" && s.countAdminsLocked() <= 1 {
		return fmt.Errorf("cannot demote the last admin")
	}

	s.users[idx].HomeFolder = homeFolder
	s.users[idx].Role = role
	s.users[idx].Quota = quota
	if newPassword != "" {
		hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
		if err != nil {
			return err
		}
		s.users[idx].Password = string(hash)
		s.users[idx].PwVersion++
	}
	return s.saveLocked()
}

// DeleteUser removes a user. Refuses to delete the last admin.
func (s *UserStore) DeleteUser(username string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	idx := -1
	for i, u := range s.users {
		if u.Username == username {
			idx = i
			break
		}
	}
	if idx == -1 {
		return fmt.Errorf("user not found")
	}
	if s.users[idx].Role == "admin" && s.countAdminsLocked() <= 1 {
		return fmt.Errorf("cannot delete the last admin")
	}
	s.users = append(s.users[:idx], s.users[idx+1:]...)
	return s.saveLocked()
}

// HashPassword is a utility for generating bcrypt hashes
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}
