package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newJSONRequest(method, target string, body any) *http.Request {
	var r *http.Request
	if body == nil {
		r = httptest.NewRequest(method, target, nil)
	} else {
		buf, _ := json.Marshal(body)
		r = httptest.NewRequest(method, target, bytes.NewReader(buf))
	}
	r.Header.Set("Content-Type", "application/json")
	return r
}

func TestAPICreatePoll(t *testing.T) {
	s := newTestServer(t, "")

	t.Run("valid request creates a poll and returns an owner token", func(t *testing.T) {
		r := newJSONRequest(http.MethodPost, "/api/polls", createPollRequest{
			Question: "Wohin geht die Firmenfeier?",
			Options:  []string{"Strand", "Berge"},
		})
		w := httptest.NewRecorder()
		s.apiCreatePoll(w, r)

		if w.Code != http.StatusCreated {
			t.Fatalf("status = %d, want 201, body=%s", w.Code, w.Body.String())
		}
		var resp createPollResponse
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if resp.OwnerToken == "" || resp.ID == "" {
			t.Errorf("expected non-empty id/ownerToken, got %#v", resp)
		}
		if resp.Question != "Wohin geht die Firmenfeier?" {
			t.Errorf("Question = %q", resp.Question)
		}
	})

	t.Run("empty question is rejected", func(t *testing.T) {
		r := newJSONRequest(http.MethodPost, "/api/polls", createPollRequest{
			Question: "",
			Options:  []string{"A", "B"},
		})
		w := httptest.NewRecorder()
		s.apiCreatePoll(w, r)
		if w.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", w.Code)
		}
	})

	t.Run("fewer than two options is rejected", func(t *testing.T) {
		r := newJSONRequest(http.MethodPost, "/api/polls", createPollRequest{
			Question: "Q",
			Options:  []string{"only one"},
		})
		w := httptest.NewRecorder()
		s.apiCreatePoll(w, r)
		if w.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", w.Code)
		}
	})

	t.Run("out-of-range durationDays is rejected", func(t *testing.T) {
		r := newJSONRequest(http.MethodPost, "/api/polls", createPollRequest{
			Question:     "Q",
			Options:      []string{"A", "B"},
			DurationDays: 9999,
		})
		w := httptest.NewRecorder()
		s.apiCreatePoll(w, r)
		if w.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", w.Code)
		}
	})

	t.Run("malformed JSON is rejected", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodPost, "/api/polls", strings.NewReader("{not json"))
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		s.apiCreatePoll(w, r)
		if w.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", w.Code)
		}
	})

	t.Run("CORS headers are always set", func(t *testing.T) {
		r := newJSONRequest(http.MethodPost, "/api/polls", createPollRequest{Question: "Q", Options: []string{"A", "B"}})
		w := httptest.NewRecorder()
		s.apiCreatePoll(w, r)
		if got := w.Header().Get("Access-Control-Allow-Origin"); got != "*" {
			t.Errorf("Access-Control-Allow-Origin = %q, want *", got)
		}
	})
}

func TestAPIGetPoll(t *testing.T) {
	s := newTestServer(t, "")

	t.Run("unknown id returns 404", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/api/polls/0123456789", nil)
		r.SetPathValue("id", "0123456789")
		w := httptest.NewRecorder()
		s.apiGetPoll(w, r)
		if w.Code != http.StatusNotFound {
			t.Errorf("status = %d, want 404", w.Code)
		}
	})

	t.Run("password-protected poll requires the password", func(t *testing.T) {
		hash, _ := HashPassword("geheim")
		id, _, _ := s.store.Create("Q", []string{"A", "B"}, hash, 0)

		noAuth := httptest.NewRequest(http.MethodGet, "/api/polls/"+id, nil)
		noAuth.SetPathValue("id", id)
		w := httptest.NewRecorder()
		s.apiGetPoll(w, noAuth)
		if w.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want 401", w.Code)
		}

		withAuth := httptest.NewRequest(http.MethodGet, "/api/polls/"+id+"?password=geheim", nil)
		withAuth.SetPathValue("id", id)
		w2 := httptest.NewRecorder()
		s.apiGetPoll(w2, withAuth)
		if w2.Code != http.StatusOK {
			t.Errorf("status = %d, want 200", w2.Code)
		}
	})
}

