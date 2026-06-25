package services

import (
	"path/filepath"
	"testing"
)

func TestNotificationsCappedPerUser(t *testing.T) {
	store := NewNotificationStore(t.TempDir())

	// Add more than the cap for martin, plus a few for nika.
	for i := 0; i < maxNotificationsPerUser+50; i++ {
		if err := store.Add("martin", "msg", "/"); err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < 5; i++ {
		if err := store.Add("nika", "msg", "/"); err != nil {
			t.Fatal(err)
		}
	}

	martin := countFor(store, "martin")
	if martin != maxNotificationsPerUser {
		t.Errorf("martin notifications = %d, want capped at %d", martin, maxNotificationsPerUser)
	}
	// Other users are unaffected by martin's cap.
	if nika := countFor(store, "nika"); nika != 5 {
		t.Errorf("nika notifications = %d, want 5", nika)
	}
}

func TestNotificationsKeepMostRecent(t *testing.T) {
	store := NewNotificationStore(t.TempDir())

	for i := 0; i < maxNotificationsPerUser; i++ {
		_ = store.Add("martin", "old", "/")
	}
	_ = store.Add("martin", "newest", "/")

	all := store.GetAll("martin", maxNotificationsPerUser+10)
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
		_ = store.Add("martin", "msg", "/")
	}
	for _, n := range store.GetAll("martin", 100) {
		if seen[n.ID] {
			t.Fatalf("duplicate notification id: %s", n.ID)
		}
		seen[n.ID] = true
	}
}

func TestNotificationsPersist(t *testing.T) {
	dir := t.TempDir()
	store := NewNotificationStore(dir)
	_ = store.Add("martin", "hello", "/x")

	// A fresh store over the same dir must load what was saved.
	reloaded := NewNotificationStore(dir)
	if got := countFor(reloaded, "martin"); got != 1 {
		t.Errorf("reloaded notifications = %d, want 1; file %s", got, filepath.Join(dir, ".notifications.json"))
	}
}

func countFor(s *NotificationStore, username string) int {
	return len(s.GetAll(username, 1<<30))
}
