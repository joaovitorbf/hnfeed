package main

import (
	"fmt"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// ── Messages ──────────────────────────────────────────────────────────────────

// tickMsg fires every 100 ms to drive the poll timer.
type tickMsg struct{}

// seedResultMsg carries the result of the initial data load.
type seedResultMsg struct {
	frontItems []*Item
	frontRanks map[int]int
	newItems   []*Item
}

// pollResultMsg carries the result of a periodic refresh.
type pollResultMsg struct {
	newItems      []*Item
	frontItems    []*Item
	newFrontRanks map[int]int
	newMaxID      int
}

// ── Model ─────────────────────────────────────────────────────────────────────

type model struct {
	st         feedState
	width      int
	height     int
	pollSec    int
	lastPoll   time.Time
	ready      bool
	config     feedConfig
	configOpen bool
	configCur  int
}

func (m model) Init() tea.Cmd {
	return tea.Batch(
		m.seedFeedCmd(),
		tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
			return tickMsg{}
		}),
	)
}

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

// ── Config field helpers ──────────────────────────────────────────────────────

type cfgField int

const (
	cfgFPToggle cfgField = iota
	cfgFPEntered
	cfgFPRankUp
	cfgFPRankDown
	cfgFPLeft
	cfgNSToggle
	cfgPollSlider
	cfgInitItems
)

func (m model) configFields() []cfgField {
	fields := []cfgField{cfgFPToggle}
	if m.config.ShowFrontPage {
		fields = append(fields, cfgFPEntered, cfgFPRankUp, cfgFPRankDown, cfgFPLeft)
	}
	fields = append(fields, cfgNSToggle, cfgPollSlider, cfgInitItems)
	return fields
}

