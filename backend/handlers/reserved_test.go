package handlers

import "testing"

func TestIsReservedRel(t *testing.T) {
	reserved := []string{
		"/users.json", "users.json",
		"/.jwt_secret", "/.sessions.json", "/.audit.log",
		"/.permissions.json", "/.tags.json", "/.notifications.json", "/.backup-tiers.json",
		"/.thumbs/a.png", "/.uploads/x", "/.trash/y", "/.versions/h/1.bin",
	}
	for _, p := range reserved {
		if !isReservedRel(p) {
			t.Errorf("isReservedRel(%q) = false, want true", p)
		}
	}
	allowed := []string{
		"/", "", "/Documents", "/Photos/a.jpg", "/notes.md", "/Documents/report.json",
		// users.json is only reserved at the ROOT — a user's own file in a
		// subfolder must still work.
		"/Documents/users.json",
	}
	for _, p := range allowed {
		if isReservedRel(p) {
			t.Errorf("isReservedRel(%q) = true, want false", p)
		}
	}
}

func TestSafePathDeniesReserved(t *testing.T) {
	h := &FileHandler{root: t.TempDir()}
	for _, p := range []string{"/users.json", "/.jwt_secret", "/.sessions.json", "/.thumbs/x"} {
		if _, err := h.safePath(p); err == nil {
			t.Errorf("safePath(%q) should be denied", p)
		}
	}
	// Legitimate paths still resolve.
	for _, p := range []string{"/", "/Documents", "/notes.md", "/Documents/users.json"} {
		if _, err := h.safePath(p); err != nil {
			t.Errorf("safePath(%q) should be allowed, got %v", p, err)
		}
	}
}
