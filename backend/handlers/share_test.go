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

// Regression (L4): a correct share password must NOT be stored verbatim in the
// auth cookie — the cookie carries a derived value, and a GET presenting that
// derived value is what authenticates.
func TestSharePasswordCookieIsDerived(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "secret.txt"), []byte("classified"), 0644); err != nil {
		t.Fatal(err)
	}
	h := NewShareHandler(root, nil, nil, nil)

	const token = "protectedtoken"
	const password = "s3cr3t-share-pw"
	h.shares[token] = &ShareEntry{
		Token:     token,
		FilePath:  "/secret.txt",
		FileName:  "secret.txt",
		Mode:      "download",
		Password:  password,
		ExpiresAt: time.Now().Add(time.Hour).UnixMilli(),
	}

	// Correct password via POST → redirect + auth cookie set.
	req := httptest.NewRequest(http.MethodPost, "/share/"+token, strings.NewReader("password="+password))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.Download(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("correct password should redirect (303), got %d", rec.Code)
	}
	var authCookie *http.Cookie
	for _, c := range rec.Result().Cookies() {
		if c.Name == shareAuthCookieName(token) {
			authCookie = c
		}
	}
	if authCookie == nil {
		t.Fatal("expected an auth cookie to be set")
	}
	if authCookie.Value == password {
		t.Error("auth cookie stores the plaintext share password")
	}
	if !authCookie.HttpOnly {
		t.Error("auth cookie should be HttpOnly")
	}

	// A GET presenting that cookie is authenticated and serves the file.
	req2 := httptest.NewRequest(http.MethodGet, "/share/"+token, nil)
	req2.AddCookie(authCookie)
	rec2 := httptest.NewRecorder()
	h.Download(rec2, req2)
	if rec2.Code != http.StatusOK || rec2.Body.String() != "classified" {
		t.Fatalf("valid auth cookie should serve the file, got %d %q", rec2.Code, rec2.Body.String())
	}

	// A wrong cookie value must NOT serve the file (shows the password page).
	req3 := httptest.NewRequest(http.MethodGet, "/share/"+token, nil)
	req3.AddCookie(&http.Cookie{Name: shareAuthCookieName(token), Value: "bogus"})
	rec3 := httptest.NewRecorder()
	h.Download(rec3, req3)
	if strings.Contains(rec3.Body.String(), "classified") {
		t.Error("wrong cookie must not serve the file")
	}
}
