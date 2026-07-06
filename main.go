package main

/*
 * KonsensOmat
 * Copyright (C) 2026 OpenKunde
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License, version 3.
 *
 * See <https://www.gnu.org/licenses/>.
 */

import (
	"embed"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

//go:embed templates/*.html
var templateFS embed.FS

//go:embed static
var staticFS embed.FS

// version is set at build time via -ldflags "-X main.version=..." (see
// Makefile); defaults to "dev" for plain `go run .`/`go build` invocations.
var version = "dev"

// infoLog carries normal operational messages (startup, requests) to
// stdout; errorLog carries failures and fatal conditions to stderr, so the
// two streams can be captured/filtered independently (e.g. by a container
// log driver or `2>/dev/null`).
var (
	infoLog  = log.New(os.Stdout, "", log.LstdFlags)
	errorLog = log.New(os.Stderr, "", log.LstdFlags)
)

func loadTemplates() map[string]*template.Template {
	pages := []string{
		"index.html",
		"poll.html",
		"poll_locked.html",
		"statistik.html",
		"info.html",
		"impressum.html",
		"datenschutz.html",
		"admin_login.html",
	}

	templates := make(map[string]*template.Template, len(pages))
	for _, page := range pages {
		templates[page] = template.Must(template.ParseFS(templateFS, "templates/layout.html", "templates/"+page))
	}

	return templates
}

// noCache wraps h so that every response tells the browser not to cache it,
// so a plain page reload during development always picks up rebuilt static
// assets instead of a stale cached copy.
func noCache(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		h.ServeHTTP(w, r)
	})
}

// statusRecorder wraps a ResponseWriter to capture the status code that was
// sent, so requestLog can report it after the handler has already written
// the response (http.ResponseWriter itself exposes no way to read it back).
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (rec *statusRecorder) WriteHeader(code int) {
	rec.status = code
	rec.ResponseWriter.WriteHeader(code)
}

// requestLog logs one line per request (client IP, method, path, status,
// duration) to stdout, so the application produces visible activity while
// it's running rather than only at startup or on error.
func requestLog(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		h.ServeHTTP(rec, r)
		infoLog.Printf("%s %s %s %d %s", clientIP(r), r.Method, r.URL.Path, rec.status, time.Since(start))
	})
}

// securityHeaders adds baseline hardening headers to every response. The
// CSP has no 'unsafe-inline' for scripts/styles - all templates avoid inline
// <script>/style="" attributes for exactly this reason - and frame-ancestors
// 'none' (backed by X-Frame-Options for older clients) blocks a poll or the
// admin page from being embedded in a clickjacking iframe.
func securityHeaders(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := w.Header()
		header.Set("X-Frame-Options", "DENY")
		header.Set("X-Content-Type-Options", "nosniff")
		header.Set("Referrer-Policy", "same-origin")
		header.Set("Content-Security-Policy",
			"default-src 'self'; base-uri 'none'; form-action 'self'; frame-ancestors 'none'; object-src 'none'")
		if isHTTPS(r) {
			header.Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains")
		}
		h.ServeHTTP(w, r)
	})
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envIntOr(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		errorLog.Printf("%s=%q ist keine gültige positive Ganzzahl, verwende %d", key, v, fallback)
		return fallback
	}
	return n
}

// maxExpiryDays is the hard ceiling for KONSENSOMAT_EXPIRY_DAYS, regardless
// of what's configured in the environment.
const maxExpiryDays = 365

