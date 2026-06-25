package services

import (
	"path/filepath"
	"testing"
)

func TestPermissionStoreMovePath(t *testing.T) {
	ps := NewPermissionStore(t.TempDir())
	_ = ps.SetPrivate("/Nika/secret", "nika", []string{"nika"})
	_ = ps.SetPrivate("/Nika/secret/inner", "nika", []string{"nika"})
	_ = ps.SetPrivate("/Nika/other", "nika", []string{"nika"})

	if err := ps.MovePath("/Nika/secret", "/Nika/renamed"); err != nil {
		t.Fatal(err)
	}

	// The moved folder and its descendant keep their restriction at the new path.
	if !ps.IsPrivate("/Nika/renamed") {
		t.Error("moved folder lost its restriction (would become public)")
	}
	if !ps.IsPrivate("/Nika/renamed/inner") {
		t.Error("descendant restriction not migrated")
	}
	// Old keys are gone; an unrelated sibling is untouched.
	if ps.IsPrivate("/Nika/secret") {
		t.Error("stale entry left at old path")
	}
	if !ps.IsPrivate("/Nika/other") {
		t.Error("unrelated sibling should be untouched")
	}
}

func TestMovePathKeysSiblingPrefixNotMatched(t *testing.T) {
	m := map[string]int{"/a/b": 2, "/a/bc": 2}
	movePathKeys(m, "/a/b", "/a/x")
	if _, ok := m["/a/x"]; !ok {
		t.Error("/a/b should have moved to /a/x")
	}
	// "/a/bc" must NOT be treated as a descendant of "/a/b".
	if _, ok := m["/a/bc"]; !ok {
		t.Error("sibling /a/bc must be untouched")
	}
	if _, ok := m["/a/b"]; ok {
		t.Error("old key /a/b should be gone")
	}
}

func TestTagStoreMovePath(t *testing.T) {
	ts := NewTagStore(t.TempDir())
	_ = ts.SetTags("/docs/a.txt", []string{"red"})
	if err := ts.MovePath("/docs/a.txt", filepath.ToSlash("/docs/b.txt")); err != nil {
		t.Fatal(err)
	}
	if len(ts.GetTags("/docs/b.txt")) != 1 {
		t.Error("tag did not follow the rename")
	}
	if len(ts.GetTags("/docs/a.txt")) != 0 {
		t.Error("stale tag left at old path")
	}
}

func TestPermissionStorePrunePath(t *testing.T) {
	ps := NewPermissionStore(t.TempDir())
	_ = ps.SetPrivate("/Nika/x", "nika", []string{"nika"})
	_ = ps.SetPrivate("/Nika/x/inner", "nika", []string{"nika"})
	_ = ps.SetPrivate("/Nika/xy", "nika", []string{"nika"}) // sibling prefix

	if err := ps.PrunePath("/Nika/x"); err != nil {
		t.Fatal(err)
	}
	if ps.IsPrivate("/Nika/x") || ps.IsPrivate("/Nika/x/inner") {
		t.Error("path and descendant should be pruned")
	}
	if !ps.IsPrivate("/Nika/xy") {
		t.Error("sibling /Nika/xy must not be pruned by /Nika/x")
	}
}

func TestTrashPermanentDeleteTriggersPruner(t *testing.T) {
	root := t.TempDir()
	store := NewTrashStore(root)
	var pruned []string
	store.SetMetadataPruner(func(p string) { pruned = append(pruned, p) })

	orig := filepath.Join(root, "doc.txt")
	writeFile(t, orig, "x")
	if err := store.MoveToTrash(orig, "/doc.txt", "nika"); err != nil {
		t.Fatal(err)
	}
	items := store.List("nika", "admin")

	// Restore must NOT prune (item kept its metadata while in trash).
	// Here we permanently delete instead and expect a prune call.
	if err := store.PermanentDelete(items[0].ID, "nika", "admin"); err != nil {
		t.Fatal(err)
	}
	if len(pruned) != 1 || pruned[0] != "/doc.txt" {
		t.Errorf("expected pruner called with /doc.txt, got %v", pruned)
	}
}
