package services

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadJSONFileValid(t *testing.T) {
	path := filepath.Join(t.TempDir(), "store.json")
	if err := os.WriteFile(path, []byte(`{"a":1,"b":2}`), 0600); err != nil {
		t.Fatal(err)
	}
	m := map[string]int{}
	loadJSONFile(path, &m)
	if m["a"] != 1 || m["b"] != 2 {
		t.Errorf("expected {a:1,b:2}, got %v", m)
	}
}

func TestLoadJSONFileMissingIsNoop(t *testing.T) {
	m := map[string]int{}
	loadJSONFile(filepath.Join(t.TempDir(), "nope.json"), &m)
	if len(m) != 0 {
		t.Errorf("missing file should leave the map empty, got %v", m)
	}
}

func TestLoadJSONFilePreservesCorrupt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "store.json")
	if err := os.WriteFile(path, []byte("{ this is not valid json"), 0600); err != nil {
		t.Fatal(err)
	}
	m := map[string]int{}
	loadJSONFile(path, &m)

	if len(m) != 0 {
		t.Errorf("corrupt file should yield an empty map, got %v", m)
	}
	// The corrupt content must be preserved, not silently discarded.
	if _, err := os.Stat(path + ".corrupt"); err != nil {
		t.Errorf("corrupt content should be preserved as %s.corrupt: %v", path, err)
	}
	// The original path is renamed away so a fresh save starts clean.
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("corrupt original should have been renamed away, stat err = %v", err)
	}
}

func TestTagStoreSurvivesCorruptFile(t *testing.T) {
	dir := t.TempDir()
	tagsPath := filepath.Join(dir, ".tags.json")
	if err := os.WriteFile(tagsPath, []byte("garbage{"), 0600); err != nil {
		t.Fatal(err)
	}

	store := NewTagStore(dir)
	if len(store.GetAllTagged()) != 0 {
		t.Error("store should start empty after a corrupt file")
	}
	// Corrupt content preserved rather than overwritten on next save.
	if _, err := os.Stat(tagsPath + ".corrupt"); err != nil {
		t.Errorf("expected preserved .corrupt file: %v", err)
	}
	if err := store.SetTags("/x", []string{"red"}); err != nil {
		t.Fatalf("store should still be usable: %v", err)
	}
}
