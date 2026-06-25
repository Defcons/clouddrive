package handlers

import (
	"clouddrive/middleware"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestDownloadSupportsRange(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "f.bin"), []byte("0123456789"), 0644); err != nil {
		t.Fatal(err)
	}
	h := NewFileHandler(root, nil, nil, nil, nil, nil)

	// Full request advertises range support.
	full := serve(h.Download, http.MethodGet, "/api/files/download?path=/f.bin", "", sessionToken(t, "admin", "admin", "/"))
	if full.Code != http.StatusOK {
		t.Fatalf("full download: got %d", full.Code)
	}
	if full.Header().Get("Accept-Ranges") != "bytes" {
		t.Errorf("expected Accept-Ranges: bytes, got %q", full.Header().Get("Accept-Ranges"))
	}

	// A ranged request returns 206 with just the requested bytes.
	am := middleware.NewAuthMiddleware(authzTestSecret, fixedPwChecker{})
	req := httptest.NewRequest(http.MethodGet, "/api/files/download?path=/f.bin", nil)
	req.Header.Set("Authorization", "Bearer "+sessionToken(t, "admin", "admin", "/"))
	req.Header.Set("Range", "bytes=2-5")
	rec := httptest.NewRecorder()
	am.Wrap(h.Download)(rec, req)

	if rec.Code != http.StatusPartialContent {
		t.Fatalf("ranged download: got %d, want 206", rec.Code)
	}
	if got := rec.Body.String(); got != "2345" {
		t.Errorf("ranged body = %q, want %q", got, "2345")
	}
	if rec.Header().Get("Content-Range") == "" {
		t.Error("expected a Content-Range header on 206")
	}
}
