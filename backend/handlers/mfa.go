package handlers

import (
	"clouddrive/middleware"
	"clouddrive/services"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// MfaHandler groups MFA-related endpoints. It reuses the AuthHandler's user
// store and JWT secret — a thin wrapper rather than duplicating state.
type MfaHandler struct {
	userStore    *services.UserStore
	jwtSecret    []byte
	audit        *services.AuditLogger
	cookieSecure bool
}

func NewMfaHandler(userStore *services.UserStore, jwtSecret string, audit *services.AuditLogger) *MfaHandler {
	return &MfaHandler{
		userStore:    userStore,
		jwtSecret:    []byte(jwtSecret),
		audit:        audit,
		cookieSecure: os.Getenv("COOKIE_INSECURE") != "1",
	}
}

// Status — does the current user have MFA enabled? Used by Settings UI.
func (h *MfaHandler) Status(w http.ResponseWriter, r *http.Request) {
	username := middleware.GetUsername(r)
	enabled, codesLeft := h.userStore.MfaStatus(username)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"enabled":              enabled,
		"backupCodesRemaining": codesLeft,
	})
}

// StartSetup — generates a TOTP secret + QR code. The secret is returned to
// the client temporarily; it's committed only when /mfa/confirm succeeds.
// Password is re-verified here so a stolen session can't enable MFA silently.
func (h *MfaHandler) StartSetup(w http.ResponseWriter, r *http.Request) {
	var req struct {
		CurrentPassword string `json:"currentPassword"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}
	username := middleware.GetUsername(r)
	if _, err := h.userStore.Authenticate(username, req.CurrentPassword); err != nil {
		http.Error(w, "Invalid password", http.StatusUnauthorized)
		return
	}

	enrollment, err := h.userStore.GenerateEnrollment(username)
	if err != nil {
		slog.Error("mfa setup generate failed", "err", err)
		http.Error(w, "Failed to start MFA setup", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(enrollment)
}

// Confirm — user types the first 6-digit code from their authenticator to
// prove the QR code was scanned correctly. On success, MFA is enabled and
// backup codes are returned (shown once, never again).
func (h *MfaHandler) Confirm(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Secret string `json:"secret"`
		Code   string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}
	username := middleware.GetUsername(r)

	codes, err := h.userStore.ConfirmEnrollment(username, req.Secret, req.Code)
	if err != nil {
		http.Error(w, "Invalid code", http.StatusBadRequest)
		return
	}

	if h.audit != nil {
		h.audit.Log("MFA_ENABLE", username, clientIP(r), "MFA enabled")
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"enabled":     true,
		"backupCodes": codes,
	})
}

// Disable — turns MFA off. Password required to make session theft insufficient.
func (h *MfaHandler) Disable(w http.ResponseWriter, r *http.Request) {
	var req struct {
		CurrentPassword string `json:"currentPassword"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}
	username := middleware.GetUsername(r)
	if _, err := h.userStore.Authenticate(username, req.CurrentPassword); err != nil {
		http.Error(w, "Invalid password", http.StatusUnauthorized)
		return
	}
	if err := h.userStore.DisableMFA(username); err != nil {
		http.Error(w, "Failed to disable MFA", http.StatusInternalServerError)
		return
	}
	if h.audit != nil {
		h.audit.Log("MFA_DISABLE", username, clientIP(r), "MFA disabled")
	}
	// Also clear any trusted-device cookie so all browsers are re-verified
	// next time (if MFA is re-enabled).
	h.clearTrustedDeviceCookie(w)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]bool{"ok": true})
}

