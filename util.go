package main

import (
	"strings"
	"unicode/utf8"
)

// Server-side ceilings so a single request can't bloat a poll file (and the
// O(votes*options) Tally cost) arbitrarily - generous enough for real use,
// small enough to keep per-poll storage and rendering cost bounded.
const (
	maxQuestionLen = 500
	maxOptionLen   = 200
	maxOptions     = 30
	maxNameLen     = 100
	maxCommentLen  = 1000
)

// clamp trims s and truncates it to at most max bytes, cutting on a UTF-8
// rune boundary so multi-byte characters (e.g. ä, ö, ü, emoji) never get
// split into invalid UTF-8.
func clamp(s string, max int) string {
	s = strings.TrimSpace(s)
	if len(s) <= max {
		return s
	}
	for max > 0 && !utf8.RuneStart(s[max]) {
		max--
	}
	return strings.TrimSpace(s[:max])
}
