package main

import (
	"html/template"
	"net/http"
)

const csrfCookieName = "csrf_token"

// server holds everything shared across requests: the poll store, parsed
// templates, and the small pieces of state (secret, admin password, caches,
// rate limiter) that the handlers in handlers_*.go depend on.
type server struct {
	store     *Store
	templates map[string]*template.Template
	// secret is an in-memory, per-process key used to derive the "this
	// browser already entered the correct password/is an admin" cookies. It
	// does not need to survive restarts: on restart, everyone simply has to
	// unlock password-protected polls (or log back in as admin) again.
	secret []byte
	// adminPassword, if set (via KONSENSOMAT_ADMIN_PASSWORD), lets whoever
	// knows it view and delete any poll regardless of its own password. An
	// empty string disables the admin feature entirely.
	adminPassword string
	// statsCache memoizes the (unauthenticated, disk-scanning) stats
	// computation shared by /statistik and /api/stats.
	statsCache *statsCache
	// limiter throttles repeated failed authentication attempts (wrong
	// admin/poll password) per client IP.
	limiter *rateLimiter
}

// csrfToken returns the token for the current session, creating and setting
// a cookie for it if none exists yet.
func (s *server) csrfToken(w http.ResponseWriter, r *http.Request) string {
	if c, err := r.Cookie(csrfCookieName); err == nil && c.Value != "" {
		return c.Value
	}

	token, err := randomHex(32)
	if err != nil {
		errorLog.Printf("csrf token generation failed: %v", err)
		return ""
	}

	http.SetCookie(w, &http.Cookie{
		Name:     csrfCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   isHTTPS(r),
		SameSite: http.SameSiteLaxMode,
	})

	return token
}

func (s *server) validCSRF(r *http.Request) bool {
	c, err := r.Cookie(csrfCookieName)
	if err != nil || c.Value == "" {
		return false
	}
	return subtleEqual(c.Value, r.PostFormValue("token"))
}

// render executes the named template with data, writing status as the
// response's HTTP status code.
func (s *server) render(w http.ResponseWriter, status int, name string, data any) {
	tmpl, ok := s.templates[name]
	if !ok {
		http.Error(w, "template not found", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if err := tmpl.ExecuteTemplate(w, "layout.html", data); err != nil {
		errorLog.Printf("render %s: %v", name, err)
	}
}

// basePage is embedded by every page's template data to supply the <title>.
type basePage struct {
	PageTitle string
}

func pageTitle(suffix string) string {
	if suffix == "" {
		return "🤖 KonsensOmat"
	}
	return "🤖 " + suffix + " – KonsensOmat"
}
