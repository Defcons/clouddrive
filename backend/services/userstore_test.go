package services

import (
	"clouddrive/models"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func newTestStore(t *testing.T, password string) *UserStore {
	t.Helper()
	hash, err := HashPassword(password)
	if err != nil {
		t.Fatal(err)
	}
	cfg := models.UsersConfig{Users: []models.User{
		{Username: "martin", Password: hash, HomeFolder: "/", Role: "admin"},
	}}
	data, _ := json.MarshalIndent(cfg, "", "  ")
	path := filepath.Join(t.TempDir(), "users.json")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	store, err := NewUserStore(path)
	if err != nil {
		t.Fatal(err)
	}
	return store
}

func TestAuthenticate(t *testing.T) {
	store := newTestStore(t, "correct horse battery")

	t.Run("correct credentials succeed", func(t *testing.T) {
		u, err := store.Authenticate("martin", "correct horse battery")
		if err != nil || u == nil || u.Username != "martin" {
			t.Fatalf("expected success, got user=%v err=%v", u, err)
		}
	})

	t.Run("wrong password fails", func(t *testing.T) {
		if _, err := store.Authenticate("martin", "wrong"); err == nil {
			t.Fatal("expected failure for wrong password")
		}
	})

	// The timing-equalization dummy-hash path must not accidentally authenticate
	// an unknown user.
	t.Run("unknown user fails", func(t *testing.T) {
		if _, err := store.Authenticate("ghost", "anything"); err == nil {
			t.Fatal("expected failure for unknown user")
		}
	})

	t.Run("returned user is a snapshot copy", func(t *testing.T) {
		u, _ := store.Authenticate("martin", "correct horse battery")
		u.Role = "tampered"
		again, _ := store.Authenticate("martin", "correct horse battery")
		if again.Role != "admin" {
			t.Fatalf("mutating returned user leaked into the store: role=%q", again.Role)
		}
	})
}
