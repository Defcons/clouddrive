package handlers

import (
	"bytes"
	"clouddrive/middleware"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func multipartFile(t *testing.T, field, filename string, content []byte) (*bytes.Buffer, string) {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile(field, filename)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = fw.Write(content)
	_ = mw.Close()
	return &buf, mw.FormDataContentType()
}

func uploadAs(t *testing.T, h *FileHandler, target string, body *bytes.Buffer, contentType string) *httptest.ResponseRecorder {
	t.Helper()
	am := middleware.NewAuthMiddleware(authzTestSecret, fixedPwChecker{})
	req := httptest.NewRequest(http.MethodPost, "/api/files/upload?path="+target, body)
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Authorization", "Bearer "+sessionToken(t, "nika", "user", "/Nika"))
	rec := httptest.NewRecorder()
	am.Wrap(h.Upload)(rec, req)
	return rec
}

func TestUploadEnforcesQuota(t *testing.T) {
	root := t.TempDir()
	home := filepath.Join(root, "Nika")
	if err := os.MkdirAll(home, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, "existing.bin"), make([]byte, 100), 0644); err != nil {
		t.Fatal(err)
	}

	h := NewFileHandler(root, nil, nil, nil, nil, nil)
	h.SetQuotaLookup(func(string) int64 { return 150 })

	// 100 existing + 80 incoming = 180 > 150 → rejected, nothing written.
	body, ct := multipartFile(t, "files", "new.bin", make([]byte, 80))
	if rec := uploadAs(t, h, "/Nika", body, ct); rec.Code != http.StatusInsufficientStorage {
		t.Fatalf("over-quota upload: got %d, want 507", rec.Code)
	}
	if _, err := os.Stat(filepath.Join(home, "new.bin")); !os.IsNotExist(err) {
		t.Error("file was written despite quota rejection")
	}

	// 100 existing + 40 incoming = 140 <= 150 → allowed.
	body2, ct2 := multipartFile(t, "files", "ok.bin", make([]byte, 40))
	if rec := uploadAs(t, h, "/Nika", body2, ct2); rec.Code != http.StatusOK {
		t.Fatalf("within-quota upload: got %d, want 200", rec.Code)
	}
	if _, err := os.Stat(filepath.Join(home, "ok.bin")); err != nil {
		t.Errorf("within-quota file not written: %v", err)
	}
}

func TestUploadNoQuotaWhenUnset(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "Nika"), 0755); err != nil {
		t.Fatal(err)
	}
	h := NewFileHandler(root, nil, nil, nil, nil, nil) // no quota lookup
	body, ct := multipartFile(t, "files", "big.bin", make([]byte, 5000))
	if rec := uploadAs(t, h, "/Nika", body, ct); rec.Code != http.StatusOK {
		t.Fatalf("upload without quota: got %d", rec.Code)
	}
}
