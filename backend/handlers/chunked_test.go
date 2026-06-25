package handlers

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

func TestChunkedUploadAssembles(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "Nika"), 0755); err != nil {
		t.Fatal(err)
	}
	h := NewFileHandler(root, nil, nil, nil, nil, nil)
	tok := sessionToken(t, "nika", "user", "/Nika")
	uid := "upload-abc123"
	parts := []string{"Hello, ", "chunked ", "world!"}

	for i, p := range parts {
		target := fmt.Sprintf("/api/files/upload/chunk?uploadId=%s&index=%d&path=/Nika", uid, i)
		if rec := serve(h.UploadChunk, http.MethodPost, target, p, tok); rec.Code != http.StatusOK {
			t.Fatalf("chunk %d: got %d", i, rec.Code)
		}
	}

	body := fmt.Sprintf(`{"uploadId":%q,"name":"greeting.txt","path":"/Nika","total":3}`, uid)
	if rec := serve(h.UploadComplete, http.MethodPost, "/api/files/upload/complete", body, tok); rec.Code != http.StatusOK {
		t.Fatalf("complete: got %d (%s)", rec.Code, rec.Body.String())
	}

	got, err := os.ReadFile(filepath.Join(root, "Nika", "greeting.txt"))
	if err != nil {
		t.Fatalf("assembled file missing: %v", err)
	}
	if string(got) != "Hello, chunked world!" {
		t.Errorf("assembled = %q", got)
	}
	// Staging dir cleaned up.
	if _, err := os.Stat(filepath.Join(root, ".uploads", uid)); !os.IsNotExist(err) {
		t.Error("upload staging dir should be removed after complete")
	}
}

func TestChunkedUploadMissingChunkFails(t *testing.T) {
	root := t.TempDir()
	_ = os.MkdirAll(filepath.Join(root, "Nika"), 0755)
	h := NewFileHandler(root, nil, nil, nil, nil, nil)
	tok := sessionToken(t, "nika", "user", "/Nika")
	uid := "upload-gap999"

	// Send only chunk 0 but claim total 2.
	serve(h.UploadChunk, http.MethodPost, "/api/files/upload/chunk?uploadId="+uid+"&index=0&path=/Nika", "x", tok)
	body := fmt.Sprintf(`{"uploadId":%q,"name":"f.txt","path":"/Nika","total":2}`, uid)
	if rec := serve(h.UploadComplete, http.MethodPost, "/api/files/upload/complete", body, tok); rec.Code != http.StatusBadRequest {
		t.Fatalf("missing chunk: got %d, want 400", rec.Code)
	}
}

func TestChunkedUploadOwnershipEnforced(t *testing.T) {
	root := t.TempDir()
	_ = os.MkdirAll(filepath.Join(root, "Nika"), 0755)
	h := NewFileHandler(root, nil, nil, nil, nil, nil)
	uid := "upload-own777"

	// nika starts the upload...
	serve(h.UploadChunk, http.MethodPost, "/api/files/upload/chunk?uploadId="+uid+"&index=0&path=/Nika", "x", sessionToken(t, "nika", "user", "/Nika"))
	// ...martin (admin, so checkAccess passes) must not be able to add to it.
	rec := serve(h.UploadChunk, http.MethodPost, "/api/files/upload/chunk?uploadId="+uid+"&index=1&path=/Nika", "y", sessionToken(t, "martin", "admin", "/"))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("foreign writer: got %d, want 403", rec.Code)
	}
}
