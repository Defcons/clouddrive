package handlers

import (
	"clouddrive/models"
	"clouddrive/services"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func userStoreForDav(t *testing.T) *services.UserStore {
	t.Helper()
	hash, _ := services.HashPassword("password123")
	cfg := models.UsersConfig{Users: []models.User{
		{Username: "nika", Password: hash, HomeFolder: "/Nika", Role: "user"},
	}}
	data, _ := json.MarshalIndent(cfg, "", "  ")
	p := filepath.Join(t.TempDir(), "users.json")
	if err := os.WriteFile(p, data, 0600); err != nil {
		t.Fatal(err)
	}
	s, err := services.NewUserStore(p)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestWebDAVRequiresAuth(t *testing.T) {
	h := NewWebDAVHandler(t.TempDir(), userStoreForDav(t))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("OPTIONS", "/webdav/", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("no auth: got %d, want 401", rec.Code)
	}
	if rec.Header().Get("WWW-Authenticate") == "" {
		t.Error("expected WWW-Authenticate header")
	}

	// Wrong password also 401.
	req := httptest.NewRequest("OPTIONS", "/webdav/", nil)
	req.SetBasicAuth("nika", "wrong")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("bad creds: got %d, want 401", rec.Code)
	}
}

func TestWebDAVPutScopedToHome(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "Nika"), 0755); err != nil {
		t.Fatal(err)
	}
	h := NewWebDAVHandler(root, userStoreForDav(t))

	req := httptest.NewRequest("PUT", "/webdav/hello.txt", strings.NewReader("hi"))
	req.SetBasicAuth("nika", "password123")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated && rec.Code != http.StatusNoContent {
		t.Fatalf("PUT: got %d", rec.Code)
	}

	// Written inside the user's home, not at the root.
	if b, err := os.ReadFile(filepath.Join(root, "Nika", "hello.txt")); err != nil || string(b) != "hi" {
		t.Fatalf("home file = %q (err %v)", b, err)
	}
	if _, err := os.Stat(filepath.Join(root, "hello.txt")); !os.IsNotExist(err) {
		t.Error("file escaped the home-folder scope")
	}
}
