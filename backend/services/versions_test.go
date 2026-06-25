package services

import (
	"os"
	"path/filepath"
	"testing"
)

func TestVersioningSaveListRestore(t *testing.T) {
	root := t.TempDir()
	vs := NewVersionStore(root)

	file := filepath.Join(root, "doc.txt")
	if err := os.WriteFile(file, []byte("v1"), 0644); err != nil {
		t.Fatal(err)
	}

	// Snapshot v1, then overwrite with v2.
	if err := vs.SaveVersion(file, "/doc.txt"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte("v2-current"), 0644); err != nil {
		t.Fatal(err)
	}

	versions := vs.ListVersions("/doc.txt")
	if len(versions) != 1 {
		t.Fatalf("want 1 version, got %d", len(versions))
	}

	// Restore v1 over the current file; current (v2) is itself snapshotted.
	if err := vs.RestoreVersion("/doc.txt", versions[0].ID, file); err != nil {
		t.Fatalf("restore: %v", err)
	}
	if b, _ := os.ReadFile(file); string(b) != "v1" {
		t.Errorf("restored content = %q, want %q", b, "v1")
	}
	if got := len(vs.ListVersions("/doc.txt")); got != 2 {
		t.Errorf("after reversible restore want 2 versions, got %d", got)
	}
}

func TestVersioningRetention(t *testing.T) {
	root := t.TempDir()
	vs := NewVersionStore(root)
	file := filepath.Join(root, "log.txt")
	for i := 0; i < maxVersionsPerFile+5; i++ {
		if err := os.WriteFile(file, []byte{byte(i)}, 0644); err != nil {
			t.Fatal(err)
		}
		if err := vs.SaveVersion(file, "/log.txt"); err != nil {
			t.Fatal(err)
		}
	}
	if got := len(vs.ListVersions("/log.txt")); got != maxVersionsPerFile {
		t.Errorf("retention: want %d versions, got %d", maxVersionsPerFile, got)
	}
}

func TestVersioningRejectsBadID(t *testing.T) {
	root := t.TempDir()
	vs := NewVersionStore(root)
	file := filepath.Join(root, "x.txt")
	_ = os.WriteFile(file, []byte("a"), 0644)
	_ = vs.SaveVersion(file, "/x.txt")
	// A non-numeric id (path traversal attempt) must be rejected.
	if err := vs.RestoreVersion("/x.txt", "../../etc/passwd", file); err == nil {
		t.Error("expected invalid version id to be rejected")
	}
}
