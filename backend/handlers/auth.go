package handlers

import (
	"clouddrive/middleware"
	"clouddrive/models"
	"clouddrive/services"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// JWTLifetime is the session lifetime. Reduced from 7d to 24h.
const JWTLifetime = 24 * time.Hour

// mfaChallengeLifetime is how long the user has after entering password to
// submit their TOTP code. Short on purpose.
const mfaChallengeLifetime = 5 * time.Minute

type RateLimitResetter interface {
	Reset(r *http.Request)
}

type AuthHandler struct {
	userStore    *services.UserStore
	jwtSecret    []byte
	rateLimiter  RateLimitResetter
	audit        *services.AuditLogger
	cookieSecure bool
	mfa          *MfaHandler // non-nil; used to check trusted-device cookie
}

func NewAuthHandler(userStore *services.UserStore, jwtSecret string, rateLimiter RateLimitResetter, audit *services.AuditLogger, mfa *MfaHandler) *AuthHandler {
	secure := os.Getenv("COOKIE_INSECURE") != "1"
	return &AuthHandler{
		userStore:    userStore,
		jwtSecret:    []byte(jwtSecret),
		rateLimiter:  rateLimiter,
		audit:        audit,
		cookieSecure: secure,
		mfa:          mfa,
	}
}

func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if comma := strings.Index(xff, ","); comma >= 0 {
			return strings.TrimSpace(xff[:comma])
		}
		return strings.TrimSpace(xff)
	}
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}
	return r.RemoteAddr
}

func (h *AuthHandler) setAuthCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     middleware.AuthCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   h.cookieSecure,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now().Add(JWTLifetime),
		MaxAge:   int(JWTLifetime.Seconds()),
	})
}

func (h *AuthHandler) clearAuthCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     middleware.AuthCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   h.cookieSecure,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
	})
}

// signSessionToken mints the main session JWT carried in the auth cookie.
func (h *AuthHandler) signSessionToken(user *models.User) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":        user.Username,
		"role":       user.Role,
		"homeFolder": user.HomeFolder,
		"pwv":        user.PwVersion,
		"kind":       "session",
		"exp":        time.Now().Add(JWTLifetime).Unix(),
		"iat":        time.Now().Unix(),
	})
	return token.SignedString(h.jwtSecret)
}

// issueSession writes the session cookie and the user-info JSON. Shared by
// Login (MFA-disabled path) and MfaHandler.Challenge (MFA-enabled path).
func (h *AuthHandler) issueSession(w http.ResponseWriter, user *models.User) error {
	tokenString, err := h.signSessionToken(user)
	if err != nil {
		return err
	}
	h.setAuthCookie(w, tokenString)
	w.Header().Set("Content-Type", "application/json")
	return json.NewEncoder(w).Encode(map[string]interface{}{
		"username":   user.Username,
		"role":       user.Role,
		"homeFolder": user.HomeFolder,
	})
}

// signMfaChallengeToken mints a short-lived JWT returned to the client when
// MFA is required. The client submits it along with the TOTP code to
// /api/auth/mfa/challenge. This is NOT the session cookie — it carries no
// file-access privileges, only proves the user got past the password step.
func (h *AuthHandler) signMfaChallengeToken(username string) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":  username,
		"kind": "mfa_challenge",
		"exp":  time.Now().Add(mfaChallengeLifetime).Unix(),
		"iat":  time.Now().Unix(),
	})
	return token.SignedString(h.jwtSecret)
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	ip := clientIP(r)

	user, err := h.userStore.Authenticate(req.Username, req.Password)
	if err != nil {
		if h.audit != nil {
			h.audit.Log("LOGIN_FAIL", req.Username, ip, "invalid credentials")
		}
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	// If MFA is enabled, we either:
	//   (a) have a valid trusted-device cookie from the last 30 days → skip MFA
	//   (b) return mfa_required + mfa_token so client can submit TOTP next
	if user.MfaEnabled {
		if h.mfa != nil && h.mfa.HasValidTrustedDevice(r, user.Username, user.PwVersion) {
			// Trusted device — issue session directly.
			if h.audit != nil {
				h.audit.Log("LOGIN_OK", user.Username, ip, "trusted device")
			}
			if h.rateLimiter != nil {
				h.rateLimiter.Reset(r)
			}
			if err := h.issueSession(w, user); err != nil {
				http.Error(w, "Failed to create session", http.StatusInternalServerError)
			}
			return
		}

		// MFA challenge required.
		mfaToken, err := h.signMfaChallengeToken(user.Username)
		if err != nil {
			http.Error(w, "Failed to start MFA challenge", http.StatusInternalServerError)
			return
		}
		if h.audit != nil {
			h.audit.Log("MFA_REQUIRED", user.Username, ip, "")
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"mfa_required": true,
			"mfa_token":    mfaToken,
		})
		return
	}

	if h.audit != nil {
		h.audit.Log("LOGIN_OK", user.Username, ip, "")
	}

	if h.rateLimiter != nil {
		h.rateLimiter.Reset(r)
	}

	if err := h.issueSession(w, user); err != nil {
		slog.Warn("issue session failed", "err", err)
		http.Error(w, "Failed to create session", http.StatusInternalServerError)
	}
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	h.clearAuthCookie(w)
	// Note: trusted-device cookie is NOT cleared on logout — that's the point
	// of "trust this device." Clearing happens on password change or explicit
	// MFA disable.
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]bool{"ok": true})
}

func (h *AuthHandler) Check(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"valid":      true,
		"username":   middleware.GetUsername(r),
		"role":       middleware.GetRole(r),
		"homeFolder": middleware.GetHomeFolder(r),
	})
}

func (h *AuthHandler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	var req struct {
		CurrentPassword string `json:"currentPassword"`
		NewPassword     string `json:"newPassword"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	username := middleware.GetUsername(r)
	if username == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if len(req.NewPassword) < 8 {
		http.Error(w, "New password must be at least 8 characters", http.StatusBadRequest)
		return
	}

	if err := h.userStore.ChangePassword(username, req.CurrentPassword, req.NewPassword); err != nil {
		slog.Info("password change failed", "user", username, "err", err)
		http.Error(w, "Password change failed", http.StatusBadRequest)
		return
	}

	if h.audit != nil {
		h.audit.Log("PW_CHANGE", username, clientIP(r), "password changed")
	}

	// Password change bumps pwv → invalidates session AND all trusted-device
	// cookies. Clear cookies here so the browser stops sending them.
	h.clearAuthCookie(w)
	if h.mfa != nil {
		h.mfa.clearTrustedDeviceCookie(w)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "password changed"})
}
