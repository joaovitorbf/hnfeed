package main

// ── Feed state ────────────────────────────────────────────────────────────────

type feedState struct {
	buf              []string
	frontRanks       map[int]int   // id → last known rank (1-based)
	frontBestRanks   map[int]int   // id → best (lowest-number) rank ever seen
	frontWorstRanks  map[int]int   // id → worst (highest-number) rank ever seen
	frontCache       map[int]*Item // last known item data for front-page items
	seenIDs          map[int]bool  // ids already emitted as new-story entries
	maxID            int           // highest new-story ID seen; watermark for incremental polling
	scroll           int           // lines scrolled up from the bottom (0 = live)
	totalItems       int           // total entries ever appended
}