// RegenerateBackup — user lost their backup codes, generate a new set. The
// old ones are invalidated.
func (h *MfaHandler) RegenerateBackup(w http.ResponseWriter, r *http.Request) {
	var req struct {
		CurrentPassword string `json:"currentPassword"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}
	username := middleware.GetUsername(r)
	if _, err := h.userStore.Authenticate(username, req.CurrentPassword); err != nil {
		http.Error(w, "Invalid password", http.StatusUnauthorized)
		return
	}
	codes, err := h.userStore.RegenerateBackupCodes(username)
	if err != nil {
		http.Error(w, "Failed", http.StatusInternalServerError)
		return
	}
	if h.audit != nil {
		h.audit.Log("MFA_BACKUP_REGEN", username, clientIP(r), "backup codes regenerated")
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string][]string{"backupCodes": codes})
}

// Challenge — called by the client after Login returned mfa_required. Accepts
// the short-lived MFA-challenge JWT + a TOTP code (or backup code). On
// success, issues the real session cookie.
//
// This endpoint is NOT wrapped in auth middleware (the user has no session
// yet); instead it validates the mfa_token itself.
func (h *MfaHandler) Challenge(auth *AuthHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			MfaToken    string `json:"mfa_token"`
			Code        string `json:"code"`
			BackupCode  string `json:"backup_code"`
			TrustDevice bool   `json:"trust_device"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request", http.StatusBadRequest)
			return
		}

		// Validate the MFA challenge token.
		token, err := jwt.Parse(req.MfaToken, func(t *jwt.Token) (interface{}, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, jwt.ErrSignatureInvalid
			}
			return h.jwtSecret, nil
		})
		if err != nil || !token.Valid {
			http.Error(w, "Invalid or expired MFA session", http.StatusUnauthorized)
			return
		}
		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok || claims["kind"] != "mfa_challenge" {
			http.Error(w, "Invalid MFA session", http.StatusUnauthorized)
			return
		}
		username, _ := claims["sub"].(string)
		if username == "" {
			http.Error(w, "Invalid MFA session", http.StatusUnauthorized)
			return
		}

		user := h.userStore.GetUser(username)
		if user == nil || !user.MfaEnabled {
			http.Error(w, "MFA not configured", http.StatusBadRequest)
			return
		}

		// Accept either a TOTP code or a backup code.
		accepted := false
		usedBackup := false
		if req.Code != "" && h.userStore.ValidateTOTP(username, req.Code) {
			accepted = true
		} else if req.BackupCode != "" && h.userStore.ValidateBackupCode(username, req.BackupCode) {
			accepted = true
			usedBackup = true
		}

		if !accepted {
			if h.audit != nil {
				h.audit.Log("MFA_FAIL", username, clientIP(r), "invalid code")
			}
			http.Error(w, "Invalid code", http.StatusUnauthorized)
			return
		}

		if h.audit != nil {
			detail := "totp"
			if usedBackup {
				detail = "backup code (one consumed)"
			}
			h.audit.Log("LOGIN_OK", username, clientIP(r), detail)
		}

		if req.TrustDevice {
			h.issueTrustedDeviceCookie(w, username, user.PwVersion)
		}

		if err := auth.issueSession(w, user); err != nil {
			http.Error(w, "Failed to create session", http.StatusInternalServerError)
			return
		}
	}
}

// ---- Trusted device cookie ----

// trustedDeviceCookieName is set on successful MFA challenge (when user
// checks "Trust this device"). Presence + validity means skip MFA for 30 days.
const trustedDeviceCookieName = "clouddrive_trusted_device"
const trustedDeviceLifetime = 30 * 24 * time.Hour

// issueTrustedDeviceCookie mints a signed JWT tying this browser to this user.
// The cookie is path-scoped to / so it's included on login POSTs. It's
// HttpOnly to stop XSS from reading it and exfiltrating the "skip MFA" proof.
func (h *MfaHandler) issueTrustedDeviceCookie(w http.ResponseWriter, username string, pwVersion int) {
	// A small random nonce so two devices for same user have different cookies.
	nonce := make([]byte, 16)
	_, _ = rand.Read(nonce)

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":    username,
		"pwv":    pwVersion,
		"nonce":  base64.RawURLEncoding.EncodeToString(nonce),
		"kind":   "trusted_device",
		"exp":    time.Now().Add(trustedDeviceLifetime).Unix(),
		"iat":    time.Now().Unix(),
	})
	signed, err := token.SignedString(h.jwtSecret)
	if err != nil {
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     trustedDeviceCookieName,
		Value:    signed,
		Path:     "/",
		HttpOnly: true,
		Secure:   h.cookieSecure,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now().Add(trustedDeviceLifetime),
		MaxAge:   int(trustedDeviceLifetime.Seconds()),
	})
}

func (h *MfaHandler) clearTrustedDeviceCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     trustedDeviceCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   h.cookieSecure,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
	})
}

// HasValidTrustedDevice returns true if the cookie on r is valid for username.
func (h *MfaHandler) HasValidTrustedDevice(r *http.Request, username string, pwVersion int) bool {
	c, err := r.Cookie(trustedDeviceCookieName)
	if err != nil || c.Value == "" {
		return false
	}
	token, err := jwt.Parse(c.Value, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return h.jwtSecret, nil
	})
	if err != nil || !token.Valid {
		return false
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return false
	}
	if claims["kind"] != "trusted_device" {
		return false
	}
	if sub, _ := claims["sub"].(string); sub != username {
		return false
	}
	// Password change invalidates trusted devices (pwv is bumped on change).
	tokenPwv := 0
	if v, ok := claims["pwv"].(float64); ok {
		tokenPwv = int(v)
	}
	if tokenPwv != pwVersion {
		return false
	}
	return true
}
