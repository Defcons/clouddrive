package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"
	"sync"
	"time"
)

type csrfToken struct {
	token     string
	createdAt time.Time
}

type CSRFMiddleware struct {
	tokens map[string]*csrfToken // keyed by JWT token (user session)
	mu     sync.RWMutex
}

func NewCSRFMiddleware() *CSRFMiddleware {
	csrf := &CSRFMiddleware{
		tokens: make(map[string]*csrfToken),
	}
	// Cleanup old tokens every 30 minutes
	go func() {
		for {
			time.Sleep(30 * time.Minute)
			csrf.cleanup()
		}
	}()
	return csrf
}

func (c *CSRFMiddleware) cleanup() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	for key, tok := range c.tokens {
		if now.Sub(tok.createdAt) > 8*time.Hour {
			delete(c.tokens, key)
		}
	}
}

func generateCSRFToken() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func extractBearer(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if auth == "" {
		return r.URL.Query().Get("token")
	}
	return strings.TrimPrefix(auth, "Bearer ")
}

// GetToken returns (or creates) a CSRF token for the current session
func (c *CSRFMiddleware) GetToken(w http.ResponseWriter, r *http.Request) {
	sessionKey := extractBearer(r)
	if sessionKey == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	c.mu.Lock()
	tok, exists := c.tokens[sessionKey]
	if !exists || time.Now().Sub(tok.createdAt) > 8*time.Hour {
		tok = &csrfToken{token: generateCSRFToken(), createdAt: time.Now()}
		c.tokens[sessionKey] = tok
	}
	c.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"csrfToken":"` + tok.token + `"}`))
}

// Protect wraps a handler and validates CSRF token on state-changing requests (POST, DELETE)
func (c *CSRFMiddleware) Protect(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Only check state-changing methods
		if r.Method != "POST" && r.Method != "DELETE" && r.Method != "PUT" && r.Method != "PATCH" {
			next(w, r)
			return
		}

		sessionKey := extractBearer(r)
		if sessionKey == "" {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		csrfHeader := r.Header.Get("X-CSRF-Token")
		if csrfHeader == "" {
			http.Error(w, "CSRF token required", http.StatusForbidden)
			return
		}

		c.mu.RLock()
		tok, exists := c.tokens[sessionKey]
		c.mu.RUnlock()

		if !exists || tok.token != csrfHeader {
			http.Error(w, "Invalid CSRF token", http.StatusForbidden)
			return
		}

		next(w, r)
	}
}
