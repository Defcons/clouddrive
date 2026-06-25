package handlers

import (
	"clouddrive/middleware"
	"clouddrive/services"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const authzTestSecret = "authz-test-secret-at-least-32-chars-xx"

type fixedPwChecker struct{}

func (fixedPwChecker) GetPwVersion(string) int { return 0 }

// sessionToken mints a valid session JWT for the given identity.
func sessionToken(t *testing.T, sub, role, home string) string {
	t.Helper()
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":        sub,
		"role":       role,
		"homeFolder": home,
		"pwv":        0,
		"kind":       "session",
		"exp":        time.Now().Add(time.Hour).Unix(),
		"iat":        time.Now().Unix(),
	})
	s, err := tok.SignedString([]byte(authzTestSecret))
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// serve runs handler through the real auth middleware as the given identity.
func serve(handler http.HandlerFunc, method, target, body, token string) *httptest.ResponseRecorder {
	am := middleware.NewAuthMiddleware(authzTestSecret, fixedPwChecker{})
	var r *http.Request
	if body != "" {
		r = httptest.NewRequest(method, target, strings.NewReader(body))
	} else {
		r = httptest.NewRequest(method, target, nil)
	}
	r.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	am.Wrap(handler)(rec, r)
	return rec
}

func TestGetPermissionDoesNotLeakCrossHome(t *testing.T) {
	ps := services.NewPermissionStore(t.TempDir())
	if err := ps.SetPrivate("/Martin/secret", "martin", []string{"martin"}); err != nil {
		t.Fatal(err)
	}
	h := NewPermissionsHandler(ps, nil)

	// Owner/admin sees the real privacy info.
	rec := serve(h.GetPermission, http.MethodGet, "/api/files/permissions?path=/Martin/secret", "",
		sessionToken(t, "martin", "admin", "/"))
	var admin map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &admin)
	if admin["isPrivate"] != true || admin["owner"] != "martin" {
		t.Errorf("owner/admin should see ACL, got %v", admin)
	}

	// A different non-admin must NOT learn it's private or who's allowed.
	rec = serve(h.GetPermission, http.MethodGet, "/api/files/permissions?path=/Martin/secret", "",
		sessionToken(t, "nika", "user", "/Nika"))
	var nika map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &nika)
	if nika["isPrivate"] != false || nika["owner"] != nil {
		t.Errorf("cross-home user must not see the ACL, got %v", nika)
	}
}

func TestBackupTierSetDeniedCrossHome(t *testing.T) {
	ps := services.NewPermissionStore(t.TempDir())
	store := services.NewBackupTierStore(t.TempDir())
	h := NewBackupTierHandler(store, ps, nil)

	rec := serve(h.Set, http.MethodPost, "/api/files/backup-tier", `{"path":"/Martin/data","tier":2}`,
		sessionToken(t, "nika", "user", "/Nika"))
	if rec.Code != http.StatusForbidden {
		t.Errorf("non-admin setting a tier outside home: got %d, want 403", rec.Code)
	}
	if store.GetTierExact("/Martin/data") != 0 {
		t.Error("tier was written despite the denial")
	}
}

func TestBackupTierListAdminOnly(t *testing.T) {
	store := services.NewBackupTierStore(t.TempDir())
	h := NewBackupTierHandler(store, nil, nil)

	if rec := serve(h.List, http.MethodGet, "/api/backup-tiers", "",
		sessionToken(t, "nika", "user", "/Nika")); rec.Code != http.StatusForbidden {
		t.Errorf("non-admin List: got %d, want 403", rec.Code)
	}
	if rec := serve(h.List, http.MethodGet, "/api/backup-tiers", "",
		sessionToken(t, "admin", "admin", "/")); rec.Code != http.StatusOK {
		t.Errorf("admin List: got %d, want 200", rec.Code)
	}
}
