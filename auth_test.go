package main

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func newTestServer(t *testing.T, adminPassword string) *server {
	t.Helper()
	secret, err := randomHex(32)
	if err != nil {
		t.Fatalf("randomHex: %v", err)
	}
	return &server{
		store:         NewStore(t.TempDir(), 7*24*time.Hour),
		templates:     loadTemplates(),
		secret:        []byte(secret),
		adminPassword: adminPassword,
		statsCache:    &statsCache{},
		limiter:       newRateLimiter(10, 5*time.Minute),
	}
}

func TestSubtleEqual(t *testing.T) {
	if !subtleEqual("abc", "abc") {
		t.Error("expected equal strings to compare equal")
	}
	if subtleEqual("abc", "abd") {
		t.Error("expected different strings to compare unequal")
	}
	if subtleEqual("abc", "abcd") {
		t.Error("expected different-length strings to compare unequal")
	}
}

func TestIsHTTPS(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	if isHTTPS(r) {
		t.Error("plain request should not be considered HTTPS")
	}

	r.TLS = &tls.ConnectionState{}
	if !isHTTPS(r) {
		t.Error("request with r.TLS set should be considered HTTPS")
	}

	r2 := httptest.NewRequest(http.MethodGet, "/", nil)
	r2.Header.Set("X-Forwarded-Proto", "https")
	if !isHTTPS(r2) {
		t.Error("X-Forwarded-Proto: https should be considered HTTPS")
	}
}

func TestMaxAgeUntil(t *testing.T) {
	future := time.Now().Add(time.Hour).Unix()
	if got := maxAgeUntil(future); got <= 0 || got > 3600 {
		t.Errorf("maxAgeUntil(future) = %d, want roughly 3600", got)
	}

	past := time.Now().Add(-time.Hour).Unix()
	if got := maxAgeUntil(past); got != 0 {
		t.Errorf("maxAgeUntil(past) = %d, want 0", got)
	}
}

func TestCSRFTokenAndValidCSRF(t *testing.T) {
	s := newTestServer(t, "")

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	token := s.csrfToken(w, r)
	if token == "" {
		t.Fatal("expected a non-empty CSRF token")
	}

	// Re-issuing the token for a request that already carries the cookie
	// must return the same value, not mint a new one.
	r2 := httptest.NewRequest(http.MethodGet, "/", nil)
	for _, c := range w.Result().Cookies() {
		r2.AddCookie(c)
	}
	if got := s.csrfToken(httptest.NewRecorder(), r2); got != token {
		t.Errorf("csrfToken on a request with an existing cookie returned %q, want %q", got, token)
	}

	valid := httptest.NewRequest(http.MethodPost, "/", nil)
	for _, c := range w.Result().Cookies() {
		valid.AddCookie(c)
	}
	valid.PostForm = map[string][]string{"token": {token}}
	if !s.validCSRF(valid) {
		t.Error("expected matching cookie+form token to be valid")
	}

	wrong := httptest.NewRequest(http.MethodPost, "/", nil)
	for _, c := range w.Result().Cookies() {
		wrong.AddCookie(c)
	}
	wrong.PostForm = map[string][]string{"token": {"something-else"}}
	if s.validCSRF(wrong) {
		t.Error("expected mismatched form token to be rejected")
	}

	noCookie := httptest.NewRequest(http.MethodPost, "/", nil)
	noCookie.PostForm = map[string][]string{"token": {token}}
	if s.validCSRF(noCookie) {
		t.Error("expected a request without the CSRF cookie to be rejected")
	}
}

func TestIsOwner(t *testing.T) {
	s := newTestServer(t, "")
	poll := &Poll{OwnerToken: "secret-owner-token"}

	cookieReq := httptest.NewRequest(http.MethodGet, "/", nil)
	cookieReq.AddCookie(&http.Cookie{Name: ownerCookieName("p1"), Value: "secret-owner-token"})
	if !s.isOwner(cookieReq, "p1", poll) {
		t.Error("expected the owner cookie to authenticate as owner")
	}

	headerReq := httptest.NewRequest(http.MethodGet, "/", nil)
	headerReq.Header.Set("X-Owner-Token", "secret-owner-token")
	if !s.isOwner(headerReq, "p1", poll) {
		t.Error("expected X-Owner-Token header to authenticate as owner")
	}

	queryReq := httptest.NewRequest(http.MethodGet, "/?owner=secret-owner-token", nil)
	if !s.isOwner(queryReq, "p1", poll) {
		t.Error("expected ?owner= query param to authenticate as owner")
	}

	wrongReq := httptest.NewRequest(http.MethodGet, "/", nil)
	wrongReq.AddCookie(&http.Cookie{Name: ownerCookieName("p1"), Value: "wrong-token"})
	if s.isOwner(wrongReq, "p1", poll) {
		t.Error("expected a wrong owner token to be rejected")
	}

	noTokenPoll := &Poll{OwnerToken: ""}
	if s.isOwner(cookieReq, "p1", noTokenPoll) {
		t.Error("a poll with no OwnerToken must never authenticate anyone as owner")
	}
}

