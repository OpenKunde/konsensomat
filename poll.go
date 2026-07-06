package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// Vote is a single participant's submission for a poll. Votes and Comments are
// parallel to the poll's Options slice.
type Vote struct {
	Name     string   `json:"name"`
	Votes    []int    `json:"votes"`
	Comments []string `json:"comments"`
}

// Poll is a single consensus question with its options and collected votes.
//
// ExpiresAt is chosen by the creator at creation time (bounded by the
// server's configured maximum) and is what actually governs deletion, not a
// fixed global duration - different polls can run for different lengths of
// time.
//
// OwnerToken authenticates the poll's creator (set as an HttpOnly cookie in
// their browser right after creation) and is the only way (besides a site
// admin) to delete the poll. PasswordHash, if set, additionally gates
// viewing/voting for everyone else, but never deletion. Neither field is
// ever exposed through the HTML templates or the JSON API beyond the
// creation response.
type Poll struct {
	Created      int64    `json:"created"`
	ExpiresAt    int64    `json:"expiresAt"`
	Question     string   `json:"question"`
	Options      []string `json:"options"`
	Votes        []Vote   `json:"votes"`
	PasswordHash string   `json:"passwordHash,omitempty"`
	OwnerToken   string   `json:"ownerToken"`
}

// Store persists polls as JSON files on disk, one file per poll id.
// maxExpiry is both the ceiling and the default for how long a newly created
// poll may run.
type Store struct {
	dir       string
	maxExpiry time.Duration
}

func NewStore(dir string, maxExpiry time.Duration) *Store {
	return &Store{dir: dir, maxExpiry: maxExpiry}
}

// MaxDurationDays is the largest number of days a poll is allowed to run for.
func (s *Store) MaxDurationDays() int {
	days := int(s.maxExpiry / (24 * time.Hour))
	if days < 1 {
		days = 1
	}
	return days
}

func (s *Store) path(id string) string {
	return filepath.Join(s.dir, id+".json")
}