func main() {
	if err := loadDotenv(".env"); err != nil {
		errorLog.Fatalf(".env: %v", err)
	}

	dataDir := envOr("KONSENSOMAT_DATA_DIR", "files/data")
	addr := envOr("KONSENSOMAT_ADDR", ":8080")
	expiryDays := envIntOr("KONSENSOMAT_EXPIRY_DAYS", 7)
	if expiryDays > maxExpiryDays {
		errorLog.Printf("KONSENSOMAT_EXPIRY_DAYS=%d überschreitet das Maximum, verwende %d", expiryDays, maxExpiryDays)
		expiryDays = maxExpiryDays
	}

	proxies, err := parseTrustedProxies(envOr("KONSENSOMAT_TRUSTED_PROXIES", ""))
	if err != nil {
		errorLog.Fatalf("KONSENSOMAT_TRUSTED_PROXIES: %v", err)
	}
	trustedProxies = proxies

	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		errorLog.Fatalf("data dir %q: %v", dataDir, err)
	}

	secret, err := randomHex(32)
	if err != nil {
		errorLog.Fatalf("secret generation: %v", err)
	}

	s := &server{
		store:         NewStore(dataDir, time.Duration(expiryDays)*24*time.Hour),
		templates:     loadTemplates(),
		secret:        []byte(secret),
		adminPassword: os.Getenv("KONSENSOMAT_ADMIN_PASSWORD"),
		statsCache:    &statsCache{},
		limiter:       newRateLimiter(10, 5*time.Minute),
	}

	static, err := fs.Sub(staticFS, "static")
	if err != nil {
		errorLog.Fatal(err)
	}

	mux := http.NewServeMux()
	mux.Handle("GET /static/", http.StripPrefix("/static/", noCache(http.FileServerFS(static))))

	mux.HandleFunc("GET /{$}", s.handleIndex)
	mux.HandleFunc("POST /create", s.handleCreate)
	mux.HandleFunc("GET /poll/{id}", s.handlePollView)
	mux.HandleFunc("POST /poll/{id}/vote", s.handleVote)
	mux.HandleFunc("POST /poll/{id}/delete", s.handleDelete)
	mux.HandleFunc("POST /poll/{id}/edit", s.handleEditPoll)
	mux.HandleFunc("POST /poll/{id}/expiry", s.handleSetExpiry)
	mux.HandleFunc("POST /poll/{id}/password", s.handleSetPassword)
	mux.HandleFunc("POST /poll/{id}/unlock", s.handleUnlock)
	mux.HandleFunc("GET /info", s.handleInfo)
	mux.HandleFunc("GET /impressum", s.handleImpressum)
	mux.HandleFunc("GET /datenschutz", s.handleDatenschutz)
	mux.HandleFunc("GET /statistik", s.handleStatistik)
	mux.HandleFunc("GET /admin", s.handleAdminLoginForm)
	mux.HandleFunc("POST /admin/login", s.handleAdminLogin)
	mux.HandleFunc("POST /admin/logout", s.handleAdminLogout)

	mux.HandleFunc("POST /api/polls", s.apiCreatePoll)
	mux.HandleFunc("GET /api/polls/{id}", s.apiGetPoll)
	mux.HandleFunc("POST /api/polls/{id}/votes", s.apiVote)
	mux.HandleFunc("DELETE /api/polls/{id}", s.apiDeletePoll)
	mux.HandleFunc("GET /api/stats", s.apiStats)
	mux.HandleFunc("OPTIONS /api/polls", s.apiPreflight)
	mux.HandleFunc("OPTIONS /api/polls/{id}", s.apiPreflight)
	mux.HandleFunc("OPTIONS /api/polls/{id}/votes", s.apiPreflight)
	mux.HandleFunc("OPTIONS /api/stats", s.apiPreflight)

	infoLog.Printf("KonsensOmat %s läuft auf %s (Datenverzeichnis: %s)", version, displayURL(addr), dataDir)
	errorLog.Fatal(http.ListenAndServe(addr, requestLog(securityHeaders(mux))))
}

// displayURL turns an addr suitable for http.ListenAndServe (e.g. ":8080" or
// "0.0.0.0:8000") into a full URL for logging, so the terminal shows a link
// that's actually clickable/copyable instead of a bare port.
func displayURL(addr string) string {
	if strings.HasPrefix(addr, ":") {
		return "http://localhost" + addr
	}
	return "http://" + addr
}