func TestIsUnlocked(t *testing.T) {
	s := newTestServer(t, "")
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	s.setUnlocked(w, r, "p1", time.Now().Add(time.Hour).Unix())

	unlocked := httptest.NewRequest(http.MethodGet, "/", nil)
	for _, c := range w.Result().Cookies() {
		unlocked.AddCookie(c)
	}
	if !s.isUnlocked(unlocked, "p1") {
		t.Error("expected the unlock cookie to authenticate")
	}
	if s.isUnlocked(unlocked, "p2") {
		t.Error("an unlock cookie for one poll must not unlock a different poll")
	}

	noCookie := httptest.NewRequest(http.MethodGet, "/", nil)
	if s.isUnlocked(noCookie, "p1") {
		t.Error("expected no cookie to mean not unlocked")
	}
}

func TestIsAdminDisabledWithoutPassword(t *testing.T) {
	s := newTestServer(t, "")
	r := httptest.NewRequest(http.MethodGet, "/?adminPassword=whatever", nil)
	if s.isAdmin(r) {
		t.Error("isAdmin must always be false when no admin password is configured")
	}
}

func TestIsAdminViaHeaderAndQuery(t *testing.T) {
	s := newTestServer(t, "top-secret")

	headerReq := httptest.NewRequest(http.MethodGet, "/", nil)
	headerReq.Header.Set("X-Admin-Password", "top-secret")
	if !s.isAdmin(headerReq) {
		t.Error("expected correct X-Admin-Password header to authenticate")
	}

	queryReq := httptest.NewRequest(http.MethodGet, "/?adminPassword=top-secret", nil)
	if !s.isAdmin(queryReq) {
		t.Error("expected correct ?adminPassword= to authenticate")
	}

	wrongReq := httptest.NewRequest(http.MethodGet, "/?adminPassword=nope", nil)
	if s.isAdmin(wrongReq) {
		t.Error("expected wrong admin password to be rejected")
	}
}

func TestIsAdminSessionCookieExpiry(t *testing.T) {
	s := newTestServer(t, "top-secret")

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	s.setAdminSession(w, r)

	validReq := httptest.NewRequest(http.MethodGet, "/", nil)
	for _, c := range w.Result().Cookies() {
		validReq.AddCookie(c)
	}
	if !s.isAdmin(validReq) {
		t.Error("expected a freshly issued admin session cookie to authenticate")
	}

	// A cookie value whose embedded expiry has already passed must be
	// rejected server-side, even though the browser would normally have
	// dropped it on its own - this is what stops a captured cookie from
	// being replayed past its intended session lifetime.
	expiredValue := s.adminSessionValue(time.Now().Add(-time.Minute).Unix())
	expiredReq := httptest.NewRequest(http.MethodGet, "/", nil)
	expiredReq.AddCookie(&http.Cookie{Name: adminCookieName, Value: expiredValue})
	if s.isAdmin(expiredReq) {
		t.Error("expected an expired admin session value to be rejected")
	}

	// A tampered value (attacker-forged expiry, wrong signature) must fail.
	forgedReq := httptest.NewRequest(http.MethodGet, "/", nil)
	forgedReq.AddCookie(&http.Cookie{Name: adminCookieName, Value: "9999999999.deadbeef"})
	if s.isAdmin(forgedReq) {
		t.Error("expected a forged admin session value to be rejected")
	}

	clearW := httptest.NewRecorder()
	s.clearAdminSession(clearW, r)
	clearedReq := httptest.NewRequest(http.MethodGet, "/", nil)
	for _, c := range clearW.Result().Cookies() {
		clearedReq.AddCookie(c)
	}
	if s.isAdmin(clearedReq) {
		t.Error("expected a cleared admin session cookie to no longer authenticate")
	}
}

func TestHasAccessAndCanManage(t *testing.T) {
	s := newTestServer(t, "adminpw")
	hash, err := HashPassword("geheim")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	poll := &Poll{OwnerToken: "owner-tok", PasswordHash: hash}

	anon := httptest.NewRequest(http.MethodGet, "/", nil)
	if s.hasAccess(anon, "p1", poll) {
		t.Error("anonymous request to a password-protected poll should not have access")
	}
	if s.canManage(anon, "p1", poll) {
		t.Error("anonymous request should not be able to manage the poll")
	}

	owner := httptest.NewRequest(http.MethodGet, "/", nil)
	owner.AddCookie(&http.Cookie{Name: ownerCookieName("p1"), Value: "owner-tok"})
	if !s.hasAccess(owner, "p1", poll) {
		t.Error("owner should have access even without supplying the password")
	}
	if !s.canManage(owner, "p1", poll) {
		t.Error("owner should be able to manage the poll")
	}

	admin := httptest.NewRequest(http.MethodGet, "/?adminPassword=adminpw", nil)
	if !s.hasAccess(admin, "p1", poll) {
		t.Error("admin should have access")
	}
	if !s.canManage(admin, "p1", poll) {
		t.Error("admin should be able to manage the poll")
	}

	rightPassword := httptest.NewRequest(http.MethodGet, "/?password=geheim", nil)
	if !s.hasAccess(rightPassword, "p1", poll) {
		t.Error("the correct poll password should grant access")
	}
	if s.canManage(rightPassword, "p1", poll) {
		t.Error("knowing the poll password must never grant management rights")
	}

	unprotected := &Poll{OwnerToken: "owner-tok"}
	if !s.hasAccess(anon, "p1", unprotected) {
		t.Error("a poll without a password should be accessible to anyone")
	}
}
