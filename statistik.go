package main

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type heatmapCell struct {
	Date  string
	Count int
	Class string
}

// pollSummary is one row of the admin-only poll list on the statistik page -
// enough to identify a poll and jump to it, without exposing vote details.
type pollSummary struct {
	ID               string
	Question         string
	ParticipantCount int
	HasPassword      bool
	ExpiresAt        string
	created          int64 // sort key only, not rendered
}

type statistikPage struct {
	basePage
	stats
	HeatmapCells []heatmapCell
	IsAdmin      bool
	Polls        []pollSummary
	CSRFToken    string
}

// stats holds the aggregate usage numbers shared by the HTML statistik page
// and the JSON API.
type stats struct {
	ActiveSurveys   int     `json:"activeSurveys"`
	ActiveValid     int     `json:"activeWithParticipants"`
	ActiveNotValid  int     `json:"activeWithoutParticipants"`
	ExpiringSoon    int     `json:"expiringSoon"`
	AvgParticipants float64 `json:"avgParticipants"`
	MaxParticipants int     `json:"maxParticipants"`
}

// collectStats scans the poll data directory and reports aggregate usage
// stats, deleting any polls found to have exceeded their (per-poll) expiry
// along the way. dailyCount maps a creation date (YYYY-MM-DD) to the number
// of valid polls created that day, and is only needed for the HTML heatmap.
// polls lists every still-active poll (newest first) and is only needed for
// the admin-only poll list on that same page. A poll counts as "expiring
// soon" once less than a day is left to run.
func collectStats(dir string) (result stats, dailyCount map[string]int, polls []pollSummary, err error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return stats{}, nil, nil, err
	}

	now := time.Now()
	dailyCount = map[string]int{}
	var validParticipantCounts []int

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		id := strings.TrimSuffix(entry.Name(), ".json")
		path := filepath.Join(dir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		var p Poll
		if err := json.Unmarshal(data, &p); err != nil || p.Created == 0 {
			continue
		}

		created := time.Unix(p.Created, 0)
		remaining := time.Unix(p.ExpiresAt, 0).Sub(now)

		if remaining <= 0 {
			os.Remove(path)
			continue
		}

		result.ActiveSurveys++
		if remaining < 24*time.Hour {
			result.ExpiringSoon++
		}

		participants := len(p.Votes)

		if participants >= 2 {
			result.ActiveValid++
			validParticipantCounts = append(validParticipantCounts, participants)
			dailyCount[created.Format("2006-01-02")]++
		} else {
			result.ActiveNotValid++
		}

		polls = append(polls, pollSummary{
			ID:               id,
			Question:         p.Question,
			ParticipantCount: participants,
			HasPassword:      p.PasswordHash != "",
			ExpiresAt:        time.Unix(p.ExpiresAt, 0).Format("02.01.2006 15:04"),
			created:          p.Created,
		})
	}

	sort.Slice(polls, func(i, j int) bool { return polls[i].created > polls[j].created })

	if len(validParticipantCounts) > 0 {
		sum := 0
		for _, c := range validParticipantCounts {
			sum += c
			if c > result.MaxParticipants {
				result.MaxParticipants = c
			}
		}
		result.AvgParticipants = roundTo(float64(sum)/float64(len(validParticipantCounts)), 2)
	}

	return result, dailyCount, polls, nil
}

// statsCacheTTL bounds how often collectStats actually hits the disk.
// /statistik and /api/stats are both unauthenticated and collectStats scans
// every poll file (and deletes expired ones) on every call - without this,
// repeatedly hitting either endpoint is a cheap way to force expensive,
// repeated directory scans.
const statsCacheTTL = 60 * time.Second

// statsCache memoizes collectStats for statsCacheTTL, shared by the HTML
// statistik page and the JSON stats API.
type statsCache struct {
	mu         sync.Mutex
	computedAt time.Time
	stats      stats
	dailyCount map[string]int
	polls      []pollSummary
	err        error
}

// invalidate forces the next get to recompute from disk instead of serving a
// stale snapshot - used after an admin deletes a poll from the admin panel's
// list, so the list and the poll it links to don't disagree until the TTL
// naturally expires.
func (c *statsCache) invalidate() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.computedAt = time.Time{}
}

func (c *statsCache) get(dir string) (stats, map[string]int, []pollSummary, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if time.Since(c.computedAt) < statsCacheTTL {
		return c.stats, c.dailyCount, c.polls, c.err
	}

	c.stats, c.dailyCount, c.polls, c.err = collectStats(dir)
	c.computedAt = time.Now()
	return c.stats, c.dailyCount, c.polls, c.err
}

func (s *server) handleStatistik(w http.ResponseWriter, r *http.Request) {
	st, dailyCount, polls, err := s.statsCache.get(s.store.dir)
	if err != nil {
		http.Error(w, "Statistik konnte nicht ermittelt werden.", http.StatusInternalServerError)
		return
	}

	isAdmin := s.isAdmin(r)
	if !isAdmin {
		polls = nil
	}

	now := time.Now()
	cells := make([]heatmapCell, 0, 180)
	for i := 179; i >= 0; i-- {
		date := now.AddDate(0, 0, -i).Format("2006-01-02")
		count := dailyCount[date]

		class := ""
		switch {
		case count == 1:
			class = "l1"
		case count == 2:
			class = "l2"
		case count == 3:
			class = "l3"
		case count >= 4:
			class = "l4"
		}

		cells = append(cells, heatmapCell{Date: date, Count: count, Class: class})
	}

	s.render(w, http.StatusOK, "statistik.html", statistikPage{
		basePage:     basePage{PageTitle: pageTitle("Statistik")},
		stats:        st,
		HeatmapCells: cells,
		IsAdmin:      isAdmin,
		Polls:        polls,
		CSRFToken:    s.csrfToken(w, r),
	})
}

func roundTo(v float64, decimals int) float64 {
	factor := 1.0
	for i := 0; i < decimals; i++ {
		factor *= 10
	}
	return float64(int(v*factor+0.5)) / factor
}
