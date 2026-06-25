package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const testSecret = "test-secret-at-least-32-chars-long-xxxxx"

type fakePwChecker struct{ version int }

func (f fakePwChecker) GetPwVersion(string) int { return f.version }

func mintHS256(t *testing.T, secret string, claims jwt.MapClaims) string {
	t.Helper()
	s, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return s
}

func baseClaims() jwt.MapClaims {
	return jwt.MapClaims{
		"sub": "martin",
		"exp": time.Now().Add(time.Hour).Unix(),
		"iat": time.Now().Unix(),
	}
}

// runWrap sends a request carrying token as a Bearer header through the auth
// middleware and reports whether the inner handler ran + the resulting status.
func runWrap(t *testing.T, m *AuthMiddleware, token string) (called bool, status int) {
	t.Helper()
	h := m.Wrap(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})
	req := httptest.NewRequest(http.MethodGet, "/api/files", nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	h(rec, req)
	return called, rec.Code
}

func TestWrapRejectsNonSessionTokens(t *testing.T) {
	m := NewAuthMiddleware(testSecret, fakePwChecker{version: 0})

	tests := []struct {
		name       string
		claims     jwt.MapClaims
		wantCalled bool
		wantStatus int
	}{
		{
			name:       "valid session token is accepted",
			claims:     withKind(baseClaims(), "session"),
			wantCalled: true,
			wantStatus: http.StatusOK,
		},
		{
			name:       "mfa_challenge token is rejected",
			claims:     withKind(baseClaims(), "mfa_challenge"),
			wantCalled: false,
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "trusted_device token is rejected",
			claims:     withKind(baseClaims(), "trusted_device"),
			wantCalled: false,
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "legacy token without kind is rejected",
			claims:     baseClaims(),
			wantCalled: false,
			wantStatus: http.StatusUnauthorized,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			token := mintHS256(t, testSecret, tc.claims)
			called, status := runWrap(t, m, token)
			if called != tc.wantCalled || status != tc.wantStatus {
				t.Errorf("got called=%v status=%d, want called=%v status=%d",
					called, status, tc.wantCalled, tc.wantStatus)
			}
		})
	}
}

func TestWrapRejectsBadSignatureAndAlg(t *testing.T) {
	m := NewAuthMiddleware(testSecret, fakePwChecker{version: 0})

	t.Run("wrong signing secret", func(t *testing.T) {
		token := mintHS256(t, "a-totally-different-secret-also-32-chars", withKind(baseClaims(), "session"))
		if called, status := runWrap(t, m, token); called || status != http.StatusUnauthorized {
			t.Errorf("got called=%v status=%d, want called=false status=401", called, status)
		}
	})

	t.Run("alg none is rejected", func(t *testing.T) {
		token, err := jwt.NewWithClaims(jwt.SigningMethodNone, withKind(baseClaims(), "session")).
			SignedString(jwt.UnsafeAllowNoneSignatureType)
		if err != nil {
			t.Fatalf("sign none: %v", err)
		}
		if called, status := runWrap(t, m, token); called || status != http.StatusUnauthorized {
			t.Errorf("got called=%v status=%d, want called=false status=401", called, status)
		}
	})

	t.Run("missing token is rejected", func(t *testing.T) {
		if called, status := runWrap(t, m, ""); called || status != http.StatusUnauthorized {
			t.Errorf("got called=%v status=%d, want called=false status=401", called, status)
		}
	})
}

func TestWrapEnforcesPasswordVersion(t *testing.T) {
	// pwChecker reports current version 2; a session token stamped with an older
	// version must be rejected (password was changed since it was issued).
	m := NewAuthMiddleware(testSecret, fakePwChecker{version: 2})

	stale := withKind(baseClaims(), "session")
	stale["pwv"] = 1
	if called, status := runWrap(t, m, mintHS256(t, testSecret, stale)); called || status != http.StatusUnauthorized {
		t.Errorf("stale pwv: got called=%v status=%d, want called=false status=401", called, status)
	}

	current := withKind(baseClaims(), "session")
	current["pwv"] = 2
	if called, status := runWrap(t, m, mintHS256(t, testSecret, current)); !called || status != http.StatusOK {
		t.Errorf("current pwv: got called=%v status=%d, want called=true status=200", called, status)
	}
}

func withKind(c jwt.MapClaims, kind string) jwt.MapClaims {
	c["kind"] = kind
	return c
}

type fakeSessions struct{ valid map[string]bool }

func (f fakeSessions) IsValid(id string) bool        { return f.valid[id] }
func (f fakeSessions) Touch(id, ip string, ms int64) {}

func TestWrapEnforcesSessionValidator(t *testing.T) {
	m := NewAuthMiddleware(testSecret, fakePwChecker{version: 0})
	m.SetSessionValidator(fakeSessions{valid: map[string]bool{"good": true}})

	withJTI := func(jti string) jwt.MapClaims {
		c := withKind(baseClaims(), "session")
		c["jti"] = jti
		return c
	}

	// Valid, active session → allowed.
	if called, status := runWrap(t, m, mintHS256(t, testSecret, withJTI("good"))); !called || status != http.StatusOK {
		t.Errorf("active session: called=%v status=%d, want true/200", called, status)
	}
	// Revoked/unknown session id → 401.
	if called, status := runWrap(t, m, mintHS256(t, testSecret, withJTI("revoked"))); called || status != http.StatusUnauthorized {
		t.Errorf("revoked session: called=%v status=%d, want false/401", called, status)
	}
	// Missing jti with a validator set → 401.
	if called, status := runWrap(t, m, mintHS256(t, testSecret, withKind(baseClaims(), "session"))); called || status != http.StatusUnauthorized {
		t.Errorf("missing jti: called=%v status=%d, want false/401", called, status)
	}
}
