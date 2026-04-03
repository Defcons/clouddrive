package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

type PwVersionChecker interface {
	GetPwVersion(username string) int
}

type AuthMiddleware struct {
	jwtSecret  []byte
	pwChecker  PwVersionChecker
}

func NewAuthMiddleware(jwtSecret string, pwChecker PwVersionChecker) *AuthMiddleware {
	return &AuthMiddleware{jwtSecret: []byte(jwtSecret), pwChecker: pwChecker}
}

func (m *AuthMiddleware) Wrap(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Check Authorization header first, then fall back to ?token= query param
		tokenString := ""
		authHeader := r.Header.Get("Authorization")
		if authHeader != "" {
			tokenString = strings.TrimPrefix(authHeader, "Bearer ")
			if tokenString == authHeader {
				http.Error(w, "Invalid authorization format", http.StatusUnauthorized)
				return
			}
		} else {
			tokenString = r.URL.Query().Get("token")
		}

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

		// Extract claims and add to context
		if claims, ok := token.Claims.(jwt.MapClaims); ok {
			ctx := r.Context()
			username := ""
			if sub, ok := claims["sub"].(string); ok {
				username = sub
				ctx = context.WithValue(ctx, "username", sub)
			}
			if role, ok := claims["role"].(string); ok {
				ctx = context.WithValue(ctx, "role", role)
			}
			if homeFolder, ok := claims["homeFolder"].(string); ok {
				ctx = context.WithValue(ctx, "homeFolder", homeFolder)
			}

			// Check password version — reject tokens issued before a password change
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

			r = r.WithContext(ctx)
		}

		next(w, r)
	}
}

// Helper to get username from request context
func GetUsername(r *http.Request) string {
	if username, ok := r.Context().Value("username").(string); ok {
		return username
	}
	return ""
}

// Helper to get role from request context
func GetRole(r *http.Request) string {
	if role, ok := r.Context().Value("role").(string); ok {
		return role
	}
	return ""
}
