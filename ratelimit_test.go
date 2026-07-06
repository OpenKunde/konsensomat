package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRateLimiterBlocksAfterMaxFailures(t *testing.T) {
	rl := newRateLimiter(3, time.Minute)

	for i := 0; i < 2; i++ {
		if rl.blocked("1.2.3.4") {
			t.Fatalf("blocked after %d failures, want not blocked yet (max=3)", i)
		}
		rl.fail("1.2.3.4")
	}
	// Two failures recorded so far - still under the limit.
	if rl.blocked("1.2.3.4") {
		t.Fatal("blocked after 2 failures, want not blocked yet (max=3)")
	}

	rl.fail("1.2.3.4")
	if !rl.blocked("1.2.3.4") {
		t.Fatal("expected blocked after 3 failures (max=3)")
	}
}

func TestRateLimiterKeysAreIndependent(t *testing.T) {
	rl := newRateLimiter(1, time.Minute)
	rl.fail("1.2.3.4")

	if !rl.blocked("1.2.3.4") {
		t.Error("expected 1.2.3.4 to be blocked")
	}
	if rl.blocked("5.6.7.8") {
		t.Error("failures for one key must not affect a different key")
	}
}

func TestRateLimiterCheckingIsNotAnAttempt(t *testing.T) {
	rl := newRateLimiter(1, time.Minute)

	for i := 0; i < 5; i++ {
		if rl.blocked("1.2.3.4") {
			t.Fatal("blocked() must never itself count as a failed attempt")
		}
	}
}

func TestRateLimiterWindowExpiry(t *testing.T) {
	rl := newRateLimiter(1, 50*time.Millisecond)
	rl.fail("1.2.3.4")

	if !rl.blocked("1.2.3.4") {
		t.Fatal("expected blocked immediately after hitting the limit")
	}

	time.Sleep(80 * time.Millisecond)

	if rl.blocked("1.2.3.4") {
		t.Error("expected the failure to have aged out of the window")
	}
}

func TestCredentialSupplied(t *testing.T) {
	none := httptest.NewRequest(http.MethodGet, "/", nil)
	if credentialSupplied(none) {
		t.Error("a plain request should not be seen as supplying a credential")
	}

	pw := httptest.NewRequest(http.MethodGet, "/?password=x", nil)
	if !credentialSupplied(pw) {
		t.Error("?password= should count as a supplied credential")
	}

	header := httptest.NewRequest(http.MethodGet, "/", nil)
	header.Header.Set("X-Poll-Password", "x")
	if !credentialSupplied(header) {
		t.Error("X-Poll-Password header should count as a supplied credential")
	}

	adminHeader := httptest.NewRequest(http.MethodGet, "/", nil)
	adminHeader.Header.Set("X-Admin-Password", "x")
	if !credentialSupplied(adminHeader) {
		t.Error("X-Admin-Password header should count as a supplied credential")
	}

	adminQuery := httptest.NewRequest(http.MethodGet, "/?adminPassword=x", nil)
	if !credentialSupplied(adminQuery) {
		t.Error("?adminPassword= should count as a supplied credential")
	}
}

func TestClientIP(t *testing.T) {
	withPort := httptest.NewRequest(http.MethodGet, "/", nil)
	withPort.RemoteAddr = "203.0.113.7:54321"
	if got := clientIP(withPort); got != "203.0.113.7" {
		t.Errorf("clientIP = %q, want 203.0.113.7", got)
	}

	// No port present: SplitHostPort fails, so clientIP falls back to the
	// raw RemoteAddr rather than panicking or returning empty.
	noPort := httptest.NewRequest(http.MethodGet, "/", nil)
	noPort.RemoteAddr = "203.0.113.7"
	if got := clientIP(noPort); got != "203.0.113.7" {
		t.Errorf("clientIP (no port) = %q, want 203.0.113.7", got)
	}
}
