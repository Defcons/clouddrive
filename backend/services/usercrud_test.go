package services

import (
	"clouddrive/models"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func storeWith(t *testing.T, users ...models.User) *UserStore {
	t.Helper()
	data, _ := json.MarshalIndent(models.UsersConfig{Users: users}, "", "  ")
	path := filepath.Join(t.TempDir(), "users.json")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	s, err := NewUserStore(path)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestCreateUser(t *testing.T) {
	s := storeWith(t, models.User{Username: "admin", Role: "admin", HomeFolder: "/"})

	if err := s.CreateUser("alice", "longenough", "/Alice", "user", 1000); err != nil {
		t.Fatalf("create: %v", err)
	}
	if got := s.GetQuota("alice"); got != 1000 {
		t.Errorf("quota = %d, want 1000", got)
	}
	// Duplicate, weak password, bad role all rejected.
	if err := s.CreateUser("alice", "longenough", "/Alice", "user", 0); err == nil {
		t.Error("expected duplicate rejection")
	}
	if err := s.CreateUser("bob", "short", "/Bob", "user", 0); err == nil {
		t.Error("expected weak-password rejection")
	}
	if err := s.CreateUser("bob", "longenough", "/Bob", "superuser", 0); err == nil {
		t.Error("expected bad-role rejection")
	}
	// Created user can authenticate.
	if _, err := s.Authenticate("alice", "longenough"); err != nil {
		t.Errorf("created user can't log in: %v", err)
	}
}

func TestUpdateUserBumpsPwVersionOnPasswordChange(t *testing.T) {
	s := storeWith(t,
		models.User{Username: "admin", Role: "admin", HomeFolder: "/"},
		models.User{Username: "alice", Role: "user", HomeFolder: "/Alice"},
	)
	before := s.GetPwVersion("alice")
	if err := s.UpdateUser("alice", "/Alice2", "user", 500, "newpassword1"); err != nil {
		t.Fatalf("update: %v", err)
	}
	if s.GetPwVersion("alice") != before+1 {
		t.Error("password change should bump PwVersion")
	}
	if _, err := s.Authenticate("alice", "newpassword1"); err != nil {
		t.Errorf("new password should work: %v", err)
	}
}

func TestLastAdminProtection(t *testing.T) {
	s := storeWith(t,
		models.User{Username: "admin", Role: "admin", HomeFolder: "/"},
		models.User{Username: "alice", Role: "user", HomeFolder: "/Alice"},
	)
	// Can't demote or delete the only admin.
	if err := s.UpdateUser("admin", "/", "user", 0, ""); err == nil {
		t.Error("expected last-admin demote to be refused")
	}
	if err := s.DeleteUser("admin"); err == nil {
		t.Error("expected last-admin delete to be refused")
	}
	// With a second admin, demotion is allowed.
	if err := s.CreateUser("admin2", "longenough", "/", "admin", 0); err != nil {
		t.Fatal(err)
	}
	if err := s.UpdateUser("admin", "/", "user", 0, ""); err != nil {
		t.Errorf("demote with a second admin present should work: %v", err)
	}
}

func TestDeleteUser(t *testing.T) {
	s := storeWith(t,
		models.User{Username: "admin", Role: "admin", HomeFolder: "/"},
		models.User{Username: "alice", Role: "user", HomeFolder: "/Alice"},
	)
	if err := s.DeleteUser("alice"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if len(s.ListUsers()) != 1 {
		t.Errorf("expected 1 user after delete, got %d", len(s.ListUsers()))
	}
	if err := s.DeleteUser("ghost"); err == nil {
		t.Error("expected not-found on deleting missing user")
	}
}
