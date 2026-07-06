package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

// The JSON API mirrors the actions available in the web UI (create, view,
// vote, delete, stats). As in the HTML forms, a poll's creator authenticates
// via an owner token (returned once, on creation) that is the only way to
// delete it (besides a site admin); an optional password additionally gates
// viewing/voting for everyone else, via the X-Poll-Password header or a
// "password" query parameter, but never grants deletion. A site admin (if
// KONSENSOMAT_ADMIN_PASSWORD is configured) can view and delete any poll via
// X-Admin-Password or ?adminPassword=. There is deliberately no "list all
// polls" endpoint, since polls are only meant to be reachable via their
// unlisted id/link.

type pollResponse struct {
	ID               string           `json:"id"`
	Question         string           `json:"question"`
	Options          []string         `json:"options"`
	Created          int64            `json:"created"`
	ExpiresAt        int64            `json:"expiresAt"`
	ParticipantCount int              `json:"participantCount"`
	WinnerIndex      int              `json:"winnerIndex"`
	HasPassword      bool             `json:"hasPassword"`
	Results          []resultResponse `json:"results"`
}

type createPollResponse struct {
	pollResponse
	OwnerToken string `json:"ownerToken"`
}

type resultResponse struct {
	Index    int      `json:"index"`
	Text     string   `json:"text"`
	Total    int      `json:"total"`
	Comments []string `json:"comments"`
}

func pollToResponse(id string, p *Poll) pollResponse {
	results := Tally(p)

	winner := 0
	if len(results) > 0 {
		winner = results[0].Index
	}

	responseResults := make([]resultResponse, len(results))
	for i, res := range results {
		comments := res.Comments
		if comments == nil {
			comments = []string{}
		}
		responseResults[i] = resultResponse{Index: res.Index, Text: res.Text, Total: res.Total, Comments: comments}
	}

	return pollResponse{
		ID:               id,
		Question:         p.Question,
		Options:          p.Options,
		Created:          p.Created,
		ExpiresAt:        p.ExpiresAt,
		ParticipantCount: len(p.Votes),
		WinnerIndex:      winner,
		HasPassword:      p.PasswordHash != "",
		Results:          responseResults,
	}
}

type createPollRequest struct {
	Question     string   `json:"question"`
	Options      []string `json:"options"`
	Password     string   `json:"password"`
	DurationDays int      `json:"durationDays"`
}

