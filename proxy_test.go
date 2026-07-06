package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// withTrustedProxies temporarily replaces the package-level trustedProxies
// for the duration of the test, restoring the previous value afterwards -
// needed because clientIP consults that global rather than taking it as a
// parameter.
func withTrustedProxies(t *testing.T, raw string) {
	t.Helper()
	nets, err := parseTrustedProxies(raw)
	if err != nil {
		t.Fatalf("parseTrustedProxies(%q): %v", raw, err)
	}
	old := trustedProxies
	trustedProxies = nets
	t.Cleanup(func() { trustedProxies = old })
}

func TestParseTrustedProxiesEmpty(t *testing.T) {
	nets, err := parseTrustedProxies("")
	if err != nil {
		t.Fatalf("parseTrustedProxies(\"\"): %v", err)
	}
	if nets != nil {
		t.Errorf("expected nil for an empty config, got %#v", nets)
	}
}

func TestParseTrustedProxiesSingleIPsAndCIDRs(t *testing.T) {
	nets, err := parseTrustedProxies("127.0.0.1, 10.0.0.0/8 ,::1")
	if err != nil {
		t.Fatalf("parseTrustedProxies: %v", err)
	}
	if len(nets) != 3 {
		t.Fatalf("expected 3 entries, got %d: %#v", len(nets), nets)
	}
}

func TestParseTrustedProxiesRejectsGarbage(t *testing.T) {
	cases := []string{"not-an-ip", "10.0.0.0/999", "999.999.999.999"}
	for _, c := range cases {
		if _, err := parseTrustedProxies(c); err == nil {
			t.Errorf("parseTrustedProxies(%q): expected an error", c)
		}
	}
}

func TestClientIPIgnoresForwardedForByDefault(t *testing.T) {
	withTrustedProxies(t, "") // explicit: no proxies trusted

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = "203.0.113.7:54321"
	r.Header.Set("X-Forwarded-For", "9.9.9.9")

	if got := clientIP(r); got != "203.0.113.7" {
		t.Errorf("clientIP = %q, want the raw connecting address 203.0.113.7 (X-Forwarded-For must be ignored)", got)
	}
}

func TestClientIPTrustsForwardedForFromTrustedProxy(t *testing.T) {
	withTrustedProxies(t, "203.0.113.7")

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = "203.0.113.7:54321" // the trusted proxy's own connection
	r.Header.Set("X-Forwarded-For", "198.51.100.42")

	if got := clientIP(r); got != "198.51.100.42" {
		t.Errorf("clientIP = %q, want 198.51.100.42 from X-Forwarded-For", got)
	}
}

func TestClientIPIgnoresForwardedForFromUntrustedAddress(t *testing.T) {
	withTrustedProxies(t, "10.0.0.0/8") // only trusts 10.x, not 203.x

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = "203.0.113.7:54321"
	r.Header.Set("X-Forwarded-For", "198.51.100.42")

	if got := clientIP(r); got != "203.0.113.7" {
		t.Errorf("clientIP = %q, want the raw address 203.0.113.7 (proxy not trusted)", got)
	}
}

func TestClientIPWalksChainOfTrustedProxies(t *testing.T) {
	withTrustedProxies(t, "203.0.113.7,203.0.113.8")

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = "203.0.113.7:54321" // the nearest (trusted) hop
	// Real client, then an intermediate trusted proxy, then the nearest trusted proxy.
	r.Header.Set("X-Forwarded-For", "198.51.100.42, 203.0.113.8")

	if got := clientIP(r); got != "198.51.100.42" {
		t.Errorf("clientIP = %q, want 198.51.100.42 (the first non-trusted entry from the right)", got)
	}
}

func TestClientIPFallsBackToRealIPHeader(t *testing.T) {
	withTrustedProxies(t, "203.0.113.7")

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = "203.0.113.7:54321"
	r.Header.Set("X-Real-IP", "198.51.100.99")

	if got := clientIP(r); got != "198.51.100.99" {
		t.Errorf("clientIP = %q, want 198.51.100.99 from X-Real-IP", got)
	}
}

func TestClientIPMalformedForwardedForFallsBackToRawAddress(t *testing.T) {
	withTrustedProxies(t, "203.0.113.7")

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = "203.0.113.7:54321"
	r.Header.Set("X-Forwarded-For", "not-an-ip, also-not-an-ip")

	if got := clientIP(r); got != "203.0.113.7" {
		t.Errorf("clientIP = %q, want the raw address 203.0.113.7 when X-Forwarded-For has no parseable IP", got)
	}
}
