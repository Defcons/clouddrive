package services

import (
	"errors"
	"path/filepath"
	"testing"
)

func TestNewUserStoreMissingFileStartsEmpty(t *testing.T) {
	path := filepath.Join(t.TempDir(), "users.json") // deliberately not created
	s, err := NewUserStore(path)
	if err != nil {
		t.Fatalf("NewUserStore on a missing file should succeed (fresh install), got: %v", err)
	}
	if s.Count() != 0 {
		t.Errorf("fresh store should have 0 users, got %d", s.Count())
	}
}

func TestCreateFirstAdmin(t *testing.T) {
	path := filepath.Join(t.TempDir(), "users.json")
	s, err := NewUserStore(path)
	if err != nil {
		t.Fatal(err)
	}

	// Weak/empty input is rejected and must NOT consume the one-shot setup.
	if _, err := s.CreateFirstAdmin("admin", "short"); err == nil {
		t.Error("expected weak-password rejection")
	}
	if _, err := s.CreateFirstAdmin("", "longenough"); err == nil {
		t.Error("expected empty-username rejection")
	}
	if s.Count() != 0 {
		t.Fatalf("rejected setup attempts must not create a user, got %d", s.Count())
	}

	u, err := s.CreateFirstAdmin("admin", "longenough")
	if err != nil {
		t.Fatalf("first admin: %v", err)
	}
	if u.Role != "admin" || u.HomeFolder != "/" {
		t.Errorf("first admin should be admin at /, got role=%q home=%q", u.Role, u.HomeFolder)
	}
	if s.Count() != 1 {
		t.Errorf("expected 1 user, got %d", s.Count())
	}
	if _, err := s.Authenticate("admin", "longenough"); err != nil {
		t.Errorf("first admin should be able to log in: %v", err)
	}

	// Setup is one-shot: a second call is refused with ErrSetupComplete.
	if _, err := s.CreateFirstAdmin("intruder", "longenough"); !errors.Is(err, ErrSetupComplete) {
		t.Errorf("second CreateFirstAdmin should return ErrSetupComplete, got %v", err)
	}
	if s.Count() != 1 {
		t.Errorf("refused setup must not add a user, got %d", s.Count())
	}
}

func TestCreateFirstAdminPersists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "users.json")
	s, err := NewUserStore(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateFirstAdmin("admin", "longenough"); err != nil {
		t.Fatal(err)
	}
	// A brand-new store reading the same file sees the persisted admin — and so
	// reports setup as already complete.
	s2, err := NewUserStore(path)
	if err != nil {
		t.Fatal(err)
	}
	if s2.Count() != 1 {
		t.Errorf("persisted admin should survive reload, got %d users", s2.Count())
	}
	if _, err := s2.CreateFirstAdmin("x", "longenough"); !errors.Is(err, ErrSetupComplete) {
		t.Errorf("reloaded store should treat setup as complete, got %v", err)
	}
}
