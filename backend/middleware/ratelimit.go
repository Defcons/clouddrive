package middleware

import (
	"net"
	"net/http"
	"os"
	"strings"
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

// trustedProxies is the set of CIDRs whose X-Forwarded-For / X-Real-IP headers
// are trusted, parsed once from TRUSTED_PROXIES (comma-separated IPs or CIDRs,
// e.g. "10.0.0.0/8,127.0.0.1"). When empty (default), proxy headers are IGNORED
// and the direct connection IP is used — so a client hitting the origin
// directly cannot spoof X-Forwarded-For to evade the limiter.
var trustedProxies = parseTrustedProxies(os.Getenv("TRUSTED_PROXIES"))

func parseTrustedProxies(s string) []*net.IPNet {
	var nets []*net.IPNet
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if !strings.Contains(part, "/") {
			if strings.Contains(part, ":") {
				part += "/128"
			} else {
				part += "/32"
			}
		}
		if _, n, err := net.ParseCIDR(part); err == nil {
			nets = append(nets, n)
		}
	}
	return nets
}

func remoteIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func isTrustedProxy(ip string) bool {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return false
	}
	for _, n := range trustedProxies {
		if n.Contains(parsed) {
			return true
		}
	}
	return false
}

// getIP returns the client IP for rate-limiting. It honours
// X-Forwarded-For / X-Real-IP ONLY when the direct peer is a configured trusted
// proxy; otherwise it uses the direct connection address. This prevents a
// client from spoofing the header to bypass the login limiter.
func getIP(r *http.Request) string {
	peer := remoteIP(r)
	if isTrustedProxy(peer) {
		if xri := strings.TrimSpace(r.Header.Get("X-Real-IP")); xri != "" {
			return xri
		}
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			// Take the RIGHT-most entry. A trusted proxy appends the peer it
			// actually saw (nginx's $proxy_add_x_forwarded_for), so the
			// right-most value is the real client and cannot be forged. The
			// left-most is whatever the client chose to send, so trusting it
			// would let an attacker rotate it to evade the limiter.
			parts := strings.Split(xff, ",")
			if c := strings.TrimSpace(parts[len(parts)-1]); c != "" {
				return c
			}
		}
	}
	return peer
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
