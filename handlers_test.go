package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// csrfPair mints a CSRF token together with the cookie that carries it, the
// same pair a browser would hold after loading any page on the site.
func csrfPair(s *server) (token string, cookie *http.Cookie) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	token = s.csrfToken(w, r)
	return token, w.Result().Cookies()[0]
}

// newFormRequest builds a POST request with an application/x-www-form-urlencoded
// body, the way a real HTML <form> submission would - so handlers that call
// r.ParseForm()/r.PostFormValue() themselves see exactly what they'd see in
// production.
func newFormRequest(target string, form url.Values) *http.Request {
	r := httptest.NewRequest(http.MethodPost, target, strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return r
}

func createTestPoll(t *testing.T, s *server) (id, ownerToken string) {
	t.Helper()
	id, ownerToken, err := s.store.Create("Testfrage", []string{"A", "B"}, "", 0)
	if err != nil {
		t.Fatalf("store.Create: %v", err)
	}
	return id, ownerToken
}

func TestHandleIndex(t *testing.T) {
	s := newTestServer(t, "")
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)

	s.handleIndex(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if !strings.Contains(w.Body.String(), `name="duration_days"`) {
		t.Error("expected the duration select to be present in the index page")
	}
}

func TestHandleCreate(t *testing.T) {
	s := newTestServer(t, "")
	token, cookie := csrfPair(s)

	t.Run("missing CSRF token is rejected", func(t *testing.T) {
		r := newFormRequest("/create", url.Values{"question": {"Q"}, "options[]": {"A", "B"}})
		w := httptest.NewRecorder()
		s.handleCreate(w, r)
		if w.Code != http.StatusForbidden {
			t.Errorf("status = %d, want 403", w.Code)
		}
	})

	t.Run("empty question redirects home without creating a poll", func(t *testing.T) {
		r := newFormRequest("/create", url.Values{"token": {token}, "question": {""}, "options[]": {"A", "B"}})
		r.AddCookie(cookie)
		w := httptest.NewRecorder()
		s.handleCreate(w, r)
		if w.Code != http.StatusSeeOther || w.Header().Get("Location") != "/" {
			t.Errorf("status=%d location=%q, want 303 to /", w.Code, w.Header().Get("Location"))
		}
	})

	t.Run("fewer than two options redirects home", func(t *testing.T) {
		r := newFormRequest("/create", url.Values{"token": {token}, "question": {"Q"}, "options[]": {"Only one"}})
		r.AddCookie(cookie)
		w := httptest.NewRecorder()
		s.handleCreate(w, r)
		if w.Code != http.StatusSeeOther || w.Header().Get("Location") != "/" {
			t.Errorf("status=%d location=%q, want 303 to /", w.Code, w.Header().Get("Location"))
		}
	})

	t.Run("valid submission creates a poll and sets the owner cookie", func(t *testing.T) {
		r := newFormRequest("/create", url.Values{
			"token":         {token},
			"question":      {"Wohin geht die Firmenfeier?"},
			"options[]":     {"Strand", "Berge"},
			"duration_days": {"3"},
		})
		r.AddCookie(cookie)
		w := httptest.NewRecorder()
		s.handleCreate(w, r)

		if w.Code != http.StatusSeeOther {
			t.Fatalf("status = %d, want 303", w.Code)
		}
		loc := w.Header().Get("Location")
		if !strings.HasPrefix(loc, "/poll/") {
			t.Fatalf("Location = %q, want /poll/<id>", loc)
		}
		id := strings.TrimPrefix(loc, "/poll/")

		p, err := s.store.Load(id)
		if err != nil {
			t.Fatalf("created poll not found in store: %v", err)
		}
		if p.Question != "Wohin geht die Firmenfeier?" {
			t.Errorf("Question = %q", p.Question)
		}

		var ownerCookieSet bool
		for _, c := range w.Result().Cookies() {
			if c.Name == ownerCookieName(id) && c.Value == p.OwnerToken {
				ownerCookieSet = true
			}
		}
		if !ownerCookieSet {
			t.Error("expected the owner cookie to be set on successful creation")
		}
	})
}

func TestHandlePollView(t *testing.T) {
	s := newTestServer(t, "")

	t.Run("unknown poll 404s", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/poll/0123456789", nil)
		r.SetPathValue("id", "0123456789")
		w := httptest.NewRecorder()
		s.handlePollView(w, r)
		if w.Code != http.StatusNotFound {
			t.Errorf("status = %d, want 404", w.Code)
		}
	})

	t.Run("open poll renders for anyone", func(t *testing.T) {
		id, _ := createTestPoll(t, s)
		r := httptest.NewRequest(http.MethodGet, "/poll/"+id, nil)
		r.SetPathValue("id", id)
		w := httptest.NewRecorder()
		s.handlePollView(w, r)
		if w.Code != http.StatusOK {
			t.Errorf("status = %d, want 200", w.Code)
		}
		if !strings.Contains(w.Body.String(), "Testfrage") {
			t.Error("expected the poll question to be rendered")
		}
	})

	t.Run("password-protected poll without credentials is locked", func(t *testing.T) {
		hash, _ := HashPassword("geheim")
		id, _, _ := s.store.Create("Geheime Frage", []string{"A", "B"}, hash, 0)
		r := httptest.NewRequest(http.MethodGet, "/poll/"+id, nil)
		r.SetPathValue("id", id)
		w := httptest.NewRecorder()
		s.handlePollView(w, r)
		if w.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want 401", w.Code)
		}
	})

	t.Run("password-protected poll with the right ?password= is visible", func(t *testing.T) {
		hash, _ := HashPassword("geheim")
		id, _, _ := s.store.Create("Geheime Frage 2", []string{"A", "B"}, hash, 0)
		r := httptest.NewRequest(http.MethodGet, "/poll/"+id+"?password=geheim", nil)
		r.SetPathValue("id", id)
		w := httptest.NewRecorder()
		s.handlePollView(w, r)
		if w.Code != http.StatusOK {
			t.Errorf("status = %d, want 200", w.Code)
		}
	})
}

