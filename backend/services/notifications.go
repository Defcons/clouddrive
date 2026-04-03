package services

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Notification struct {
	ID        string `json:"id"`
	Username  string `json:"username"` // recipient
	Message   string `json:"message"`
	Link      string `json:"link"` // path to navigate to
	Read      bool   `json:"read"`
	CreatedAt int64  `json:"createdAt"`
}

type NotificationStore struct {
	filePath      string
	notifications []Notification
	mu            sync.Mutex
}

func NewNotificationStore(storageRoot string) *NotificationStore {
	store := &NotificationStore{
		filePath:      filepath.Join(storageRoot, ".notifications.json"),
		notifications: make([]Notification, 0),
	}
	store.load()
	return store
}

func (s *NotificationStore) load() {
	data, err := os.ReadFile(s.filePath)
	if err != nil {
		return
	}
	json.Unmarshal(data, &s.notifications)
}

func (s *NotificationStore) save() error {
	data, err := json.MarshalIndent(s.notifications, "", "  ")
	if err != nil {
		return err
	}
	tmpPath := s.filePath + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0600); err != nil {
		return err
	}
	return os.Rename(tmpPath, s.filePath)
}

func (s *NotificationStore) Add(username, message, link string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	id := time.Now().Format("20060102150405") + "_" + username
	s.notifications = append(s.notifications, Notification{
		ID:        id,
		Username:  username,
		Message:   message,
		Link:      link,
		Read:      false,
		CreatedAt: time.Now().UnixMilli(),
	})

	// Keep only last 100 per user
	return s.save()
}

func (s *NotificationStore) GetUnread(username string) []Notification {
	s.mu.Lock()
	defer s.mu.Unlock()

	var result []Notification
	for i := len(s.notifications) - 1; i >= 0; i-- {
		n := s.notifications[i]
		if n.Username == username && !n.Read {
			result = append(result, n)
		}
	}
	return result
}

func (s *NotificationStore) GetAll(username string, limit int) []Notification {
	s.mu.Lock()
	defer s.mu.Unlock()

	var result []Notification
	for i := len(s.notifications) - 1; i >= 0; i-- {
		n := s.notifications[i]
		if n.Username == username {
			result = append(result, n)
			if len(result) >= limit {
				break
			}
		}
	}
	return result
}

func (s *NotificationStore) MarkRead(username string, ids []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	idSet := make(map[string]bool)
	for _, id := range ids {
		idSet[id] = true
	}

	for i := range s.notifications {
		if s.notifications[i].Username == username && idSet[s.notifications[i].ID] {
			s.notifications[i].Read = true
		}
	}
	return s.save()
}

func (s *NotificationStore) MarkAllRead(username string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.notifications {
		if s.notifications[i].Username == username {
			s.notifications[i].Read = true
		}
	}
	return s.save()
}
