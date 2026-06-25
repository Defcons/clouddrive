package services

import (
	"path/filepath"
	"testing"
)

func TestNotificationsCappedPerUser(t *testing.T) {
	store := NewNotificationStore(t.TempDir())

	// Add more than the cap for bob, plus a few for alice.
	for i := 0; i < maxNotificationsPerUser+50; i++ {
		if err := store.Add("bob", "msg", "/"); err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < 5; i++ {
		if err := store.Add("alice", "msg", "/"); err != nil {
			t.Fatal(err)
		}
	}

	bob := countFor(store, "bob")
	if bob != maxNotificationsPerUser {
		t.Errorf("bob notifications = %d, want capped at %d", bob, maxNotificationsPerUser)
	}
	// Other users are unaffected by bob's cap.
	if alice := countFor(store, "alice"); alice != 5 {
		t.Errorf("alice notifications = %d, want 5", alice)
	}
}

func TestNotificationsKeepMostRecent(t *testing.T) {
	store := NewNotificationStore(t.TempDir())

	for i := 0; i < maxNotificationsPerUser; i++ {
		_ = store.Add("bob", "old", "/")
	}
	_ = store.Add("bob", "newest", "/")

	all := store.GetAll("bob", maxNotificationsPerUser+10)
	if len(all) != maxNotificationsPerUser {
		t.Fatalf("want %d after overflow, got %d", maxNotificationsPerUser, len(all))
	}
	// GetAll returns newest-first; the most recently added must survive the trim.
	if all[0].Message != "newest" {
		t.Errorf("newest notification was trimmed; head = %q", all[0].Message)
	}
}

func TestNotificationIDsUnique(t *testing.T) {
	store := NewNotificationStore(t.TempDir())
	seen := map[string]bool{}
	for i := 0; i < 50; i++ {
		_ = store.Add("bob", "msg", "/")
	}
	for _, n := range store.GetAll("bob", 100) {
		if seen[n.ID] {
			t.Fatalf("duplicate notification id: %s", n.ID)
		}
		seen[n.ID] = true
	}
}

func TestNotificationsPersist(t *testing.T) {
	dir := t.TempDir()
	store := NewNotificationStore(dir)
	_ = store.Add("bob", "hello", "/x")

	// A fresh store over the same dir must load what was saved.
	reloaded := NewNotificationStore(dir)
	if got := countFor(reloaded, "bob"); got != 1 {
		t.Errorf("reloaded notifications = %d, want 1; file %s", got, filepath.Join(dir, ".notifications.json"))
	}
}

func countFor(s *NotificationStore, username string) int {
	return len(s.GetAll(username, 1<<30))
}