func TestHandleVote(t *testing.T) {
	s := newTestServer(t, "")
	token, cookie := csrfPair(s)
	id, _ := createTestPoll(t, s)

	t.Run("missing CSRF token is rejected", func(t *testing.T) {
		r := newFormRequest("/poll/"+id+"/vote", url.Values{"name": {"Alice"}})
		r.SetPathValue("id", id)
		w := httptest.NewRecorder()
		s.handleVote(w, r)
		if w.Code != http.StatusForbidden {
			t.Errorf("status = %d, want 403", w.Code)
		}
	})

	t.Run("valid vote is persisted", func(t *testing.T) {
		r := newFormRequest("/poll/"+id+"/vote", url.Values{
			"token":       {token},
			"name":        {"Alice"},
			"votes[0]":    {"0"},
			"votes[1]":    {"4"},
			"comments[1]": {"zu riskant"},
		})
		r.AddCookie(cookie)
		r.SetPathValue("id", id)
		w := httptest.NewRecorder()
		s.handleVote(w, r)

		if w.Code != http.StatusSeeOther {
			t.Fatalf("status = %d, want 303", w.Code)
		}

		p, err := s.store.Load(id)
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if len(p.Votes) != 1 || p.Votes[0].Name != "Alice" {
			t.Fatalf("expected 1 vote from Alice, got %#v", p.Votes)
		}
		if p.Votes[0].Votes[1] != 4 || p.Votes[0].Comments[1] != "zu riskant" {
			t.Errorf("vote not recorded as submitted: %#v", p.Votes[0])
		}
	})

	t.Run("out-of-range vote values are clamped to [0,4]", func(t *testing.T) {
		r := newFormRequest("/poll/"+id+"/vote", url.Values{
			"token":    {token},
			"name":     {"Bob"},
			"votes[0]": {"-5"},
			"votes[1]": {"99"},
		})
		r.AddCookie(cookie)
		r.SetPathValue("id", id)
		w := httptest.NewRecorder()
		s.handleVote(w, r)

		p, _ := s.store.Load(id)
		last := p.Votes[len(p.Votes)-1]
		if last.Votes[0] != 0 || last.Votes[1] != 4 {
			t.Errorf("expected votes to be clamped to [0,4], got %v", last.Votes)
		}
	})
}

