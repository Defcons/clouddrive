package middleware

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"net/http"
	"sync"
	"time"
)

type csrfToken struct {
	token     string
	createdAt time.Time
}

type CSRFMiddleware struct {
	tokens map[string]*csrfToken // keyed by sha256(session JWT)
	mu     sync.RWMutex
}

func NewCSRFMiddleware() *CSRFMiddleware {
	csrf := &CSRFMiddleware{
		tokens: make(map[string]*csrfToken),
	}
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
	if _, err := rand.Read(b); err != nil {
		// On CSPRNG failure, return empty — callers treat as auth failure.
		return ""
	}
	return hex.EncodeToString(b)
}

// sessionKey hashes the session JWT to avoid storing it raw in the CSRF map.
func sessionKey(r *http.Request) string {
	raw := ExtractToken(r)
	if raw == "" {
		return ""
	}
	h := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(h[:])
}

// GetToken returns (or creates) a CSRF token for the current session.
func (c *CSRFMiddleware) GetToken(w http.ResponseWriter, r *http.Request) {
	key := sessionKey(r)
	if key == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	c.mu.Lock()
	tok, exists := c.tokens[key]
	if !exists || time.Since(tok.createdAt) > 8*time.Hour {
		generated := generateCSRFToken()
		if generated == "" {
			c.mu.Unlock()
			http.Error(w, "Failed to generate CSRF token", http.StatusInternalServerError)
			return
		}
		tok = &csrfToken{token: generated, createdAt: time.Now()}
		c.tokens[key] = tok
	}
	c.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"csrfToken":"` + tok.token + `"}`))
}

// Protect wraps a handler and validates CSRF on state-changing requests.
func (c *CSRFMiddleware) Protect(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" && r.Method != "DELETE" && r.Method != "PUT" && r.Method != "PATCH" {
			next(w, r)
			return
		}

		key := sessionKey(r)
		if key == "" {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		csrfHeader := r.Header.Get("X-CSRF-Token")
		if csrfHeader == "" {
			http.Error(w, "CSRF token required", http.StatusForbidden)
			return
		}

		c.mu.RLock()
		tok, exists := c.tokens[key]
		c.mu.RUnlock()

		if !exists || subtle.ConstantTimeCompare([]byte(tok.token), []byte(csrfHeader)) != 1 {
			http.Error(w, "Invalid CSRF token", http.StatusForbidden)
			return
		}

		next(w, r)
	}
}
