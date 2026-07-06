package main

import (
	"net/http"
	"strconv"
	"strings"
	"time"
)

/* ===== Startseite / Umfrage erstellen ===== */

type indexPage struct {
	basePage
	CSRFToken       string
	DurationOptions []int
	DefaultDuration int
}

func (s *server) handleIndex(w http.ResponseWriter, r *http.Request) {
	maxDays := s.store.MaxDurationDays()
	s.render(w, http.StatusOK, "index.html", indexPage{
		basePage:        basePage{PageTitle: pageTitle("")},
		CSRFToken:       s.csrfToken(w, r),
		DurationOptions: daysRange(maxDays),
		DefaultDuration: maxDays,
	})
}

// daysRange returns [1, 2, ..., max], used to populate the poll duration
// <select> on the creation form.
func daysRange(max int) []int {
	days := make([]int, max)
	for i := range days {
		days[i] = i + 1
	}
	return days
}

// parseOptions reads the repeated "options[]" form field, trimming and
// length-clamping each entry, dropping blank ones, and capping the total
// count at maxOptions.
func parseOptions(r *http.Request) []string {
	var options []string
	for _, opt := range r.PostForm["options[]"] {
		opt = clamp(opt, maxOptionLen)
		if opt == "" {
			continue
		}
		options = append(options, opt)
		if len(options) >= maxOptions {
			break
		}
	}
	return options
}

func (s *server) handleCreate(w http.ResponseWriter, r *http.Request) {
	if !s.validCSRF(r) {
		http.Error(w, "Ungültiger CSRF-Token.", http.StatusForbidden)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Ungültige Anfrage.", http.StatusBadRequest)
		return
	}

	question := clamp(r.PostFormValue("question"), maxQuestionLen)
	if question == "" {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	options := parseOptions(r)
	if len(options) < 2 {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	durationDays, _ := strconv.Atoi(r.PostFormValue("duration_days"))

	passwordHash := ""
	if password := strings.TrimSpace(r.PostFormValue("password")); password != "" {
		hash, err := HashPassword(password)
		if err != nil {
			errorLog.Printf("hash poll password: %v", err)
			http.Error(w, "Umfrage konnte nicht erstellt werden.", http.StatusInternalServerError)
			return
		}
		passwordHash = hash
	}

	id, ownerToken, err := s.store.Create(question, options, passwordHash, durationDays)
	if err != nil {
		errorLog.Printf("create poll: %v", err)
		http.Error(w, "Umfrage konnte nicht erstellt werden.", http.StatusInternalServerError)
		return
	}

	poll, err := s.store.Load(id)
	if err != nil {
		errorLog.Printf("load created poll: %v", err)
		http.Error(w, "Umfrage konnte nicht erstellt werden.", http.StatusInternalServerError)
		return
	}

	s.setOwnerCookie(w, r, id, ownerToken, poll.ExpiresAt)
	http.Redirect(w, r, "/poll/"+id, http.StatusSeeOther)
}

/* ===== Umfrage entsperren (falls passwortgeschützt) ===== */

type lockedPage struct {
	basePage
	CSRFToken string
	ID        string
	Error     bool
}

func (s *server) handleUnlock(w http.ResponseWriter, r *http.Request) {
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

	if poll.PasswordHash == "" || VerifyPassword(poll.PasswordHash, r.PostFormValue("password")) {
		s.setUnlocked(w, r, id, poll.ExpiresAt)
		http.Redirect(w, r, "/poll/"+id, http.StatusSeeOther)
		return
	}

	s.limiter.fail(clientIP(r))

	s.render(w, http.StatusUnauthorized, "poll_locked.html", lockedPage{
		basePage:  basePage{PageTitle: pageTitle("")},
		CSRFToken: s.csrfToken(w, r),
		ID:        id,
		Error:     true,
	})
}

/* ===== Umfrage ansehen / abstimmen ===== */

type pollPage struct {
	basePage
	CSRFToken        string
	ID               string
	Poll             *Poll
	Results          []Result
	ParticipantCount int
	WinnerIndex      int
	HasPassword      bool
	CanManage        bool
	IsOwner          bool
	OwnerLink        string
	ExpiresAt        string
	DurationOptions  []int
	RemainingDays    int
}

func (s *server) handlePollView(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	poll, err := s.store.Load(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	if s.limiter.blocked(clientIP(r)) {
		writeRateLimitedHTML(w)
		return
	}

	ok := s.hasAccess(r, id, poll)
	if !ok && credentialSupplied(r) {
		s.limiter.fail(clientIP(r))
	}
	if !ok {
		s.render(w, http.StatusUnauthorized, "poll_locked.html", lockedPage{
			basePage:  basePage{PageTitle: pageTitle("")},
			CSRFToken: s.csrfToken(w, r),
			ID:        id,
		})
		return
	}

	results := Tally(poll)
	winnerIndex := -1
	if len(results) > 0 {
		winnerIndex = results[0].Index
	}

	isOwner := s.isOwner(r, id, poll)
	ownerLink := ""
	if isOwner {
		// Claim ownership for this browser too, so a shared admin link
		// only needs to be opened once - from then on the cookie carries
		// it, just like handleUnlock does for the poll password.
		s.setOwnerCookie(w, r, id, poll.OwnerToken, poll.ExpiresAt)
		ownerLink = "/poll/" + id + "?owner=" + poll.OwnerToken
	}

	maxDays := s.store.MaxDurationDays()
	remainingDays := int(time.Until(time.Unix(poll.ExpiresAt, 0)).Hours()/24) + 1
	if remainingDays < 1 {
		remainingDays = 1
	} else if remainingDays > maxDays {
		remainingDays = maxDays
	}

	s.render(w, http.StatusOK, "poll.html", pollPage{
		basePage:         basePage{PageTitle: pageTitle(poll.Question)},
		CSRFToken:        s.csrfToken(w, r),
		ID:               id,
		Poll:             poll,
		Results:          results,
		ParticipantCount: len(poll.Votes),
		WinnerIndex:      winnerIndex,
		HasPassword:      poll.PasswordHash != "",
		CanManage:        s.canManage(r, id, poll),
		IsOwner:          isOwner,
		OwnerLink:        ownerLink,
		ExpiresAt:        time.Unix(poll.ExpiresAt, 0).Format("02.01.2006 15:04"),
		DurationOptions:  daysRange(maxDays),
		RemainingDays:    remainingDays,
	})
}

func (s *server) handleVote(w http.ResponseWriter, r *http.Request) {
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

	ok := s.hasAccess(r, id, poll)
	if !ok && credentialSupplied(r) {
		s.limiter.fail(clientIP(r))
	}
	if !ok {
		http.Redirect(w, r, "/poll/"+id, http.StatusSeeOther)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Ungültige Anfrage.", http.StatusBadRequest)
		return
	}

	name := clamp(r.PostFormValue("name"), maxNameLen)
	votes := make([]int, len(poll.Options))
	comments := make([]string, len(poll.Options))

	for i := range poll.Options {
		key := "votes[" + strconv.Itoa(i) + "]"
		v, _ := strconv.Atoi(r.PostFormValue(key))
		if v < 0 {
			v = 0
		}
		if v > 4 {
			v = 4
		}
		votes[i] = v
		comments[i] = clamp(r.PostFormValue("comments["+strconv.Itoa(i)+"]"), maxCommentLen)
	}

	if err := s.store.AddVote(id, Vote{Name: name, Votes: votes, Comments: comments}); err != nil {
		errorLog.Printf("add vote: %v", err)
		http.Error(w, "Stimme konnte nicht gespeichert werden.", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/poll/"+id, http.StatusSeeOther)
}
