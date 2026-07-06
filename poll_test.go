package main

import (
	"errors"
	"os"
	"testing"
	"time"
)

func newTestStore(t *testing.T, maxExpiry time.Duration) *Store {
	t.Helper()
	return NewStore(t.TempDir(), maxExpiry)
}

func TestStoreCreateAndLoad(t *testing.T) {
	s := newTestStore(t, 7*24*time.Hour)

	id, ownerToken, err := s.Create("Wohin geht die Firmenfeier?", []string{"Strand", "Berge"}, "", 0)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if id == "" || ownerToken == "" {
		t.Fatal("expected a non-empty id and owner token")
	}

	p, err := s.Load(id)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if p.Question != "Wohin geht die Firmenfeier?" {
		t.Errorf("Question = %q", p.Question)
	}
	if len(p.Options) != 2 {
		t.Errorf("expected 2 options, got %d", len(p.Options))
	}
	if p.OwnerToken != ownerToken {
		t.Error("stored OwnerToken should match the token returned by Create")
	}
	if p.Votes == nil || len(p.Votes) != 0 {
		t.Errorf("expected an empty (non-nil) Votes slice, got %#v", p.Votes)
	}
}

func TestStoreCreateClampsDurationToMax(t *testing.T) {
	s := newTestStore(t, 3*24*time.Hour)

	id, _, err := s.Create("Q", []string{"A", "B"}, "", 999)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	p, err := s.Load(id)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	maxExpiry := time.Now().Add(3 * 24 * time.Hour).Unix()
	if p.ExpiresAt > maxExpiry+5 { // small slack for test execution time
		t.Errorf("expected ExpiresAt to be clamped to the 3-day maximum, got %d (max ~%d)", p.ExpiresAt, maxExpiry)
	}
}

func TestStoreLoadUnknownID(t *testing.T) {
	s := newTestStore(t, 24*time.Hour)

	if _, err := s.Load("0123456789"); !errors.Is(err, ErrNotFound) {
		t.Errorf("Load(unknown id): got err=%v, want ErrNotFound", err)
	}
}

// TestStoreLoadRejectsMaliciousIDs guards the path-traversal defense in
// validPollID: an id must never be allowed to escape the store directory via
// filepath.Join, regardless of what the HTTP layer already filtered out.
func TestStoreLoadRejectsMaliciousIDs(t *testing.T) {
	s := newTestStore(t, 24*time.Hour)

	cases := []string{
		"",
		"..",
		"../../etc/passwd",
		"a/b",
		"UPPERCASE",
		"not-hex!!",
		"00000000000000000000", // longer than pollIDLength
	}
	for _, id := range cases {
		if _, err := s.Load(id); !errors.Is(err, ErrNotFound) {
			t.Errorf("Load(%q): got err=%v, want ErrNotFound", id, err)
		}
	}
}

func TestStoreLoadDeletesExpiredPoll(t *testing.T) {
	s := newTestStore(t, 24*time.Hour)

	id, _, err := s.Create("Q", []string{"A", "B"}, "", 1)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Backdate the poll's expiry directly on disk, bypassing SetExpiry's
	// clamp to [1, MaxDurationDays()] (which can't express "already expired").
	p, err := s.Load(id)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	p.ExpiresAt = time.Now().Add(-time.Hour).Unix()
	if err := s.save(id, p); err != nil {
		t.Fatalf("save: %v", err)
	}

	if _, err := s.Load(id); !errors.Is(err, ErrNotFound) {
		t.Errorf("Load(expired poll): got err=%v, want ErrNotFound", err)
	}
	if _, err := os.Stat(s.path(id)); !os.IsNotExist(err) {
		t.Error("expected the expired poll's file to have been removed from disk")
	}
}

func TestStoreSetPassword(t *testing.T) {
	s := newTestStore(t, 24*time.Hour)
	id, _, _ := s.Create("Q", []string{"A", "B"}, "", 0)

	hash, err := HashPassword("geheim")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if err := s.SetPassword(id, hash); err != nil {
		t.Fatalf("SetPassword: %v", err)
	}

	p, _ := s.Load(id)
	if p.PasswordHash != hash {
		t.Error("expected the new password hash to be persisted")
	}

	if err := s.SetPassword(id, ""); err != nil {
		t.Fatalf("SetPassword (clear): %v", err)
	}
	p, _ = s.Load(id)
	if p.PasswordHash != "" {
		t.Error("expected an empty password to remove protection")
	}
}

func TestStoreUpdate(t *testing.T) {
	s := newTestStore(t, 24*time.Hour)
	id, _, _ := s.Create("Alte Frage", []string{"A", "B"}, "", 0)

	if err := s.Update(id, "Neue Frage", []string{"X", "Y", "Z"}); err != nil {
		t.Fatalf("Update: %v", err)
	}

	p, _ := s.Load(id)
	if p.Question != "Neue Frage" {
		t.Errorf("Question = %q", p.Question)
	}
	if len(p.Options) != 3 {
		t.Errorf("expected 3 options, got %d", len(p.Options))
	}
}

