package handlers

import (
	"clouddrive/services"
	"encoding/json"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type RateLimitResetter interface {
	Reset(r *http.Request)
}

type AuthHandler struct {
	userStore   *services.UserStore
	jwtSecret   []byte
	rateLimiter RateLimitResetter
	audit       *services.AuditLogger
}

func NewAuthHandler(userStore *services.UserStore, jwtSecret string, rateLimiter RateLimitResetter, audit *services.AuditLogger) *AuthHandler {
	return &AuthHandler{
		userStore:   userStore,
		jwtSecret:   []byte(jwtSecret),
		rateLimiter: rateLimiter,
		audit:       audit,
	}
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

	ip := r.Header.Get("X-Forwarded-For")
	if ip == "" {
		ip = r.RemoteAddr
	}

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
		"exp":        time.Now().Add(7 * 24 * time.Hour).Unix(),
		"iat":        time.Now().Unix(),
	})

	tokenString, err := token.SignedString(h.jwtSecret)
	if err != nil {
		http.Error(w, "Failed to create token", http.StatusInternalServerError)
		return
	}

	// Clear rate limit on successful login
	if h.rateLimiter != nil {
		h.rateLimiter.Reset(r)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"token":      tokenString,
		"username":   user.Username,
		"role":       user.Role,
		"homeFolder": user.HomeFolder,
	})
}

func (h *AuthHandler) Check(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"valid": true})
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

	// Get username from context (set by auth middleware)
	username, ok := r.Context().Value("username").(string)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if err := h.userStore.ChangePassword(username, req.CurrentPassword, req.NewPassword); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if h.audit != nil {
		auditIP := r.Header.Get("X-Forwarded-For")
		if auditIP == "" {
			auditIP = r.RemoteAddr
		}
		h.audit.Log("PW_CHANGE", username, auditIP, "password changed")
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "password changed"})
}
