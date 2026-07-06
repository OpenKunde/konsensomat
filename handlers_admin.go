package main

import "net/http"

/* ===== Admin-Login ===== */

type adminLoginPage struct {
	basePage
	CSRFToken string
	Error     bool
}

func (s *server) handleAdminLoginForm(w http.ResponseWriter, r *http.Request) {
	s.render(w, http.StatusOK, "admin_login.html", adminLoginPage{
		basePage:  basePage{PageTitle: pageTitle("Admin-Login")},
		CSRFToken: s.csrfToken(w, r),
	})
}

func (s *server) handleAdminLogin(w http.ResponseWriter, r *http.Request) {
	if !s.validCSRF(r) {
		http.Error(w, "Ungültiger CSRF-Token.", http.StatusForbidden)
		return
	}

	if s.limiter.blocked(clientIP(r)) {
		writeRateLimitedHTML(w)
		return
	}

	if s.adminPassword != "" && subtleEqual(r.PostFormValue("password"), s.adminPassword) {
		s.setAdminSession(w, r)
		http.Redirect(w, r, "/statistik", http.StatusSeeOther)
		return
	}

	s.limiter.fail(clientIP(r))

	s.render(w, http.StatusUnauthorized, "admin_login.html", adminLoginPage{
		basePage:  basePage{PageTitle: pageTitle("Admin-Login")},
		CSRFToken: s.csrfToken(w, r),
		Error:     true,
	})
}

func (s *server) handleAdminLogout(w http.ResponseWriter, r *http.Request) {
	if !s.validCSRF(r) {
		http.Error(w, "Ungültiger CSRF-Token.", http.StatusForbidden)
		return
	}

	s.clearAdminSession(w, r)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}