func TestStoreSetExpiryClampsRange(t *testing.T) {
	s := newTestStore(t, 5*24*time.Hour)
	id, _, _ := s.Create("Q", []string{"A", "B"}, "", 1)

	if err := s.SetExpiry(id, 0); err != nil {
		t.Fatalf("SetExpiry(0): %v", err)
	}
	p, _ := s.Load(id)
	wantMin := time.Now().Add(23 * time.Hour).Unix() // ~1 day, minus test slack
	if p.ExpiresAt < wantMin {
		t.Errorf("SetExpiry(0) should clamp up to 1 day, got ExpiresAt=%d", p.ExpiresAt)
	}

	if err := s.SetExpiry(id, 999); err != nil {
		t.Fatalf("SetExpiry(999): %v", err)
	}
	p, _ = s.Load(id)
	wantMax := time.Now().Add(5*24*time.Hour + time.Hour).Unix()
	if p.ExpiresAt > wantMax {
		t.Errorf("SetExpiry(999) should clamp down to the 5-day maximum, got ExpiresAt=%d", p.ExpiresAt)
	}
}

func TestStoreAddVoteAndDelete(t *testing.T) {
	s := newTestStore(t, 24*time.Hour)
	id, _, _ := s.Create("Q", []string{"A", "B"}, "", 0)

	if err := s.AddVote(id, Vote{Name: "Alice", Votes: []int{0, 4}, Comments: []string{"", "zu weit weg"}}); err != nil {
		t.Fatalf("AddVote: %v", err)
	}
	p, _ := s.Load(id)
	if len(p.Votes) != 1 || p.Votes[0].Name != "Alice" {
		t.Errorf("expected 1 vote from Alice, got %#v", p.Votes)
	}

	if err := s.Delete(id); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := s.Load(id); !errors.Is(err, ErrNotFound) {
		t.Errorf("Load after Delete: got err=%v, want ErrNotFound", err)
	}

	// Deleting an already-deleted (or never-existing) poll must report
	// os.ErrNotExist, since handlers rely on that to distinguish "already
	// gone" from a real disk error worth logging.
	if err := s.Delete(id); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("Delete (already gone): got err=%v, want os.ErrNotExist", err)
	}
}

func TestStoreNewUniqueIDLength(t *testing.T) {
	s := newTestStore(t, 24*time.Hour)
	id, err := s.newUniqueID()
	if err != nil {
		t.Fatalf("newUniqueID: %v", err)
	}
	if len(id) != pollIDLength {
		t.Errorf("expected id of length %d, got %d (%q)", pollIDLength, len(id), id)
	}
	if !validPollID(id) {
		t.Errorf("newUniqueID produced an id that fails validPollID: %q", id)
	}
}

func TestMaxDurationDays(t *testing.T) {
	cases := []struct {
		expiry time.Duration
		want   int
	}{
		{0, 1},         // floors at 1, never 0
		{time.Hour, 1}, // less than a day still rounds down to 1
		{7 * 24 * time.Hour, 7},
		{365 * 24 * time.Hour, 365},
	}
	for _, c := range cases {
		s := NewStore(t.TempDir(), c.expiry)
		if got := s.MaxDurationDays(); got != c.want {
			t.Errorf("MaxDurationDays() with expiry=%v: got %d, want %d", c.expiry, got, c.want)
		}
	}
}

func TestTallySortsByAscendingResistance(t *testing.T) {
	p := &Poll{
		Options: []string{"Strand", "Berge", "Stadt"},
		Votes: []Vote{
			{Name: "Alice", Votes: []int{4, 0, 2}, Comments: []string{"zu heiß", "", ""}},
			{Name: "Bob", Votes: []int{4, 1, 2}, Comments: []string{"stimme zu", "", ""}},
		},
	}

	results := Tally(p)
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}

	// Berge (1) < Stadt (4) < Strand (8)
	wantOrder := []string{"Berge", "Stadt", "Strand"}
	for i, r := range results {
		if r.Text != wantOrder[i] {
			t.Errorf("results[%d].Text = %q, want %q", i, r.Text, wantOrder[i])
		}
	}

	strand := results[2]
	if strand.Total != 8 {
		t.Errorf("Strand total = %d, want 8", strand.Total)
	}
	if len(strand.Comments) != 2 {
		t.Errorf("expected both resistance-4 comments on Strand to be collected, got %#v", strand.Comments)
	}

	berge := results[0]
	if len(berge.Comments) != 0 {
		t.Errorf("Berge never got a resistance-4 vote, expected no comments, got %#v", berge.Comments)
	}
}

func TestTallyIgnoresVotesForRemovedOptions(t *testing.T) {
	// A poll edited (via Update) to have fewer options than an older vote
	// recorded: Tally must not panic and must simply ignore the dangling
	// vote entries beyond the current option count.
	p := &Poll{
		Options: []string{"A"},
		Votes: []Vote{
			{Name: "Alice", Votes: []int{2, 4}, Comments: []string{"", "veraltete Option"}},
		},
	}

	results := Tally(p)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Total != 2 {
		t.Errorf("Total = %d, want 2", results[0].Total)
	}
}