type voteRequest struct {
	Name     string   `json:"name"`
	Votes    []int    `json:"votes"`
	Comments []string `json:"comments"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		errorLog.Printf("write json: %v", err)
	}
}

func writeJSONError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func decodeJSONBody(r *http.Request, v any) error {
	defer r.Body.Close()
	dec := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}

func corsHeaders(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
}

func (s *server) apiPreflight(w http.ResponseWriter, r *http.Request) {
	corsHeaders(w)
	w.WriteHeader(http.StatusNoContent)
}

func (s *server) apiCreatePoll(w http.ResponseWriter, r *http.Request) {
	corsHeaders(w)

	var req createPollRequest
	if err := decodeJSONBody(r, &req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "Ungültiges JSON: "+err.Error())
		return
	}

	question := strings.TrimSpace(req.Question)
	if question == "" {
		writeJSONError(w, http.StatusBadRequest, "question darf nicht leer sein")
		return
	}
	if len(question) > maxQuestionLen {
		writeJSONError(w, http.StatusBadRequest, fmt.Sprintf("question darf höchstens %d Zeichen lang sein", maxQuestionLen))
		return
	}

	var options []string
	for _, opt := range req.Options {
		opt = strings.TrimSpace(opt)
		if opt == "" {
			continue
		}
		if len(opt) > maxOptionLen {
			writeJSONError(w, http.StatusBadRequest, fmt.Sprintf("eine Option darf höchstens %d Zeichen lang sein", maxOptionLen))
			return
		}
		options = append(options, opt)
	}
	if len(options) < 2 {
		writeJSONError(w, http.StatusBadRequest, "es sind mindestens 2 Optionen erforderlich")
		return
	}
	if len(options) > maxOptions {
		writeJSONError(w, http.StatusBadRequest, fmt.Sprintf("es sind höchstens %d Optionen erlaubt", maxOptions))
		return
	}

	maxDays := s.store.MaxDurationDays()
	if req.DurationDays != 0 && (req.DurationDays < 1 || req.DurationDays > maxDays) {
		writeJSONError(w, http.StatusBadRequest, fmt.Sprintf("durationDays muss zwischen 1 und %d liegen", maxDays))
		return
	}

	passwordHash := ""
	if req.Password != "" {
		hash, err := HashPassword(req.Password)
		if err != nil {
			errorLog.Printf("api hash poll password: %v", err)
			writeJSONError(w, http.StatusInternalServerError, "Umfrage konnte nicht erstellt werden")
			return
		}
		passwordHash = hash
	}

	id, ownerToken, err := s.store.Create(question, options, passwordHash, req.DurationDays)
	if err != nil {
		errorLog.Printf("api create poll: %v", err)
		writeJSONError(w, http.StatusInternalServerError, "Umfrage konnte nicht erstellt werden")
		return
	}

	poll, err := s.store.Load(id)
	if err != nil {
		errorLog.Printf("api load created poll: %v", err)
		writeJSONError(w, http.StatusInternalServerError, "Umfrage konnte nicht geladen werden")
		return
	}

	writeJSON(w, http.StatusCreated, createPollResponse{
		pollResponse: pollToResponse(id, poll),
		OwnerToken:   ownerToken,
	})
}

func (s *server) apiGetPoll(w http.ResponseWriter, r *http.Request) {
	corsHeaders(w)

	id := r.PathValue("id")
	poll, err := s.store.Load(id)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, "Umfrage nicht gefunden")
		return
	}

	if s.limiter.blocked(clientIP(r)) {
		writeRateLimitedJSON(w)
		return
	}

	ok := s.hasAccess(r, id, poll)
	if !ok && credentialSupplied(r) {
		s.limiter.fail(clientIP(r))
	}
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "Diese Umfrage ist passwortgeschützt")
		return
	}

	writeJSON(w, http.StatusOK, pollToResponse(id, poll))
}

func (s *server) apiVote(w http.ResponseWriter, r *http.Request) {
	corsHeaders(w)

	id := r.PathValue("id")
	poll, err := s.store.Load(id)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, "Umfrage nicht gefunden")
		return
	}

	if s.limiter.blocked(clientIP(r)) {
		writeRateLimitedJSON(w)
		return
	}

	ok := s.hasAccess(r, id, poll)
	if !ok && credentialSupplied(r) {
		s.limiter.fail(clientIP(r))
	}
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "Diese Umfrage ist passwortgeschützt")
		return
	}

	var req voteRequest
	if err := decodeJSONBody(r, &req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "Ungültiges JSON: "+err.Error())
		return
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		writeJSONError(w, http.StatusBadRequest, "name darf nicht leer sein")
		return
	}
	if len(name) > maxNameLen {
		writeJSONError(w, http.StatusBadRequest, fmt.Sprintf("name darf höchstens %d Zeichen lang sein", maxNameLen))
		return
	}

	if len(req.Votes) != len(poll.Options) {
		writeJSONError(w, http.StatusBadRequest, fmt.Sprintf("votes muss genau %d Werte enthalten (einen pro Option)", len(poll.Options)))
		return
	}
	for _, v := range req.Votes {
		if v < 0 || v > 4 {
			writeJSONError(w, http.StatusBadRequest, "votes-Werte müssen zwischen 0 und 4 liegen")
			return
		}
	}
	if len(req.Comments) > len(poll.Options) {
		writeJSONError(w, http.StatusBadRequest, fmt.Sprintf("comments darf höchstens %d Werte enthalten", len(poll.Options)))
		return
	}

	for _, c := range req.Comments {
		if len(c) > maxCommentLen {
			writeJSONError(w, http.StatusBadRequest, fmt.Sprintf("comments dürfen höchstens %d Zeichen lang sein", maxCommentLen))
			return
		}
	}

	votes := append([]int(nil), req.Votes...)
	comments := make([]string, len(poll.Options))
	for i := range req.Comments {
		comments[i] = strings.TrimSpace(req.Comments[i])
	}

	if err := s.store.AddVote(id, Vote{Name: name, Votes: votes, Comments: comments}); err != nil {
		errorLog.Printf("api vote: %v", err)
		writeJSONError(w, http.StatusInternalServerError, "Stimme konnte nicht gespeichert werden")
		return
	}

	updated, err := s.store.Load(id)
	if err != nil {
		errorLog.Printf("api load poll after vote: %v", err)
		writeJSONError(w, http.StatusInternalServerError, "Umfrage konnte nicht geladen werden")
		return
	}

	writeJSON(w, http.StatusCreated, pollToResponse(id, updated))
}

func (s *server) apiDeletePoll(w http.ResponseWriter, r *http.Request) {
	corsHeaders(w)

	id := r.PathValue("id")
	poll, err := s.store.Load(id)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, "Umfrage nicht gefunden")
		return
	}

	if s.limiter.blocked(clientIP(r)) {
		writeRateLimitedJSON(w)
		return
	}

	ok := s.canManage(r, id, poll)
	if !ok && credentialSupplied(r) {
		s.limiter.fail(clientIP(r))
	}
	if !ok {
		writeJSONError(w, http.StatusForbidden, "nur die Erstellerin/der Ersteller oder ein Admin kann diese Umfrage löschen")
		return
	}

	if err := s.store.Delete(id); err != nil && !errors.Is(err, os.ErrNotExist) {
		errorLog.Printf("api delete poll %s: %v", id, err)
		writeJSONError(w, http.StatusInternalServerError, "Umfrage konnte nicht gelöscht werden")
		return
	}
	s.statsCache.invalidate()

	w.WriteHeader(http.StatusNoContent)
}

func (s *server) apiStats(w http.ResponseWriter, r *http.Request) {
	corsHeaders(w)

	st, _, _, err := s.statsCache.get(s.store.dir)
	if err != nil {
		errorLog.Printf("api stats: %v", err)
		writeJSONError(w, http.StatusInternalServerError, "Statistik konnte nicht ermittelt werden")
		return
	}

	writeJSON(w, http.StatusOK, st)
}
