package main

import (
	"os"
	"testing"
	"time"
)

func TestCollectStatsEmptyDir(t *testing.T) {
	dir := t.TempDir()
	st, dailyCount, polls, err := collectStats(dir)
	if err != nil {
		t.Fatalf("collectStats: %v", err)
	}
	if st.ActiveSurveys != 0 || len(dailyCount) != 0 || len(polls) != 0 {
		t.Errorf("expected all-zero stats for an empty dir, got %#v", st)
	}
}

func TestCollectStatsIgnoresNonJSONFiles(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(dir+"/.gitkeep", nil, 0o644); err != nil {
		t.Fatalf("write .gitkeep: %v", err)
	}
	if err := os.WriteFile(dir+"/notes.txt", []byte("hello"), 0o644); err != nil {
		t.Fatalf("write notes.txt: %v", err)
	}

	st, _, polls, err := collectStats(dir)
	if err != nil {
		t.Fatalf("collectStats: %v", err)
	}
	if st.ActiveSurveys != 0 || len(polls) != 0 {
		t.Errorf("expected non-.json files to be ignored, got %#v", st)
	}
}

func TestCollectStatsCountsParticipantThreshold(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir, 24*time.Hour)

	oneVoter, _, _ := s.Create("Q1", []string{"A", "B"}, "", 0)
	s.AddVote(oneVoter, Vote{Name: "Alice", Votes: []int{0, 1}})

	twoVoters, _, _ := s.Create("Q2", []string{"A", "B"}, "", 0)
	s.AddVote(twoVoters, Vote{Name: "Alice", Votes: []int{0, 1}})
	s.AddVote(twoVoters, Vote{Name: "Bob", Votes: []int{1, 0}})

	st, dailyCount, polls, err := collectStats(dir)
	if err != nil {
		t.Fatalf("collectStats: %v", err)
	}

	if st.ActiveSurveys != 2 {
		t.Errorf("ActiveSurveys = %d, want 2", st.ActiveSurveys)
	}
	if st.ActiveValid != 1 {
		t.Errorf("ActiveValid = %d, want 1 (only the 2-participant poll)", st.ActiveValid)
	}
	if st.ActiveNotValid != 1 {
		t.Errorf("ActiveNotValid = %d, want 1 (only the 1-participant poll)", st.ActiveNotValid)
	}
	if st.MaxParticipants != 2 {
		t.Errorf("MaxParticipants = %d, want 2", st.MaxParticipants)
	}
	if len(polls) != 2 {
		t.Fatalf("expected 2 poll summaries, got %d", len(polls))
	}

	total := 0
	for _, count := range dailyCount {
		total += count
	}
	if total != 1 {
		t.Errorf("expected exactly 1 day-bucket entry (only the valid poll counts), got %d", total)
	}
}

func TestCollectStatsExpiringSoon(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir, 24*time.Hour)

	id, _, _ := s.Create("Q", []string{"A", "B"}, "", 1)
	p, _ := s.Load(id)
	p.ExpiresAt = time.Now().Add(2 * time.Hour).Unix()
	if err := s.save(id, p); err != nil {
		t.Fatalf("save: %v", err)
	}

	st, _, _, err := collectStats(dir)
	if err != nil {
		t.Fatalf("collectStats: %v", err)
	}
	if st.ExpiringSoon != 1 {
		t.Errorf("ExpiringSoon = %d, want 1", st.ExpiringSoon)
	}
}

func TestCollectStatsRemovesExpiredPolls(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir, 24*time.Hour)

	id, _, _ := s.Create("Q", []string{"A", "B"}, "", 1)
	p, _ := s.Load(id)
	p.ExpiresAt = time.Now().Add(-time.Hour).Unix()
	if err := s.save(id, p); err != nil {
		t.Fatalf("save: %v", err)
	}

	st, _, polls, err := collectStats(dir)
	if err != nil {
		t.Fatalf("collectStats: %v", err)
	}
	if st.ActiveSurveys != 0 || len(polls) != 0 {
		t.Errorf("expected the expired poll to be excluded, got %#v / %#v", st, polls)
	}
	if _, err := os.Stat(s.path(id)); !os.IsNotExist(err) {
		t.Error("expected collectStats to delete the expired poll's file")
	}
}

func TestCollectStatsPollsSortedNewestFirst(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir, 24*time.Hour)

	older, _, _ := s.Create("Älter", []string{"A", "B"}, "", 0)
	p, _ := s.Load(older)
	p.Created = time.Now().Add(-time.Hour).Unix()
	s.save(older, p)

	newer, _, _ := s.Create("Neuer", []string{"A", "B"}, "", 0)

	_, _, polls, err := collectStats(dir)
	if err != nil {
		t.Fatalf("collectStats: %v", err)
	}
	if len(polls) != 2 {
		t.Fatalf("expected 2 polls, got %d", len(polls))
	}
	if polls[0].ID != newer || polls[1].ID != older {
		t.Errorf("expected newest-first order [%s, %s], got [%s, %s]", newer, older, polls[0].ID, polls[1].ID)
	}
}

func TestStatsCacheServesCachedValueUntilInvalidated(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir, 24*time.Hour)
	s.Create("Q1", []string{"A", "B"}, "", 0)

	cache := &statsCache{}
	st, _, _, err := cache.get(dir)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if st.ActiveSurveys != 1 {
		t.Fatalf("ActiveSurveys = %d, want 1", st.ActiveSurveys)
	}

	// A second poll appears on disk, but the cache should still serve the
	// stale value until it's invalidated (or the TTL elapses).
	s.Create("Q2", []string{"A", "B"}, "", 0)
	st, _, _, err = cache.get(dir)
	if err != nil {
		t.Fatalf("get (cached): %v", err)
	}
	if st.ActiveSurveys != 1 {
		t.Errorf("expected the cached value (1) to still be served, got %d", st.ActiveSurveys)
	}

	cache.invalidate()
	st, _, _, err = cache.get(dir)
	if err != nil {
		t.Fatalf("get (after invalidate): %v", err)
	}
	if st.ActiveSurveys != 2 {
		t.Errorf("expected a fresh recompute (2) after invalidate, got %d", st.ActiveSurveys)
	}
}
