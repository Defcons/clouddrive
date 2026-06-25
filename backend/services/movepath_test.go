package services

import (
	"path/filepath"
	"testing"
)

func TestPermissionStoreMovePath(t *testing.T) {
	ps := NewPermissionStore(t.TempDir())
	_ = ps.SetPrivate("/Alice/secret", "alice", []string{"alice"})
	_ = ps.SetPrivate("/Alice/secret/inner", "alice", []string{"alice"})
	_ = ps.SetPrivate("/Alice/other", "alice", []string{"alice"})

	if err := ps.MovePath("/Alice/secret", "/Alice/renamed"); err != nil {
		t.Fatal(err)
	}

	// The moved folder and its descendant keep their restriction at the new path.
	if !ps.IsPrivate("/Alice/renamed") {
		t.Error("moved folder lost its restriction (would become public)")
	}
	if !ps.IsPrivate("/Alice/renamed/inner") {
		t.Error("descendant restriction not migrated")
	}
	// Old keys are gone; an unrelated sibling is untouched.
	if ps.IsPrivate("/Alice/secret") {
		t.Error("stale entry left at old path")
	}
	if !ps.IsPrivate("/Alice/other") {
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
