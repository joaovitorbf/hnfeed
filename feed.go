package main

import "time"

// ── Entry types ───────────────────────────────────────────────────────────────

type entryType int

const (
	entryNew entryType = iota
	entryFrontEnter
	entryFrontUp
	entryFrontDown
	entryFrontLeave
)

// feedEntry stores the raw data needed to render a single feed entry.
// Formatting happens at render time so it always uses the current width.
type feedEntry struct {
	typ     entryType
	item    *Item
	prefix  string // e.g. "★ #3  ", "↑ #3 (was #7)  ", "✕ #28  "
	oldRank int    // used for leave events
	time    time.Time
}

// ── Feed state ────────────────────────────────────────────────────────────────

type feedState struct {
	entries         []feedEntry
	frontRanks      map[int]int   // id → last known rank (1-based)
	frontBestRanks  map[int]int   // id → best (lowest-number) rank ever seen
	frontWorstRanks map[int]int   // id → worst (highest-number) rank ever seen
	frontCache      map[int]*Item // last known item data for front-page items
	seenIDs         map[int]bool  // ids already emitted as new-story entries
	maxID           int           // highest new-story ID seen; watermark for incremental polling
	scroll          int           // lines scrolled up from the bottom (0 = live)
	totalItems      int           // total entries ever appended
}

// appendEntry adds an entry and advances scroll if the user is scrolled up,
// keeping the viewport stable.
func (s *feedState) appendEntry(e feedEntry) {
	e.time = time.Now()
	s.entries = append(s.entries, e)
	s.totalItems++
	if s.scroll > 0 {
		s.scroll += 4
	}
}

// totalLines returns the total number of rendered lines (4 per entry).
func (s *feedState) totalLines() int {
	return len(s.entries) * 4
}