func randomHex(nBytes int) (string, error) {
	buf := make([]byte, nBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

// pollIDLength is the length, in hex characters, of a poll id. It's the
// only thing standing between a stranger and an unprotected poll, so it
// needs enough entropy to make guessing/enumerating ids infeasible (40 bits
// - about a trillion possibilities) even though it's still short enough to
// paste into a chat.
const pollIDLength = 10

// randomID returns a random hex string of exactly hexLen characters.
func randomID(hexLen int) (string, error) {
	full, err := randomHex((hexLen + 1) / 2)
	if err != nil {
		return "", err
	}
	return full[:hexLen], nil
}

var ErrNotFound = errors.New("poll not found")

// validPollID reports whether id could plausibly be one Store generated:
// non-empty, no longer than any id this server ever creates, and hex-only.
// Load rejects anything else before it ever reaches filepath.Join, so a
// value like ".." or an encoded path separator can't be used to read or
// (via Delete/Update/etc., which all load first) write outside dir -
// regardless of how the HTTP router normalizes the request path upstream.
func validPollID(id string) bool {
	if id == "" || len(id) > pollIDLength {
		return false
	}
	for _, c := range id {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

// Load reads a poll by id, deleting and returning ErrNotFound if it has expired.
func (s *Store) Load(id string) (*Poll, error) {
	if !validPollID(id) {
		return nil, ErrNotFound
	}

	data, err := os.ReadFile(s.path(id))
	if err != nil {
		return nil, ErrNotFound
	}

	var p Poll
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, ErrNotFound
	}

	if time.Now().Unix() > p.ExpiresAt {
		os.Remove(s.path(id))
		return nil, ErrNotFound
	}

	return &p, nil
}

// newUniqueID generates a random poll id that doesn't already have a data
// file on disk. A collision is astronomically unlikely at pollIDLength=10,
// but Store.save writes unconditionally (no O_EXCL), so a collision would
// otherwise silently overwrite - and destroy - an existing poll.
func (s *Store) newUniqueID() (string, error) {
	const attempts = 5
	for i := 0; i < attempts; i++ {
		id, err := randomID(pollIDLength)
		if err != nil {
			return "", err
		}
		if _, err := os.Stat(s.path(id)); os.IsNotExist(err) {
			return id, nil
		}
	}
	return "", errors.New("could not generate a unique poll id")
}

// Create generates a new poll id and persists the poll. passwordHash may be
// empty for an unprotected poll. durationDays is how long the poll should
// run for; a value outside [1, MaxDurationDays()] (including 0, meaning "not
// specified") is clamped to that range. Create returns the poll id and a
// freshly generated owner token that authorizes management (in particular:
// deletion) of this specific poll.
func (s *Store) Create(question string, options []string, passwordHash string, durationDays int) (id string, ownerToken string, err error) {
	id, err = s.newUniqueID()
	if err != nil {
		return "", "", err
	}

	ownerToken, err = randomHex(32)
	if err != nil {
		return "", "", err
	}

	maxDays := s.MaxDurationDays()
	if durationDays <= 0 || durationDays > maxDays {
		durationDays = maxDays
	}

	now := time.Now()

	p := Poll{
		Created:      now.Unix(),
		ExpiresAt:    now.Add(time.Duration(durationDays) * 24 * time.Hour).Unix(),
		Question:     question,
		Options:      options,
		Votes:        []Vote{},
		PasswordHash: passwordHash,
		OwnerToken:   ownerToken,
	}

	if err := s.save(id, &p); err != nil {
		return "", "", err
	}

	return id, ownerToken, nil
}

// SetPassword persists passwordHash (already hashed by the caller) as the
// poll's new password - pass an empty string to remove password protection
// entirely.
func (s *Store) SetPassword(id string, passwordHash string) error {
	p, err := s.Load(id)
	if err != nil {
		return err
	}

	p.PasswordHash = passwordHash
	return s.save(id, p)
}

// Update overwrites a poll's question and options, e.g. to fix a typo or
// reword an option after votes have already started coming in. Existing
// votes are left as-is; a removed option's past votes simply become
// unreachable (Tally ignores vote entries beyond the current option count).
func (s *Store) Update(id string, question string, options []string) error {
	p, err := s.Load(id)
	if err != nil {
		return err
	}

	p.Question = question
	p.Options = options
	return s.save(id, p)
}

// SetExpiry changes how many days (from now, clamped to [1,
// MaxDurationDays()]) a poll has left to run.
func (s *Store) SetExpiry(id string, days int) error {
	p, err := s.Load(id)
	if err != nil {
		return err
	}

	maxDays := s.MaxDurationDays()
	if days < 1 {
		days = 1
	} else if days > maxDays {
		days = maxDays
	}

	p.ExpiresAt = time.Now().Add(time.Duration(days) * 24 * time.Hour).Unix()
	return s.save(id, p)
}

// AddVote appends a vote to the poll and persists it.
func (s *Store) AddVote(id string, v Vote) error {
	p, err := s.Load(id)
	if err != nil {
		return err
	}

	p.Votes = append(p.Votes, v)
	return s.save(id, p)
}

// Delete removes a poll's data file.
func (s *Store) Delete(id string) error {
	return os.Remove(s.path(id))
}

func (s *Store) save(id string, p *Poll) error {
	data, err := json.MarshalIndent(p, "", "    ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path(id), data, 0o644)
}

// Result is the aggregated resistance total and comments for one option.
type Result struct {
	Index    int
	Text     string
	Total    int
	Comments []string
}

// Tally aggregates a poll's votes into per-option results, sorted by ascending
// resistance total (lowest resistance first = strongest consensus).
func Tally(p *Poll) []Result {
	totals := make([]int, len(p.Options))
	comments := make([][]string, len(p.Options))

	for _, vote := range p.Votes {
		for i := range p.Options {
			if i >= len(vote.Votes) {
				continue
			}
			v := vote.Votes[i]
			totals[i] += v
			if v == 4 && i < len(vote.Comments) && vote.Comments[i] != "" {
				comments[i] = append(comments[i], vote.Name+": "+vote.Comments[i])
			}
		}
	}

	results := make([]Result, len(p.Options))
	for i, opt := range p.Options {
		results[i] = Result{
			Index:    i,
			Text:     opt,
			Total:    totals[i],
			Comments: comments[i],
		}
	}

	sort.SliceStable(results, func(a, b int) bool {
		return results[a].Total < results[b].Total
	})

	return results
}
