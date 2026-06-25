package services

import (
	"os"
	"path/filepath"
	"testing"
)

// writeFile creates a file with the given content, making parent dirs.
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestMoveToTrashAndRestore(t *testing.T) {
	root := t.TempDir()
	store := NewTrashStore(root)

	orig := filepath.Join(root, "doc.txt")
	writeFile(t, orig, "hello")

	if err := store.MoveToTrash(orig, "/doc.txt", "martin"); err != nil {
		t.Fatalf("MoveToTrash: %v", err)
	}
	if _, err := os.Stat(orig); !os.IsNotExist(err) {
		t.Fatalf("original should be gone after trashing, stat err = %v", err)
	}

	items := store.List("martin", "user")
	if len(items) != 1 {
		t.Fatalf("want 1 trashed item, got %d", len(items))
	}

	if err := store.Restore(items[0].ID, "martin", "user"); err != nil {
		t.Fatalf("Restore: %v", err)
	}
	if b, err := os.ReadFile(orig); err != nil || string(b) != "hello" {
		t.Fatalf("restored file = %q (err %v), want %q", b, err, "hello")
	}
}

func TestRestoreDeniesTraversal(t *testing.T) {
	root := t.TempDir()
	store := NewTrashStore(root)

	// Hand-craft a manifest item whose OriginalPath escapes the root, simulating
	// a corrupted/tampered manifest.
	trashed := filepath.Join(store.trashDir, "evil")
	writeFile(t, trashed, "payload")
	// A *relative* original path is the real escape vector — a leading-slash
	// path would be cleaned back inside root by filepath.Join.
	store.manifest = append(store.manifest, TrashItem{
		ID:           "evil",
		OriginalPath: "../escape.txt",
		Name:         "escape.txt",
		DeletedBy:    "martin",
		TrashPath:    trashed,
	})

	err := store.Restore("evil", "martin", "admin")
	if err == nil {
		t.Fatal("expected restore to be denied for path escaping root")
	}
	escaped := filepath.Join(filepath.Dir(root), "escape.txt")
	if _, statErr := os.Stat(escaped); statErr == nil {
		t.Fatalf("traversal wrote a file outside root at %s", escaped)
	}
}

func TestRestoreDoesNotClobber(t *testing.T) {
	root := t.TempDir()
	store := NewTrashStore(root)

	orig := filepath.Join(root, "report.txt")
	writeFile(t, orig, "old version")
	if err := store.MoveToTrash(orig, "/report.txt", "martin"); err != nil {
		t.Fatal(err)
	}
	// A new file now occupies the original location.
	writeFile(t, orig, "new version")

	items := store.List("martin", "admin")
	if err := store.Restore(items[0].ID, "martin", "admin"); err != nil {
		t.Fatalf("Restore: %v", err)
	}

	// The new file must be untouched.
	if b, _ := os.ReadFile(orig); string(b) != "new version" {
		t.Fatalf("clobbered existing file: got %q, want %q", b, "new version")
	}
	// The restored copy must exist alongside under a unique name.
	restored := filepath.Join(root, "report (restored).txt")
	if b, err := os.ReadFile(restored); err != nil || string(b) != "old version" {
		t.Fatalf("restored copy = %q (err %v), want %q at %s", b, err, "old version", restored)
	}
}

func TestMoveToTrashUniqueIDs(t *testing.T) {
	root := t.TempDir()
	store := NewTrashStore(root)

	for _, content := range []string{"first", "second"} {
		p := filepath.Join(root, "same.txt")
		writeFile(t, p, content)
		if err := store.MoveToTrash(p, "/same.txt", "martin"); err != nil {
			t.Fatal(err)
		}
	}

	items := store.List("martin", "admin")
	if len(items) != 2 {
		t.Fatalf("want 2 trashed items, got %d", len(items))
	}
	if items[0].ID == items[1].ID {
		t.Fatalf("trash ids collided: %s", items[0].ID)
	}
	// Both trashed payloads must survive on disk (no clobber in .trash).
	for _, it := range items {
		if _, err := os.Stat(it.TrashPath); err != nil {
			t.Fatalf("trashed payload missing for %s: %v", it.ID, err)
		}
	}
}

func TestRestorePermissionDenied(t *testing.T) {
	root := t.TempDir()
	store := NewTrashStore(root)

	orig := filepath.Join(root, "Nika", "private.txt")
	writeFile(t, orig, "secret")
	if err := store.MoveToTrash(orig, "/Nika/private.txt", "nika"); err != nil {
		t.Fatal(err)
	}
	items := store.List("nika", "admin")

	// A different non-admin user must not be able to restore nika's item.
	if err := store.Restore(items[0].ID, "martin", "user"); err == nil {
		t.Fatal("expected permission denied restoring another user's trashed item")
	}
}
