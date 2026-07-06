package main

import (
	"errors"
	"net/http"
	"os"
	"strconv"
	"strings"
)

/* ===== Umfrage verwalten (löschen, Passwort, Frage/Vorschläge, Laufzeit) ===== */

func (s *server) handleDelete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	if !s.validCSRF(r) {
		http.Error(w, "Ungültiger CSRF-Token.", http.StatusForbidden)
		return
	}

	poll, err := s.store.Load(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	if s.limiter.blocked(clientIP(r)) {
		writeRateLimitedHTML(w)
		return
	}

	ok := s.canManage(r, id, poll)
	if !ok && credentialSupplied(r) {
		s.limiter.fail(clientIP(r))
	}
	if !ok {
		http.Error(w, "Nur die Erstellerin/der Ersteller oder ein Admin kann diese Umfrage löschen.", http.StatusForbidden)
		return
	}

	if err := s.store.Delete(id); err != nil && !errors.Is(err, os.ErrNotExist) {
		errorLog.Printf("delete poll %s: %v", id, err)
	}
	s.statsCache.invalidate()

	if s.isAdmin(r) {
		http.Redirect(w, r, "/statistik", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// handleSetPassword lets the owner or a site admin add, change, or (with an
// empty password field) remove a poll's password after creation - e.g. to
// share a previously protected poll openly, or lock down one that wasn't
// password-protected at creation time.
func (s *server) handleSetPassword(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	if !s.validCSRF(r) {
		http.Error(w, "Ungültiger CSRF-Token.", http.StatusForbidden)
		return
	}

	poll, err := s.store.Load(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	if s.limiter.blocked(clientIP(r)) {
		writeRateLimitedHTML(w)
		return
	}

	ok := s.canManage(r, id, poll)
	if !ok && credentialSupplied(r) {
		s.limiter.fail(clientIP(r))
	}
	if !ok {
		http.Error(w, "Nur die Erstellerin/der Ersteller oder ein Admin kann das Passwort ändern.", http.StatusForbidden)
		return
	}

	passwordHash := ""
	if password := strings.TrimSpace(r.PostFormValue("password")); password != "" {
		hash, err := HashPassword(password)
		if err != nil {
			errorLog.Printf("hash poll password: %v", err)
			http.Error(w, "Passwort konnte nicht gespeichert werden.", http.StatusInternalServerError)
			return
		}
		passwordHash = hash
	}

	if err := s.store.SetPassword(id, passwordHash); err != nil {
		errorLog.Printf("set password poll %s: %v", id, err)
		http.Error(w, "Passwort konnte nicht gespeichert werden.", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/poll/"+id, http.StatusSeeOther)
}

// handleEditPoll lets the owner or a site admin correct a poll's question or
// its options after creation (e.g. to fix a typo). Invalid submissions
// (empty question, fewer than two options) are silently ignored, same as an
// invalid submission on the creation form.
func (s *server) handleEditPoll(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	if !s.validCSRF(r) {
		http.Error(w, "Ungültiger CSRF-Token.", http.StatusForbidden)
		return
	}

	poll, err := s.store.Load(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	if s.limiter.blocked(clientIP(r)) {
		writeRateLimitedHTML(w)
		return
	}

	ok := s.canManage(r, id, poll)
	if !ok && credentialSupplied(r) {
		s.limiter.fail(clientIP(r))
	}
	if !ok {
		http.Error(w, "Nur die Erstellerin/der Ersteller oder ein Admin kann diese Umfrage bearbeiten.", http.StatusForbidden)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Ungültige Anfrage.", http.StatusBadRequest)
		return
	}

	question := clamp(r.PostFormValue("question"), maxQuestionLen)
	options := parseOptions(r)
	if question == "" || len(options) < 2 {
		http.Redirect(w, r, "/poll/"+id, http.StatusSeeOther)
		return
	}

	if err := s.store.Update(id, question, options); err != nil {
		errorLog.Printf("update poll %s: %v", id, err)
		http.Error(w, "Umfrage konnte nicht gespeichert werden.", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/poll/"+id, http.StatusSeeOther)
}

// handleSetExpiry lets the owner or a site admin extend or shorten how many
// days (from now) a poll has left to run.
func (s *server) handleSetExpiry(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	if !s.validCSRF(r) {
		http.Error(w, "Ungültiger CSRF-Token.", http.StatusForbidden)
		return
	}

	poll, err := s.store.Load(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	if s.limiter.blocked(clientIP(r)) {
		writeRateLimitedHTML(w)
		return
	}

	ok := s.canManage(r, id, poll)
	if !ok && credentialSupplied(r) {
		s.limiter.fail(clientIP(r))
	}
	if !ok {
		http.Error(w, "Nur die Erstellerin/der Ersteller oder ein Admin kann die Laufzeit ändern.", http.StatusForbidden)
		return
	}

	days, err := strconv.Atoi(r.PostFormValue("duration_days"))
	if err != nil || days < 1 {
		http.Redirect(w, r, "/poll/"+id, http.StatusSeeOther)
		return
	}

	if err := s.store.SetExpiry(id, days); err != nil {
		errorLog.Printf("set expiry poll %s: %v", id, err)
		http.Error(w, "Laufzeit konnte nicht gespeichert werden.", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/poll/"+id, http.StatusSeeOther)
}
