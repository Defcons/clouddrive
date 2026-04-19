package services

import (
	"clouddrive/models"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sync"

	"golang.org/x/crypto/bcrypt"
)

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

func (s *UserStore) Authenticate(username, password string) (*models.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, u := range s.users {
		if u.Username == username {
			if err := bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(password)); err != nil {
				return nil, fmt.Errorf("invalid credentials")
			}
			return &u, nil
		}
	}
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

// HashPassword is a utility for generating bcrypt hashes
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}
