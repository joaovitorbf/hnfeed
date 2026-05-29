package main

import (
	"sync"

	tea "github.com/charmbracelet/bubbletea"
)

// ── Commands ──────────────────────────────────────────────────────────────────

// seedFeedCmd fetches the initial front page and newest stories.
func (m model) seedFeedCmd() tea.Cmd {
	return func() tea.Msg {
		throttle := 10
		initial := m.config.InitialItems

		var frontItems []*Item
		var frontRanks map[int]int
		var newItems []*Item

		var wg sync.WaitGroup
		wg.Add(2)

		go func() {
			defer wg.Done()
			if items, ranks, err := fetchFrontPage(throttle); err == nil {
				frontItems = items
				frontRanks = ranks
			}
		}()

		go func() {
			defer wg.Done()
			if ids, err := fetchNewStoryIDs(); err == nil && len(ids) > 0 {
				count := initial
				if count > len(ids) {
					count = len(ids)
				}
				newItems = fetchItemsParallel(ids[:count], throttle)
			}
		}()

		wg.Wait()

		return seedResultMsg{
			frontItems: frontItems,
			frontRanks: frontRanks,
			newItems:   newItems,
		}
	}
}

// pollNowCmd fetches new stories and the current front page for a periodic refresh.
func (m model) pollNowCmd() tea.Cmd {
	return func() tea.Msg {
		throttle := 10

		var newItems []*Item
		newMaxID := m.st.maxID
		if ids, err := fetchNewStoryIDs(); err == nil {
			if len(ids) > 0 && ids[0] > newMaxID {
				newMaxID = ids[0]
			}
			var pendingIDs []int
			for _, id := range ids {
				if id > m.st.maxID {
					pendingIDs = append(pendingIDs, id)
				}
			}
			newItems = fetchItemsParallel(pendingIDs, throttle)
		}

		var frontItems []*Item
		var newFrontRanks map[int]int
		if items, ranks, err := fetchFrontPage(throttle); err == nil {
			frontItems = items
			newFrontRanks = ranks
		}

		return pollResultMsg{
			newItems:      newItems,
			frontItems:    frontItems,
			newFrontRanks: newFrontRanks,
			newMaxID:      newMaxID,
		}
	}
}