func TestAPIVote(t *testing.T) {
	s := newTestServer(t, "")
	id, _, _ := s.store.Create("Q", []string{"A", "B"}, "", 0)

	t.Run("valid vote is recorded", func(t *testing.T) {
		r := newJSONRequest(http.MethodPost, "/api/polls/"+id+"/votes", voteRequest{
			Name:  "Alice",
			Votes: []int{0, 4},
		})
		r.SetPathValue("id", id)
		w := httptest.NewRecorder()
		s.apiVote(w, r)
		if w.Code != http.StatusCreated {
			t.Fatalf("status = %d, want 201, body=%s", w.Code, w.Body.String())
		}
	})

	t.Run("wrong number of votes is rejected", func(t *testing.T) {
		r := newJSONRequest(http.MethodPost, "/api/polls/"+id+"/votes", voteRequest{
			Name:  "Bob",
			Votes: []int{0, 1, 2},
		})
		r.SetPathValue("id", id)
		w := httptest.NewRecorder()
		s.apiVote(w, r)
		if w.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", w.Code)
		}
	})

	t.Run("out-of-range vote value is rejected", func(t *testing.T) {
		r := newJSONRequest(http.MethodPost, "/api/polls/"+id+"/votes", voteRequest{
			Name:  "Bob",
			Votes: []int{0, 5},
		})
		r.SetPathValue("id", id)
		w := httptest.NewRecorder()
		s.apiVote(w, r)
		if w.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", w.Code)
		}
	})

	t.Run("empty name is rejected", func(t *testing.T) {
		r := newJSONRequest(http.MethodPost, "/api/polls/"+id+"/votes", voteRequest{
			Name:  "  ",
			Votes: []int{0, 1},
		})
		r.SetPathValue("id", id)
		w := httptest.NewRecorder()
		s.apiVote(w, r)
		if w.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", w.Code)
		}
	})
}

func TestAPIDeletePoll(t *testing.T) {
	s := newTestServer(t, "adminpw")

	t.Run("without owner token or admin password is forbidden", func(t *testing.T) {
		id, _, _ := s.store.Create("Q", []string{"A", "B"}, "", 0)
		r := httptest.NewRequest(http.MethodDelete, "/api/polls/"+id, nil)
		r.SetPathValue("id", id)
		w := httptest.NewRecorder()
		s.apiDeletePoll(w, r)
		if w.Code != http.StatusForbidden {
			t.Errorf("status = %d, want 403", w.Code)
		}
	})

	t.Run("with the correct owner token succeeds", func(t *testing.T) {
		id, ownerToken, _ := s.store.Create("Q", []string{"A", "B"}, "", 0)
		r := httptest.NewRequest(http.MethodDelete, "/api/polls/"+id+"?owner="+ownerToken, nil)
		r.SetPathValue("id", id)
		w := httptest.NewRecorder()
		s.apiDeletePoll(w, r)
		if w.Code != http.StatusNoContent {
			t.Errorf("status = %d, want 204", w.Code)
		}
		if _, err := s.store.Load(id); err == nil {
			t.Error("expected the poll to be deleted")
		}
	})

	t.Run("with the admin password succeeds for any poll", func(t *testing.T) {
		id, _, _ := s.store.Create("Q", []string{"A", "B"}, "", 0)
		r := httptest.NewRequest(http.MethodDelete, "/api/polls/"+id+"?adminPassword=adminpw", nil)
		r.SetPathValue("id", id)
		w := httptest.NewRecorder()
		s.apiDeletePoll(w, r)
		if w.Code != http.StatusNoContent {
			t.Errorf("status = %d, want 204", w.Code)
		}
	})

	t.Run("delete invalidates the stats cache without erroring", func(t *testing.T) {
		// A dedicated server/store, isolated from the polls created (and, in
		// one case, deliberately left undeleted) by the earlier subtests.
		iso := newTestServer(t, "adminpw")
		id, ownerToken, _ := iso.store.Create("Q", []string{"A", "B"}, "", 0)
		// Prime the cache so ActiveSurveys counts this poll.
		if _, _, _, err := iso.statsCache.get(iso.store.dir); err != nil {
			t.Fatalf("get: %v", err)
		}

		r := httptest.NewRequest(http.MethodDelete, "/api/polls/"+id+"?owner="+ownerToken, nil)
		r.SetPathValue("id", id)
		w := httptest.NewRecorder()
		iso.apiDeletePoll(w, r)
		if w.Code != http.StatusNoContent {
			t.Fatalf("status = %d, want 204", w.Code)
		}

		st, _, _, err := iso.statsCache.get(iso.store.dir)
		if err != nil {
			t.Fatalf("get: %v", err)
		}
		if st.ActiveSurveys != 0 {
			t.Errorf("expected the stats cache to reflect the deletion immediately, ActiveSurveys=%d", st.ActiveSurveys)
		}
	})
}

func TestAPIStats(t *testing.T) {
	s := newTestServer(t, "")
	s.store.Create("Q", []string{"A", "B"}, "", 0)

	r := httptest.NewRequest(http.MethodGet, "/api/stats", nil)
	w := httptest.NewRecorder()
	s.apiStats(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var st stats
	if err := json.Unmarshal(w.Body.Bytes(), &st); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if st.ActiveSurveys != 1 {
		t.Errorf("ActiveSurveys = %d, want 1", st.ActiveSurveys)
	}
}

func TestAPIPreflight(t *testing.T) {
	s := newTestServer(t, "")
	r := httptest.NewRequest(http.MethodOptions, "/api/polls", nil)
	w := httptest.NewRecorder()
	s.apiPreflight(w, r)

	if w.Code != http.StatusNoContent {
		t.Errorf("status = %d, want 204", w.Code)
	}
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Errorf("Access-Control-Allow-Origin = %q, want *", got)
	}
}