func TestHandleUnlock(t *testing.T) {
	s := newTestServer(t, "")
	token, cookie := csrfPair(s)
	hash, _ := HashPassword("geheim")
	id, _, _ := s.store.Create("Geheim", []string{"A", "B"}, hash, 0)

	t.Run("wrong password re-renders locked page", func(t *testing.T) {
		r := newFormRequest("/poll/"+id+"/unlock", url.Values{"token": {token}, "password": {"falsch"}})
		r.AddCookie(cookie)
		r.SetPathValue("id", id)
		w := httptest.NewRecorder()
		s.handleUnlock(w, r)
		if w.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want 401", w.Code)
		}
	})

	t.Run("correct password redirects and sets unlock cookie", func(t *testing.T) {
		r := newFormRequest("/poll/"+id+"/unlock", url.Values{"token": {token}, "password": {"geheim"}})
		r.AddCookie(cookie)
		r.SetPathValue("id", id)
		w := httptest.NewRecorder()
		s.handleUnlock(w, r)

		if w.Code != http.StatusSeeOther {
			t.Fatalf("status = %d, want 303", w.Code)
		}
		var unlocked bool
		for _, c := range w.Result().Cookies() {
			if c.Name == unlockCookieName(id) {
				unlocked = true
			}
		}
		if !unlocked {
			t.Error("expected the unlock cookie to be set")
		}
	})
}

func TestHandleDelete(t *testing.T) {
	s := newTestServer(t, "adminpw")
	token, cookie := csrfPair(s)

	t.Run("non-owner, non-admin cannot delete", func(t *testing.T) {
		id, _ := createTestPoll(t, s)
		r := newFormRequest("/poll/"+id+"/delete", url.Values{"token": {token}})
		r.AddCookie(cookie)
		r.SetPathValue("id", id)
		w := httptest.NewRecorder()
		s.handleDelete(w, r)
		if w.Code != http.StatusForbidden {
			t.Errorf("status = %d, want 403", w.Code)
		}
		if _, err := s.store.Load(id); err != nil {
			t.Error("poll should not have been deleted")
		}
	})

	t.Run("owner can delete and is sent home", func(t *testing.T) {
		id, ownerToken := createTestPoll(t, s)
		r := newFormRequest("/poll/"+id+"/delete", url.Values{"token": {token}})
		r.AddCookie(cookie)
		r.AddCookie(&http.Cookie{Name: ownerCookieName(id), Value: ownerToken})
		r.SetPathValue("id", id)
		w := httptest.NewRecorder()
		s.handleDelete(w, r)

		if w.Code != http.StatusSeeOther || w.Header().Get("Location") != "/" {
			t.Errorf("status=%d location=%q, want 303 to /", w.Code, w.Header().Get("Location"))
		}
		if _, err := s.store.Load(id); err == nil {
			t.Error("expected the poll to be deleted")
		}
	})

	t.Run("admin can delete and is sent to the admin panel", func(t *testing.T) {
		id, _ := createTestPoll(t, s)
		r := newFormRequest("/poll/"+id+"/delete?adminPassword=adminpw", url.Values{"token": {token}})
		r.AddCookie(cookie)
		r.SetPathValue("id", id)
		w := httptest.NewRecorder()
		s.handleDelete(w, r)

		if w.Code != http.StatusSeeOther || w.Header().Get("Location") != "/statistik" {
			t.Errorf("status=%d location=%q, want 303 to /statistik", w.Code, w.Header().Get("Location"))
		}
		if _, err := s.store.Load(id); err == nil {
			t.Error("expected the poll to be deleted")
		}
	})
}

func TestHandleSetPassword(t *testing.T) {
	s := newTestServer(t, "")
	token, cookie := csrfPair(s)
	id, ownerToken := createTestPoll(t, s)

	r := newFormRequest("/poll/"+id+"/password", url.Values{"token": {token}, "password": {"neu-geheim"}})
	r.AddCookie(cookie)
	r.AddCookie(&http.Cookie{Name: ownerCookieName(id), Value: ownerToken})
	r.SetPathValue("id", id)
	w := httptest.NewRecorder()
	s.handleSetPassword(w, r)

	if w.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303", w.Code)
	}
	p, _ := s.store.Load(id)
	if !VerifyPassword(p.PasswordHash, "neu-geheim") {
		t.Error("expected the new password to be persisted")
	}
}

