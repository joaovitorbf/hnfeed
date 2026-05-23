package main

import (
	"fmt"
	"time"
)

// ── Entry formatters ──────────────────────────────────────────────────────────

func itemURL(item *Item) string {
	if item.URL != "" {
		return item.URL
	}
	return fmt.Sprintf("https://news.ycombinator.com/item?id=%d", item.ID)
}

func commentURL(id int) string {
	return fmt.Sprintf("https://news.ycombinator.com/item?id=%d", id)
}

// formatNewItemLines returns the 4 feed lines for a new story.
//
//	[HH:mm:ss] Title
//	  article URL
//	  Comments: HN URL
//	(blank)
func formatNewItemLines(item *Item, width int) []string {
	t := time.Unix(item.Time, 0).Local().Format("15:04:05")
	meta := gry + "[NEW]" + rst
	const timeVisible = 11 // "[HH:mm:ss] "
	avail := width - timeVisible - len([]rune("[NEW]")) - 1
	if avail < 1 {
		avail = 1
	}
	title := padRight(truncRunes(item.Title, avail), avail)
	return []string{
		gry + "[" + t + "]" + rst + " " + yel + title + rst + " " + meta,
		"  " + gry + itemURL(item) + rst,
		"  " + gry + "Comments: " + commentURL(item.ID) + rst,
		"",
	}
}

// formatFrontEventLines returns the 4 feed lines for a front-page event.
// prefix is e.g. "★ #3  " for a new entry or "↑ #3 (was #7)  " for a rank-up.
//
//	[HH:mm:ss] <prefix> Title …   [score▲ Nc]
//	  article URL
//	  Comments: HN URL
//	(blank)
func formatFrontEventLines(item *Item, prefix string, width int) []string {
	t := time.Now().Format("15:04:05")
	metaPlain := fmt.Sprintf("[%d▲ %dc]", item.Score, item.Descendants)
	const timeVisible = 11
	avail := width - timeVisible - len([]rune(prefix)) - len([]rune(metaPlain)) - 1
	if avail < 1 {
		avail = 1
	}
	title := padRight(truncRunes(item.Title, avail), avail)
	return []string{
		gry + "[" + t + "]" + rst + " " + org + prefix + title + rst + " " + gry + metaPlain + rst,
		"  " + gry + itemURL(item) + rst,
		"  " + gry + "Comments: " + commentURL(item.ID) + rst,
		"",
	}
}

// formatFrontLeaveLine returns the 4 feed lines for an item leaving the front page.
func formatFrontLeaveLine(item *Item, oldRank int, width int) []string {
	t := time.Now().Format("15:04:05")
	metaPlain := fmt.Sprintf("[%d▲ %dc]", item.Score, item.Descendants)
	const timeVisible = 11
	prefix := fmt.Sprintf("✕ #%d  ", oldRank)
	avail := width - timeVisible - len([]rune(prefix)) - len([]rune(metaPlain)) - 1
	if avail < 1 {
		avail = 1
	}
	title := padRight(truncRunes(item.Title, avail), avail)
	return []string{
		gry + "[" + t + "]" + rst + " " + gry + prefix + title + rst + " " + gry + metaPlain + rst,
		"  " + gry + itemURL(item) + rst,
		"  " + gry + "Comments: " + commentURL(item.ID) + rst,
		"",
	}
}

// appendEntry appends a 4-line entry to feedBuf, increments totalItems, and
// advances scroll by 4 when the user is scrolled up, keeping the viewport stable.
func appendEntry(feedBuf *[]string, lines []string, scroll *int, totalItems *int) {
	*feedBuf = append(*feedBuf, lines...)
	*totalItems++
	if *scroll > 0 {
		*scroll += 4
	}
}
