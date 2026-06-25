package middleware

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Typed context keys prevent collisions with other packages.
type ctxKey string

const (
	ctxKeyUsername   ctxKey = "username"
	ctxKeyRole       ctxKey = "role"
	ctxKeyHomeFolder ctxKey = "homeFolder"
	ctxKeySessionID  ctxKey = "sessionID"
)

// SessionValidator lets the middleware reject revoked sessions and record
// activity. Optional — when unset, the session id (jti) is not enforced.
type SessionValidator interface {
	IsValid(id string) bool
	Touch(id, ip string, nowMillis int64)
}

// AuthCookieName is the name of the HttpOnly cookie carrying the JWT.
const AuthCookieName = "clouddrive_session"

type PwVersionChecker interface {
	GetPwVersion(username string) int
}

type AuthMiddleware struct {
	jwtSecret []byte
	pwChecker PwVersionChecker
	sessions  SessionValidator
}

func NewAuthMiddleware(jwtSecret string, pwChecker PwVersionChecker) *AuthMiddleware {
	return &AuthMiddleware{jwtSecret: []byte(jwtSecret), pwChecker: pwChecker}
}

// SetSessionValidator enables per-session (jti) validation/revocation. When
// unset, session tokens are accepted on signature + claims alone.
func (m *AuthMiddleware) SetSessionValidator(v SessionValidator) {
	m.sessions = v
}

// GetSessionID returns the session id (jti) carried by the request's token.
func GetSessionID(r *http.Request) string {
	if v, ok := r.Context().Value(ctxKeySessionID).(string); ok {
		return v
	}
	return ""
}

// extractToken pulls the JWT from (1) the HttpOnly cookie, or (2) an
// Authorization: Bearer header (for non-browser API clients). Query-string
// tokens are NOT accepted — they leak into logs, history, and Referer.
func extractToken(r *http.Request) string {
	if c, err := r.Cookie(AuthCookieName); err == nil && c.Value != "" {
		return c.Value
	}
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return ""
	}
	token := strings.TrimPrefix(authHeader, "Bearer ")
	if token == authHeader {
		return "" // header was present but malformed
	}
	return token
}

func (m *AuthMiddleware) Wrap(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tokenString := extractToken(r)
		if tokenString == "" {
			http.Error(w, "Authorization required", http.StatusUnauthorized)
			return
		}

		token, err := jwt.Parse(tokenString, func(t *jwt.Token) (interface{}, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, jwt.ErrSignatureInvalid
			}
			return m.jwtSecret, nil
		})
		if err != nil || !token.Valid {
			http.Error(w, "Invalid token", http.StatusUnauthorized)
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			http.Error(w, "Invalid token", http.StatusUnauthorized)
			return
		}

		// Only full session tokens grant access here. Pre-auth tokens
		// (mfa_challenge, trusted_device) are signed with the same secret and
		// carry "sub", so without this check they would be accepted as a
		// session — letting password-only (no TOTP) reach protected routes.
		if kind, _ := claims["kind"].(string); kind != "session" {
			http.Error(w, "Invalid token", http.StatusUnauthorized)
			return
		}

		ctx := r.Context()
		username := ""
		if sub, ok := claims["sub"].(string); ok {
			username = sub
			ctx = context.WithValue(ctx, ctxKeyUsername, sub)
		}
		if role, ok := claims["role"].(string); ok {
			ctx = context.WithValue(ctx, ctxKeyRole, role)
		}
		if homeFolder, ok := claims["homeFolder"].(string); ok {
			ctx = context.WithValue(ctx, ctxKeyHomeFolder, homeFolder)
		}

		// Reject tokens issued before a password change.
		if m.pwChecker != nil && username != "" {
			tokenPwv := 0
			if pwv, ok := claims["pwv"].(float64); ok {
				tokenPwv = int(pwv)
			}
			currentPwv := m.pwChecker.GetPwVersion(username)
			if tokenPwv < currentPwv {
				http.Error(w, "Session expired — please login again", http.StatusUnauthorized)
				return
			}
		}

		// Per-session validation/revocation (when a validator is wired).
		jti, _ := claims["jti"].(string)
		if jti != "" {
			ctx = context.WithValue(ctx, ctxKeySessionID, jti)
		}
		if m.sessions != nil {
			if !m.sessions.IsValid(jti) {
				http.Error(w, "Session has been revoked — please login again", http.StatusUnauthorized)
				return
			}
			m.sessions.Touch(jti, getIP(r), time.Now().UnixMilli())
		}

		next(w, r.WithContext(ctx))
	}
}

func GetUsername(r *http.Request) string {
	if v, ok := r.Context().Value(ctxKeyUsername).(string); ok {
		return v
	}
	return ""
}

func GetRole(r *http.Request) string {
	if v, ok := r.Context().Value(ctxKeyRole).(string); ok {
		return v
	}
	return ""
}

func GetHomeFolder(r *http.Request) string {
	if v, ok := r.Context().Value(ctxKeyHomeFolder).(string); ok {
		return v
	}
	return ""
}

// ExtractToken is exported for the CSRF middleware to key tokens by session.
func ExtractToken(r *http.Request) string {
	return extractToken(r)
}
