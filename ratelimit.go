package main

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// rateLimiter throttles repeated failed authentication attempts (wrong
// admin/poll password) per client. It intentionally only counts failures,
// never successful requests or requests carrying no credential at all (e.g.
// the first, expected visit to a locked poll) - so legitimate concurrent use
// from a shared office IP isn't penalized, only active guessing is.
type rateLimiter struct {
	mu       sync.Mutex
	max      int
	window   time.Duration
	failures map[string][]time.Time
}

func newRateLimiter(max int, window time.Duration) *rateLimiter {
	rl := &rateLimiter{max: max, window: window, failures: map[string][]time.Time{}}
	go rl.sweepLoop()
	return rl
}

// blocked reports whether key has already hit the failure limit within the
// current window. Checking does not itself count as an attempt.
func (rl *rateLimiter) blocked(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	return len(rl.prune(key, time.Now())) >= rl.max
}

// fail records a failed attempt for key.
func (rl *rateLimiter) fail(key string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	now := time.Now()
	rl.failures[key] = append(rl.prune(key, now), now)
}

// prune drops (and persists) timestamps older than the window for key,
// returning what's left. Caller must hold rl.mu.
func (rl *rateLimiter) prune(key string, now time.Time) []time.Time {
	kept := rl.failures[key][:0]
	for _, t := range rl.failures[key] {
		if now.Sub(t) < rl.window {
			kept = append(kept, t)
		}
	}
	if len(kept) == 0 {
		delete(rl.failures, key)
		return nil
	}
	rl.failures[key] = kept
	return kept
}

// sweepLoop periodically prunes every key so that IPs which failed once and
// never came back don't accumulate in memory forever.
func (rl *rateLimiter) sweepLoop() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		rl.mu.Lock()
		now := time.Now()
		for key := range rl.failures {
			rl.prune(key, now)
		}
		rl.mu.Unlock()
	}
}

// clientIP extracts the client's IP for rate-limit bucketing (and request
// logging). By default this is simply the address the connection came in
// on. If that address belongs to a configured trusted proxy (see
// KONSENSOMAT_TRUSTED_PROXIES), the real client is instead read from
// X-Forwarded-For - scanned right to left, skipping any entries that are
// themselves trusted proxies, so a chain of known proxies (e.g. CDN + load
// balancer) resolves to the original client rather than the nearest hop -
// falling back to X-Real-IP if X-Forwarded-For is absent. Untrusted
// connections always use the raw address: X-Forwarded-For is just a request
// header, trivially set by anyone who can reach the server directly, so
// honoring it without knowing the request actually passed through a
// trusted proxy would let an attacker pick their own rate-limit bucket.
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}

	if len(trustedProxies) == 0 {
		return host
	}
	if ip := net.ParseIP(host); ip == nil || !isTrustedProxy(ip) {
		return host
	}

	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		for i := len(parts) - 1; i >= 0; i-- {
			candidate := strings.TrimSpace(parts[i])
			ip := net.ParseIP(candidate)
			if ip == nil {
				continue
			}
			if !isTrustedProxy(ip) {
				return candidate
			}
		}
	}

	if realIP := strings.TrimSpace(r.Header.Get("X-Real-IP")); realIP != "" {
		return realIP
	}

	return host
}

// credentialSupplied reports whether the request carries an explicit
// password/admin-password credential (header or query), as opposed to no
// credential at all - which is the normal, expected path the first time
// someone opens a locked poll and must not count as a guessing attempt.
func credentialSupplied(r *http.Request) bool {
	return suppliedPassword(r) != "" ||
		r.Header.Get("X-Admin-Password") != "" ||
		r.URL.Query().Get("adminPassword") != ""
}

func writeRateLimitedHTML(w http.ResponseWriter) {
	http.Error(w, "Zu viele Versuche, bitte später erneut versuchen.", http.StatusTooManyRequests)
}

func writeRateLimitedJSON(w http.ResponseWriter) {
	writeJSONError(w, http.StatusTooManyRequests, "Zu viele Versuche, bitte später erneut versuchen")
}
