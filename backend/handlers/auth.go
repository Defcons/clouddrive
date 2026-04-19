package handlers

import (
	"clouddrive/middleware"
	"clouddrive/services"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// JWTLifetime is the token lifetime. Reduced from 7d to 24h to limit blast
// radius if a cookie is somehow stolen.
const JWTLifetime = 24 * time.Hour

type RateLimitResetter interface {
	Reset(r *http.Request)
}

type AuthHandler struct {
	userStore   *services.UserStore
	jwtSecret   []byte
	rateLimiter RateLimitResetter
	audit       *services.AuditLogger
	cookieSecure bool
}

func NewAuthHandler(userStore *services.UserStore, jwtSecret string, rateLimiter RateLimitResetter, audit *services.AuditLogger) *AuthHandler {
	// Secure cookies require HTTPS. In production (behind CF Tunnel + NPM) this
	// is always true, but disable for local dev where Go may be hit over HTTP.
	secure := os.Getenv("COOKIE_INSECURE") != "1"
	return &AuthHandler{
		userStore:    userStore,
		jwtSecret:    []byte(jwtSecret),
		rateLimiter:  rateLimiter,
		audit:        audit,
		cookieSecure: secure,
	}
}

func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Take the left-most value (closest to the real client).
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

	if h.audit != nil {
		h.audit.Log("LOGIN_OK", user.Username, ip, "")
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":        user.Username,
		"role":       user.Role,
		"homeFolder": user.HomeFolder,
		"pwv":        user.PwVersion,
		"exp":        time.Now().Add(JWTLifetime).Unix(),
		"iat":        time.Now().Unix(),
	})

	tokenString, err := token.SignedString(h.jwtSecret)
	if err != nil {
		http.Error(w, "Failed to create token", http.StatusInternalServerError)
		return
	}

	if h.rateLimiter != nil {
		h.rateLimiter.Reset(r)
	}

	h.setAuthCookie(w, tokenString)

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]string{
		"username":   user.Username,
		"role":       user.Role,
		"homeFolder": user.HomeFolder,
	}); err != nil {
		slog.Warn("encode login response failed", "err", err)
	}
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	h.clearAuthCookie(w)
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
		// Generic error; do not reveal whether username existed or password was wrong.
		http.Error(w, "Password change failed", http.StatusBadRequest)
		return
	}

	if h.audit != nil {
		h.audit.Log("PW_CHANGE", username, clientIP(r), "password changed")
	}

	// PwVersion bump invalidates the current session too — force re-login.
	h.clearAuthCookie(w)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "password changed"})
}
