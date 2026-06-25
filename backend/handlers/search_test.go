package handlers

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

func TestContentSearch(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "notes.txt"), []byte("the quick brown fox jumps"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "other.txt"), []byte("nothing relevant here"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "data.bin"), []byte("brown binary"), 0644); err != nil {
		t.Fatal(err) // non-text ext, must be ignored
	}
	h := NewFileHandler(root, nil, nil, nil, nil, nil)

	// Without content=1, "brown" matches no filename → empty.
	noContent := serve(h.Search, http.MethodGet, "/api/files/search?q=brown", "", sessionToken(t, "admin", "admin", "/"))
	var r1 []FileInfo
	_ = json.Unmarshal(noContent.Body.Bytes(), &r1)
	if len(r1) != 0 {
		t.Errorf("filename-only search for 'brown' should be empty, got %d", len(r1))
	}

	// With content=1, it finds notes.txt (text) but not data.bin (binary ext).
	withContent := serve(h.Search, http.MethodGet, "/api/files/search?q=brown&content=1", "", sessionToken(t, "admin", "admin", "/"))
	var r2 []FileInfo
	_ = json.Unmarshal(withContent.Body.Bytes(), &r2)
	if len(r2) != 1 || r2[0].Name != "notes.txt" {
		t.Fatalf("content search = %+v, want only notes.txt", r2)
	}
	if r2[0].Snippet == "" {
		t.Error("expected a snippet on the content match")
	}
}

func TestContentSearchDedupesFilenameMatch(t *testing.T) {
	root := t.TempDir()
	// "report" is in both the name and the content — must appear once.
	if err := os.WriteFile(filepath.Join(root, "report.txt"), []byte("quarterly report inside"), 0644); err != nil {
		t.Fatal(err)
	}
	h := NewFileHandler(root, nil, nil, nil, nil, nil)
	rec := serve(h.Search, http.MethodGet, "/api/files/search?q=report&content=1", "", sessionToken(t, "admin", "admin", "/"))
	var res []FileInfo
	_ = json.Unmarshal(rec.Body.Bytes(), &res)
	if len(res) != 1 {
		t.Errorf("want 1 deduped result, got %d: %+v", len(res), res)
	}
}