func TestHandleEditPoll(t *testing.T) {
	s := newTestServer(t, "")
	token, cookie := csrfPair(s)
	id, ownerToken := createTestPoll(t, s)

	r := newFormRequest("/poll/"+id+"/edit", url.Values{
		"token":     {token},
		"question":  {"Neue Frage"},
		"options[]": {"X", "Y", "Z"},
	})
	r.AddCookie(cookie)
	r.AddCookie(&http.Cookie{Name: ownerCookieName(id), Value: ownerToken})
	r.SetPathValue("id", id)
	w := httptest.NewRecorder()
	s.handleEditPoll(w, r)

	if w.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303", w.Code)
	}
	p, _ := s.store.Load(id)
	if p.Question != "Neue Frage" || len(p.Options) != 3 {
		t.Errorf("poll not updated as expected: %#v", p)
	}
}

func TestHandleSetExpiry(t *testing.T) {
	s := newTestServer(t, "")
	token, cookie := csrfPair(s)
	id, ownerToken := createTestPoll(t, s)

	r := newFormRequest("/poll/"+id+"/expiry", url.Values{"token": {token}, "duration_days": {"2"}})
	r.AddCookie(cookie)
	r.AddCookie(&http.Cookie{Name: ownerCookieName(id), Value: ownerToken})
	r.SetPathValue("id", id)
	w := httptest.NewRecorder()
	s.handleSetExpiry(w, r)

	if w.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303", w.Code)
	}
}

func TestHandleAdminLoginLogout(t *testing.T) {
	s := newTestServer(t, "adminpw")
	token, cookie := csrfPair(s)

	t.Run("wrong password re-renders the login form", func(t *testing.T) {
		r := newFormRequest("/admin/login", url.Values{"token": {token}, "password": {"falsch"}})
		r.AddCookie(cookie)
		w := httptest.NewRecorder()
		s.handleAdminLogin(w, r)
		if w.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want 401", w.Code)
		}
	})

	var adminCookie *http.Cookie
	t.Run("correct password logs in", func(t *testing.T) {
		r := newFormRequest("/admin/login", url.Values{"token": {token}, "password": {"adminpw"}})
		r.AddCookie(cookie)
		w := httptest.NewRecorder()
		s.handleAdminLogin(w, r)
		if w.Code != http.StatusSeeOther || w.Header().Get("Location") != "/statistik" {
			t.Errorf("status=%d location=%q, want 303 to /statistik", w.Code, w.Header().Get("Location"))
		}
		for _, c := range w.Result().Cookies() {
			if c.Name == adminCookieName {
				adminCookie = c
			}
		}
		if adminCookie == nil {
			t.Fatal("expected an admin session cookie to be set")
		}
	})

	t.Run("logout clears the session", func(t *testing.T) {
		r := newFormRequest("/admin/logout", url.Values{"token": {token}})
		r.AddCookie(cookie)
		r.AddCookie(adminCookie)
		w := httptest.NewRecorder()
		s.handleAdminLogout(w, r)
		if w.Code != http.StatusSeeOther {
			t.Fatalf("status = %d, want 303", w.Code)
		}

		check := httptest.NewRequest(http.MethodGet, "/", nil)
		for _, c := range w.Result().Cookies() {
			check.AddCookie(c)
		}
		if s.isAdmin(check) {
			t.Error("expected the admin session to no longer authenticate after logout")
		}
	})
}

func TestHandleStaticPages(t *testing.T) {
	s := newTestServer(t, "")
	cases := []struct {
		name    string
		handler http.HandlerFunc
	}{
		{"info", s.handleInfo},
		{"impressum", s.handleImpressum},
		{"datenschutz", s.handleDatenschutz},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodGet, "/"+c.name, nil)
			c.handler(w, r)
			if w.Code != http.StatusOK {
				t.Errorf("status = %d, want 200", w.Code)
			}
		})
	}
}
