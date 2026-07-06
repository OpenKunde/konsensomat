package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func subtleEqual(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

// isHTTPS reports whether the request reached us over HTTPS, either directly
// or (the expected deployment: this app never terminates TLS itself) via a
// reverse proxy that sets the standard X-Forwarded-Proto header. Used to
// decide whether cookies can safely carry the Secure attribute: hardcoding
// Secure=true would break plain-HTTP local development, and hardcoding
// Secure=false would send session cookies in the clear in production.
func isHTTPS(r *http.Request) bool {
	return r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
}

// maxAgeUntil returns the number of seconds from now until the given unix
// timestamp, floored at 0.
func maxAgeUntil(unixTime int64) int {
	remaining := time.Until(time.Unix(unixTime, 0))
	if remaining < 0 {
		return 0
	}
	return int(remaining.Seconds())
}

func ownerCookieName(id string) string {
	return "owner_" + id
}

// setOwnerCookie marks the current browser as the poll's creator, letting it
// delete the poll later without needing the password. The cookie outlives
// the poll itself by exactly as long as the poll has left to run.
func (s *server) setOwnerCookie(w http.ResponseWriter, r *http.Request, id, ownerToken string, expiresAt int64) {
	http.SetCookie(w, &http.Cookie{
		Name:     ownerCookieName(id),
		Value:    ownerToken,
		Path:     "/",
		HttpOnly: true,
		Secure:   isHTTPS(r),
		SameSite: http.SameSiteLaxMode,
		MaxAge:   maxAgeUntil(expiresAt),
	})
}

// isOwner reports whether the request authenticates as the poll's creator,
// either via the browser cookie set at creation time or, for API clients,
// via an explicit owner token passed in a header or query parameter.
func (s *server) isOwner(r *http.Request, id string, poll *Poll) bool {
	if poll.OwnerToken == "" {
		return false
	}
	if c, err := r.Cookie(ownerCookieName(id)); err == nil && subtleEqual(c.Value, poll.OwnerToken) {
		return true
	}
	if tok := r.Header.Get("X-Owner-Token"); tok != "" && subtleEqual(tok, poll.OwnerToken) {
		return true
	}
	if tok := r.URL.Query().Get("owner"); tok != "" && subtleEqual(tok, poll.OwnerToken) {
		return true
	}
	return false
}

// pollAuthValue derives the value that unlocks poll id for the lifetime of
// the server process, from the server's in-memory secret.
func (s *server) pollAuthValue(id string) string {
	mac := hmac.New(sha256.New, s.secret)
	mac.Write([]byte(id))
	return hex.EncodeToString(mac.Sum(nil))
}

func unlockCookieName(id string) string {
	return "unlock_" + id
}

// setUnlocked remembers, for this browser, that the correct password for
// poll id was supplied.
func (s *server) setUnlocked(w http.ResponseWriter, r *http.Request, id string, expiresAt int64) {
	http.SetCookie(w, &http.Cookie{
		Name:     unlockCookieName(id),
		Value:    s.pollAuthValue(id),
		Path:     "/",
		HttpOnly: true,
		Secure:   isHTTPS(r),
		SameSite: http.SameSiteLaxMode,
		MaxAge:   maxAgeUntil(expiresAt),
	})
}

func (s *server) isUnlocked(r *http.Request, id string) bool {
	c, err := r.Cookie(unlockCookieName(id))
	if err != nil {
		return false
	}
	return subtleEqual(c.Value, s.pollAuthValue(id))
}

// suppliedPassword extracts a password passed directly on the request (API
// clients only; the HTML flow instead unlocks once via handleUnlock and
// relies on the cookie from setUnlocked).
func suppliedPassword(r *http.Request) string {
	if v := r.Header.Get("X-Poll-Password"); v != "" {
		return v
	}
	return r.URL.Query().Get("password")
}

const adminCookieName = "admin_session"
const adminSessionDuration = 12 * time.Hour

// adminSessionValue derives the admin-session cookie value from the
// server's in-memory secret and an expiry timestamp, in the form
// "<expiresAtUnix>.<hmac>". Binding the expiry into the signed value itself
// (rather than relying solely on the cookie's MaxAge) means a captured
// cookie can't be replayed past its session lifetime just by resending it
// outside the browser that would normally drop it - isAdmin checks the
// timestamp server-side, too.
func (s *server) adminSessionValue(expiresAt int64) string {
	mac := hmac.New(sha256.New, s.secret)
	mac.Write([]byte("admin"))
	mac.Write([]byte(strconv.FormatInt(expiresAt, 10)))
	return strconv.FormatInt(expiresAt, 10) + "." + hex.EncodeToString(mac.Sum(nil))
}

// validAdminSession reports whether value is a well-formed, unexpired,
// correctly-signed admin session cookie value.
func (s *server) validAdminSession(value string) bool {
	expiresPart, _, ok := strings.Cut(value, ".")
	if !ok {
		return false
	}
	expiresAt, err := strconv.ParseInt(expiresPart, 10, 64)
	if err != nil || time.Now().Unix() > expiresAt {
		return false
	}
	return subtleEqual(value, s.adminSessionValue(expiresAt))
}

func (s *server) setAdminSession(w http.ResponseWriter, r *http.Request) {
	expiresAt := time.Now().Add(adminSessionDuration).Unix()
	http.SetCookie(w, &http.Cookie{
		Name:     adminCookieName,
		Value:    s.adminSessionValue(expiresAt),
		Path:     "/",
		HttpOnly: true,
		Secure:   isHTTPS(r),
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(adminSessionDuration.Seconds()),
	})
}

func (s *server) clearAdminSession(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     adminCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   isHTTPS(r),
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}

// isAdmin reports whether the request authenticates as a site admin: either
// via the cookie set by handleAdminLogin, or, for API clients, via the admin
// password passed directly in a header or query parameter. Admins can view
// and delete any poll, protected or not - this is the moderation escape
// hatch for a tool that otherwise has no accounts. Disabled entirely (always
// false) unless KONSENSOMAT_ADMIN_PASSWORD is configured.
func (s *server) isAdmin(r *http.Request) bool {
	if s.adminPassword == "" {
		return false
	}
	if c, err := r.Cookie(adminCookieName); err == nil && s.validAdminSession(c.Value) {
		return true
	}
	if pw := r.Header.Get("X-Admin-Password"); pw != "" && subtleEqual(pw, s.adminPassword) {
		return true
	}
	if pw := r.URL.Query().Get("adminPassword"); pw != "" && subtleEqual(pw, s.adminPassword) {
		return true
	}
	return false
}

// hasAccess reports whether the request may view/vote on poll id: always
// true for unprotected polls, otherwise true for the owner, an admin, a
// browser that already unlocked it, or a request presenting the correct
// password.
func (s *server) hasAccess(r *http.Request, id string, poll *Poll) bool {
	if poll.PasswordHash == "" {
		return true
	}
	if s.isOwner(r, id, poll) || s.isAdmin(r) || s.isUnlocked(r, id) {
		return true
	}
	if pw := suppliedPassword(r); pw != "" && VerifyPassword(poll.PasswordHash, pw) {
		return true
	}
	return false
}

// canManage reports whether the request may manage poll id - delete it or
// change its password. Only the poll's creator (via the owner cookie/token)
// or a site admin may do so - knowing a poll's viewing password is not
// enough, since that password is routinely shared with everyone invited to
// vote and must not double as management rights.
func (s *server) canManage(r *http.Request, id string, poll *Poll) bool {
	return s.isOwner(r, id, poll) || s.isAdmin(r)
}
