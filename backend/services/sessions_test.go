package services

import "testing"

func TestSessionStoreLifecycle(t *testing.T) {
	s := NewSessionStore(t.TempDir())
	id := s.Create("nika", "Firefox", "1.2.3.4", 1000)

	if !s.IsValid(id) {
		t.Fatal("new session should be valid")
	}
	if s.IsValid("") || s.IsValid("nope") {
		t.Error("empty/unknown ids must be invalid")
	}
	if got := s.List("nika"); len(got) != 1 || got[0].ID != id {
		t.Fatalf("List(nika) = %v", got)
	}

	// Another non-admin user cannot revoke nika's session.
	if s.Revoke(id, "martin", "user") {
		t.Error("cross-user revoke should be denied")
	}
	if !s.IsValid(id) {
		t.Error("session should survive a denied revoke")
	}
	// Owner can revoke.
	if !s.Revoke(id, "nika", "user") || s.IsValid(id) {
		t.Error("owner revoke should succeed")
	}
}

func TestSessionRevokeAllAndAdmin(t *testing.T) {
	s := NewSessionStore(t.TempDir())
	a := s.Create("nika", "", "", 1)
	b := s.Create("nika", "", "", 2)
	other := s.Create("martin", "", "", 3)

	// Admin can revoke anyone's session.
	if !s.Revoke(a, "admin", "admin") {
		t.Error("admin revoke should succeed")
	}
	s.RevokeAllForUser("nika")
	if s.IsValid(a) || s.IsValid(b) {
		t.Error("RevokeAllForUser should clear nika's sessions")
	}
	if !s.IsValid(other) {
		t.Error("other user's session must be untouched")
	}
}

func TestSessionPersistence(t *testing.T) {
	dir := t.TempDir()
	s := NewSessionStore(dir)
	id := s.Create("nika", "", "", 1)
	// A fresh store over the same dir must load the saved session.
	if !NewSessionStore(dir).IsValid(id) {
		t.Error("session did not persist across reload")
	}
}
