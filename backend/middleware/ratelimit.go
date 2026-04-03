package middleware

import (
	"net/http"
	"sync"
	"time"
)

type attempt struct {
	count    int
	lastTry  time.Time
	lockedAt time.Time
}

type RateLimiter struct {
	attempts map[string]*attempt
	mu       sync.Mutex
	maxTries int
	window   time.Duration
	lockout  time.Duration
}

func NewRateLimiter(maxTries int, window, lockout time.Duration) *RateLimiter {
	rl := &RateLimiter{
		attempts: make(map[string]*attempt),
		maxTries: maxTries,
		window:   window,
		lockout:  lockout,
	}
	// Cleanup old entries every 10 minutes
	go func() {
		for {
			time.Sleep(10 * time.Minute)
			rl.cleanup()
		}
	}()
	return rl
}

func (rl *RateLimiter) cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	for key, att := range rl.attempts {
		if now.Sub(att.lastTry) > rl.window*2 {
			delete(rl.attempts, key)
		}
	}
}

func getIP(r *http.Request) string {
	// Check X-Forwarded-For (behind nginx reverse proxy)
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		return xff
	}
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}
	return r.RemoteAddr
}

// Check returns true if the request is allowed, false if rate limited
func (rl *RateLimiter) Check(r *http.Request) bool {
	ip := getIP(r)

	rl.mu.Lock()
	defer rl.mu.Unlock()

	att, exists := rl.attempts[ip]
	now := time.Now()

	if !exists {
		rl.attempts[ip] = &attempt{count: 1, lastTry: now}
		return true
	}

	// Check if locked out
	if !att.lockedAt.IsZero() && now.Sub(att.lockedAt) < rl.lockout {
		return false
	}

	// Reset if outside window
	if now.Sub(att.lastTry) > rl.window {
		att.count = 1
		att.lastTry = now
		att.lockedAt = time.Time{}
		return true
	}

	att.count++
	att.lastTry = now

	if att.count > rl.maxTries {
		att.lockedAt = now
		return false
	}

	return true
}

// RecordFailure records a failed attempt (call after auth failure)
func (rl *RateLimiter) RecordFailure(r *http.Request) {
	// Check already incremented the counter, nothing extra needed
}

// Reset clears the rate limit for an IP (call after successful login)
func (rl *RateLimiter) Reset(r *http.Request) {
	ip := getIP(r)

	rl.mu.Lock()
	defer rl.mu.Unlock()

	delete(rl.attempts, ip)
}

// WrapLogin wraps a login handler with rate limiting
func (rl *RateLimiter) WrapLogin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !rl.Check(r) {
			w.Header().Set("Retry-After", "60")
			http.Error(w, "Too many login attempts. Try again later.", http.StatusTooManyRequests)
			return
		}
		next(w, r)
	}
}
