package handlers

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The share browse page is served under CSP script-src 'self' (no
// 'unsafe-inline'), so it must contain no inline <script> or on* handlers —
// otherwise the collaborate upload silently does nothing.
func TestShareBrowsePageHasNoInlineJS(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "hello.txt"), []byte("hi"), 0644); err != nil {
		t.Fatal(err)
	}
	h := NewShareHandler(root, nil, nil, nil)

	token := "testtoken"
	h.shares[token] = &ShareEntry{
		Token:     token,
		FilePath:  "/",
		FileName:  "Shared",
		IsDir:     true,
		Mode:      "collaborate",
		ExpiresAt: time.Now().Add(time.Hour).UnixMilli(),
	}

	req := httptest.NewRequest(http.MethodGet, "/share/"+token, nil)
	rec := httptest.NewRecorder()
	h.Download(rec, req)

	body := rec.Body.String()
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	for _, bad := range []string{"<script", "onclick=", "onchange=", "ondrop=", "addEventListener"} {
		if strings.Contains(body, bad) {
			t.Errorf("browse page contains CSP-violating %q", bad)
		}
	}
	// The collaborate upload form must still be present and functional (no JS).
	if !strings.Contains(body, `type="file"`) || !strings.Contains(body, `type="submit"`) {
		t.Error("expected a no-JS upload form (file input + submit button)")
	}
}