// ── Update ────────────────────────────────────────────────────────────────────

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		if msg.Type == tea.KeyCtrlC {
			return m, tea.Quit
		}
		// Toggle config overlay with F1 or ?
		if msg.Type == tea.KeyF1 || string(msg.Runes) == "?" {
			m.configOpen = !m.configOpen
			m.configCur = 0
			return m, nil
		}
		if m.configOpen {
			fields := m.configFields()
			switch {
			case msg.Type == tea.KeyUp:
				if m.configCur > 0 {
					m.configCur--
				}
			case msg.Type == tea.KeyDown:
				if m.configCur < len(fields)-1 {
					m.configCur++
				}
			case msg.Type == tea.KeyLeft, string(msg.Runes) == "-":
				if fields[m.configCur] == cfgPollSlider {
					m.config.PollSeconds -= 5
					if m.config.PollSeconds < 5 {
						m.config.PollSeconds = 5
					}
					m.pollSec = m.config.PollSeconds
					saveSettings(m.config)
				} else if fields[m.configCur] == cfgInitItems {
					m.config.InitialItems--
					if m.config.InitialItems < 1 {
						m.config.InitialItems = 1
					}
					saveSettings(m.config)
				}
			case msg.Type == tea.KeyRight, string(msg.Runes) == "=", string(msg.Runes) == "+":
				if fields[m.configCur] == cfgPollSlider {
					m.config.PollSeconds += 5
					if m.config.PollSeconds > 300 {
						m.config.PollSeconds = 300
					}
					m.pollSec = m.config.PollSeconds
					saveSettings(m.config)
				} else if fields[m.configCur] == cfgInitItems {
					m.config.InitialItems++
					if m.config.InitialItems > 50 {
						m.config.InitialItems = 50
					}
					saveSettings(m.config)
				}
			case msg.Type == tea.KeyEnter, string(msg.Runes) == " ":
				switch fields[m.configCur] {
				case cfgFPToggle:
					m.config.ShowFrontPage = !m.config.ShowFrontPage
					if !m.config.ShowFrontPage {
						newFields := m.configFields()
						if m.configCur >= len(newFields) {
							m.configCur = len(newFields) - 1
						}
					}
				case cfgFPEntered:
					m.config.FrontEntered = !m.config.FrontEntered
				case cfgFPRankUp:
					m.config.FrontRankUp = !m.config.FrontRankUp
				case cfgFPRankDown:
					m.config.FrontRankDown = !m.config.FrontRankDown
				case cfgFPLeft:
					m.config.FrontLeft = !m.config.FrontLeft
				case cfgNSToggle:
					m.config.ShowNewStories = !m.config.ShowNewStories
				}
				saveSettings(m.config)
			case msg.Type == tea.KeyEsc:
				m.configOpen = false
			}
			return m, nil
		}
		return m, nil

	case tea.MouseMsg:
		if msg.Action == tea.MouseActionPress {
			switch msg.Button {
			case tea.MouseButtonWheelUp:
				m.st.scroll += 4
			case tea.MouseButtonWheelDown:
				m.st.scroll -= 4
				if m.st.scroll < 0 {
					m.st.scroll = 0
				}
			}
		}
		return m, nil

	case seedResultMsg:
		w := m.width
		if w < 10 {
			w = 80
		}

		for id, rank := range msg.frontRanks {
			m.st.frontRanks[id] = rank
		}
		for _, item := range msg.frontItems {
			if item != nil {
				m.st.frontCache[item.ID] = item
			}
		}
		if m.config.ShowFrontPage && m.config.FrontEntered {
			for i, item := range msg.frontItems {
				if i >= m.config.InitialItems {
					break
				}
				m.st.appendEntry(formatFrontEventLines(item, fmt.Sprintf("★ #%d  ", msg.frontRanks[item.ID]), w))
			}
		}
		for _, item := range msg.newItems {
			if item == nil {
				continue
			}
			m.st.seenIDs[item.ID] = true
			if m.config.ShowNewStories {
				m.st.appendEntry(formatNewItemLines(item, w))
			}
			if item.ID > m.st.maxID {
				m.st.maxID = item.ID
			}
		}

		m.lastPoll = time.Now()
		m.ready = true
		return m, nil

	case tickMsg:
		if !m.ready {
			return m, tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
				return tickMsg{}
			})
		}

		cmds := []tea.Cmd{
			tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
				return tickMsg{}
			}),
		}
		if time.Since(m.lastPoll) >= time.Duration(m.pollSec)*time.Second {
			m.lastPoll = time.Now()
			cmds = append(cmds, m.pollNowCmd())
		}
		return m, tea.Batch(cmds...)

	case pollResultMsg:
		w := m.width
		if w < 10 {
			w = 80
		}

		// New stories
		oldMaxID := m.st.maxID
		if msg.newMaxID > m.st.maxID {
			m.st.maxID = msg.newMaxID
		}
		for _, item := range msg.newItems {
			if item == nil {
				continue
			}
			if !m.st.seenIDs[item.ID] {
				m.st.seenIDs[item.ID] = true
				if m.config.ShowNewStories {
					m.st.appendEntry(formatNewItemLines(item, w))
				}
			}
		}

		for id := range m.st.seenIDs {
			if id <= oldMaxID {
				delete(m.st.seenIDs, id)
			}
		}

		// Front page
		if msg.newFrontRanks != nil {
			// Detect items that left the front page
			if m.config.ShowFrontPage && m.config.FrontLeft {
				for id, oldRank := range m.st.frontRanks {
					if _, stillOn := msg.newFrontRanks[id]; !stillOn {
						if item, ok := m.st.frontCache[id]; ok {
							m.st.appendEntry(formatFrontLeaveLine(item, oldRank, w))
						}
					}
				}
			}

			for _, item := range msg.frontItems {
				if item == nil {
					continue
				}
				id := item.ID
				newRank := msg.newFrontRanks[id]
				if oldRank, exists := m.st.frontRanks[id]; exists {
					if newRank < oldRank && m.config.ShowFrontPage && m.config.FrontRankUp {
						m.st.appendEntry(formatFrontEventLines(item, fmt.Sprintf("↑ #%d (was #%d)  ", newRank, oldRank), w))
					} else if newRank > oldRank && m.config.ShowFrontPage && m.config.FrontRankDown {
						m.st.appendEntry(formatFrontEventLines(item, fmt.Sprintf("↓ #%d (was #%d)  ", newRank, oldRank), w))
					}
				} else if !m.st.seenIDs[id] {
					if m.config.ShowFrontPage && m.config.FrontEntered {
						m.st.appendEntry(formatFrontEventLines(item, fmt.Sprintf("★ #%d  ", newRank), w))
					}
				}
				// Update cache for items still on the front page
				m.st.frontCache[id] = item
			}

			// Clean up cache for items that left
			for id := range m.st.frontRanks {
				if _, stillOn := msg.newFrontRanks[id]; !stillOn {
					delete(m.st.frontCache, id)
				}
			}

			m.st.frontRanks = msg.newFrontRanks
		}

		if len(m.st.buf) > 2000 {
			trim := len(m.st.buf) - 2000
			m.st.buf = m.st.buf[trim:]
			if m.st.scroll > 0 {
				m.st.scroll -= trim
				if m.st.scroll < 0 {
					m.st.scroll = 0
				}
			}
		}

		return m, nil
	}

	return m, nil
}
