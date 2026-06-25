package services

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAuditLogRoundTrip(t *testing.T) {
	root := t.TempDir()
	a := NewAuditLogger(root)
	defer a.Close()

	a.Log("LOGIN_OK", "martin", "1.2.3.4", "ok")
	a.Log("DELETE", "martin", "1.2.3.4", "removed /x")

	recent := a.GetRecent(10)
	if len(recent) != 2 {
		t.Fatalf("want 2 entries, got %d", len(recent))
	}
	// Newest first.
	if recent[0].Action != "DELETE" {
		t.Errorf("want newest entry DELETE first, got %q", recent[0].Action)
	}
}

func TestAuditLogDegradesSafely(t *testing.T) {
	// Point the storage root at a regular file so opening <root>/.audit.log
	// fails; the logger must degrade to a safe no-op rather than panic.
	f := filepath.Join(t.TempDir(), "not-a-dir")
	if err := os.WriteFile(f, []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	a := NewAuditLogger(f)
	defer a.Close()

	if a.file != nil {
		t.Fatal("expected nil file handle when audit log can't be opened")
	}
	// These must not panic.
	a.Log("LOGIN_OK", "martin", "1.2.3.4", "ok")
	if got := a.GetRecent(10); got != nil {
		t.Errorf("expected nil entries from a disabled logger, got %v", got)
	}
}
