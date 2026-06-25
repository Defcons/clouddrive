package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestGetIP(t *testing.T) {
	// getIP reads the package-level trustedProxies; set it for the test and
	// restore afterwards.
	orig := trustedProxies
	trustedProxies = parseTrustedProxies("10.0.0.0/8")
	t.Cleanup(func() { trustedProxies = orig })

	newReq := func(remoteAddr string, headers map[string]string) *http.Request {
		r := httptest.NewRequest(http.MethodPost, "/api/auth/login", nil)
		r.RemoteAddr = remoteAddr
		for k, v := range headers {
			r.Header.Set(k, v)
		}
		return r
	}

	tests := []struct {
		name    string
		req     *http.Request
		wantIP  string
		comment string
	}{
		{
			name:    "untrusted peer ignores spoofed XFF",
			req:     newReq("203.0.113.9:5555", map[string]string{"X-Forwarded-For": "1.1.1.1"}),
			wantIP:  "203.0.113.9",
			comment: "direct client must not be able to spoof its key",
		},
		{
			name:   "trusted proxy prefers X-Real-IP",
			req:    newReq("10.0.2.102:443", map[string]string{"X-Real-IP": "198.51.100.7", "X-Forwarded-For": "evil, 198.51.100.7"}),
			wantIP: "198.51.100.7",
		},
		{
			name:   "trusted proxy uses right-most XFF when no X-Real-IP",
			req:    newReq("10.0.2.102:443", map[string]string{"X-Forwarded-For": "1.1.1.1, 198.51.100.7"}),
			wantIP: "198.51.100.7",
		},
		{
			name:   "trusted proxy with no headers falls back to peer",
			req:    newReq("10.0.2.102:443", nil),
			wantIP: "10.0.2.102",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := getIP(tc.req); got != tc.wantIP {
				t.Errorf("getIP = %q, want %q (%s)", got, tc.wantIP, tc.comment)
			}
		})
	}
}

func TestRateLimiterLocksOut(t *testing.T) {
	rl := NewRateLimiter(3, time.Minute, time.Minute)
	r := httptest.NewRequest(http.MethodPost, "/api/auth/login", nil)
	r.RemoteAddr = "203.0.113.10:1234"

	// First 3 allowed, the 4th trips the lockout.
	for i := 1; i <= 3; i++ {
		if !rl.Check(r) {
			t.Fatalf("attempt %d should be allowed", i)
		}
	}
	if rl.Check(r) {
		t.Fatal("4th attempt should be rate limited")
	}

	// A successful login resets the bucket.
	rl.Reset(r)
	if !rl.Check(r) {
		t.Fatal("after reset the next attempt should be allowed")
	}
}
