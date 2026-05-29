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

// ── Pages ────────────────────────────────────────────────────────────────────

type page int

const (
	pageFeed page = iota
	pageThreads
)

// ── Model ─────────────────────────────────────────────────────────────────────

type model struct {
	st              feedState
	threads         threadsState
	width           int
	height          int
	pollSec         int
	lastPoll        time.Time
	ready           bool
	config          feedConfig
	configOpen      bool
	configCur       int
	lastThreadsUser string
	page            page
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
	cfgFrontRankUpPeak
	cfgFPRankDown
	cfgFrontRankDownWorst
	cfgFPLeft
	cfgNSToggle
	cfgPollSlider
	cfgInitItems
	cfgThreadsUser
)

func (m model) configFields() []cfgField {
	fields := []cfgField{cfgFPToggle}
	if m.config.ShowFrontPage {
		fields = append(fields, cfgFPEntered, cfgFPRankUp)
		if m.config.FrontRankUp {
			fields = append(fields, cfgFrontRankUpPeak)
		}
		fields = append(fields, cfgFPRankDown)
		if m.config.FrontRankDown {
			fields = append(fields, cfgFrontRankDownWorst)
		}
		fields = append(fields, cfgFPLeft)
	}
	fields = append(fields, cfgNSToggle, cfgPollSlider, cfgInitItems, cfgThreadsUser)
	return fields
}

// maybeRefreshThreads checks if ThreadsUser has changed and triggers a refetch
// or reset if so. Returns a command if a fetch is needed, nil otherwise.
func (m *model) maybeRefreshThreads() tea.Cmd {
	if m.config.ThreadsUser == m.lastThreadsUser {
		return nil
	}
	m.lastThreadsUser = m.config.ThreadsUser
	if m.config.ThreadsUser == "" {
		m.threads.reset()
		return nil
	}
	m.threads.reset()
	m.threads.loading = true
	return fetchThreadsCmd(m.config.ThreadsUser)
}

// ── Update ────────────────────────────────────────────────────────────────────

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		return m.handleWindowSizeMsg(msg)
	case tea.KeyMsg:
		return m.handleKeyMsg(msg)
	case tea.MouseMsg:
		return m.handleMouseMsg(msg)
	case seedResultMsg:
		return m.handleSeedResult(msg)
	case tickMsg:
		return m.handleTickMsg(msg)
	case pollResultMsg:
		return m.handlePollResult(msg)
	case threadsResultMsg:
		return m.handleThreadsResult(msg)
	}
	return m, nil
}

// ── Message handlers ──────────────────────────────────────────────────────────

func (m *model) handleWindowSizeMsg(msg tea.WindowSizeMsg) (tea.Model, tea.Cmd) {
	m.width = msg.Width
	m.height = msg.Height
	if m.threads.loaded && m.threads.forest != nil {
		w := m.threadContentWidth()
		m.threads.flatLines = flattenForest(m.threads.forest, w)
		m.threads.lastWidth = w
	}
	return m, nil
}

func (m *model) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if handled, cmd := m.handleGlobalKey(msg); handled {
		return m, cmd
	}
	if !m.configOpen && m.page == pageThreads {
		return m, m.handleThreadsKey(msg)
	}
	if m.configOpen {
		return m, m.handleConfigKey(msg)
	}
	return m, nil
}

func (m *model) handleGlobalKey(msg tea.KeyMsg) (bool, tea.Cmd) {
	if msg.Type == tea.KeyCtrlC {
		return true, tea.Quit
	}
	if msg.Type == tea.KeyF1 || msg.Type == tea.KeyCtrlF {
		m.page = pageFeed
		m.configOpen = false
		return true, m.maybeRefreshThreads()
	}
	if msg.Type == tea.KeyF2 || msg.Type == tea.KeyCtrlT {
		m.page = pageThreads
		m.configOpen = false
		if m.config.ThreadsUser != "" && !m.threads.loaded {
			m.lastThreadsUser = m.config.ThreadsUser
			m.threads.reset()
			m.threads.loading = true
			return true, fetchThreadsCmd(m.config.ThreadsUser)
		}
		return true, m.maybeRefreshThreads()
	}
	if msg.Type == tea.KeyF10 || string(msg.Runes) == "?" {
		m.configOpen = !m.configOpen
		m.configCur = 0
		return true, nil
	}
	return false, nil
}

func (m *model) handleThreadsKey(msg tea.KeyMsg) tea.Cmd {
	if len(msg.Runes) > 0 && (msg.Runes[0] == 'r' || msg.Runes[0] == 'R') {
		if m.config.ThreadsUser != "" {
			m.threads.reset()
			m.threads.loading = true
			return fetchThreadsCmd(m.config.ThreadsUser)
		}
		return nil
	}

	if m.threads.loaded && len(m.threads.flatLines) > 0 {
		curLine := findNodeLine(m.threads.flatLines, m.threads.cursor)
		if curLine < 0 {
			curLine = 0
			if len(m.threads.flatLines) > 0 {
				m.threads.cursor = m.threads.flatLines[0].nodeIdx
			}
		}

		contentH := m.contentHeight()

		if msg.Type == tea.KeyEnter || msg.Type == tea.KeySpace || (len(msg.Runes) > 0 && msg.Runes[0] == ' ') {
			threadW := m.threadContentWidth()
			m.threads.toggleCollapse(m.threads.cursor, threadW)
			m.clampThreadScroll(contentH)
			return nil
		}

		switch {
		case msg.Type == tea.KeyLeft:
			threadW := m.threadContentWidth()
			m.threads.setCollapse(m.threads.cursor, true, threadW)
			m.clampThreadScroll(contentH)
		case msg.Type == tea.KeyRight:
			threadW := m.threadContentWidth()
			m.threads.setCollapse(m.threads.cursor, false, threadW)
			m.clampThreadScroll(contentH)
		case msg.Type == tea.KeyUp:
			newIdx := prevVisibleNode(m.threads.flatLines, curLine)
			if newIdx >= 0 {
				m.threads.cursor = newIdx
				newLine := findNodeLine(m.threads.flatLines, newIdx)
				if newLine < m.threads.scroll {
					m.threads.scroll = newLine
				}
			}
		case msg.Type == tea.KeyDown:
			newIdx := nextVisibleNode(m.threads.flatLines, curLine)
			if newIdx >= 0 {
				m.threads.cursor = newIdx
				newLine := findNodeLine(m.threads.flatLines, newIdx)
				if newLine >= m.threads.scroll+contentH {
					m.threads.scroll = newLine - contentH + 1
				}
			}
		case msg.Type == tea.KeyPgUp:
			m.threads.scroll -= contentH
			if m.threads.scroll < 0 {
				m.threads.scroll = 0
			}
			if len(m.threads.flatLines) > 0 {
				m.threads.cursor = m.threads.flatLines[m.threads.scroll].nodeIdx
			}
		case msg.Type == tea.KeyPgDown:
			m.threads.scroll += contentH
			m.clampThreadScroll(contentH)
			if len(m.threads.flatLines) > 0 {
				m.threads.cursor = m.threads.flatLines[m.threads.scroll].nodeIdx
			}
		case msg.Type == tea.KeyHome:
			m.threads.scroll = 0
			if len(m.threads.flatLines) > 0 {
				m.threads.cursor = m.threads.flatLines[0].nodeIdx
			}
		case msg.Type == tea.KeyEnd:
			maxS := len(m.threads.flatLines) - contentH
			if maxS < 0 {
				maxS = 0
			}
			m.threads.scroll = maxS
			if len(m.threads.flatLines) > 0 {
				m.threads.cursor = m.threads.flatLines[len(m.threads.flatLines)-1].nodeIdx
			}
		}
	}
	return nil
}

func (m *model) handleConfigKey(msg tea.KeyMsg) tea.Cmd {
	fields := m.configFields()

	if msg.Type == tea.KeyUp {
		if m.configCur > 0 {
			m.configCur--
		}
		return nil
	}
	if msg.Type == tea.KeyDown {
		if m.configCur < len(fields)-1 {
			m.configCur++
		}
		return nil
	}

	if m.configCur < len(fields) && fields[m.configCur] == cfgThreadsUser {
		switch {
		case msg.Type == tea.KeyBackspace:
			if len(m.config.ThreadsUser) > 0 {
				m.config.ThreadsUser = m.config.ThreadsUser[:len(m.config.ThreadsUser)-1]
				saveSettings(m.config)
			}
		case msg.Type == tea.KeyEsc:
			m.configOpen = false
			return m.maybeRefreshThreads()
		case len(msg.Runes) > 0:
			m.config.ThreadsUser += string(msg.Runes)
			saveSettings(m.config)
		}
		return nil
	}

	switch {
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
			if !m.config.FrontRankUp {
				m.config.FrontRankUpPeak = false
				newFields := m.configFields()
				if m.configCur >= len(newFields) {
					m.configCur = len(newFields) - 1
				}
			}
		case cfgFrontRankUpPeak:
			m.config.FrontRankUpPeak = !m.config.FrontRankUpPeak
		case cfgFPRankDown:
			m.config.FrontRankDown = !m.config.FrontRankDown
			if !m.config.FrontRankDown {
				m.config.FrontRankDownWorst = false
				newFields := m.configFields()
				if m.configCur >= len(newFields) {
					m.configCur = len(newFields) - 1
				}
			}
		case cfgFrontRankDownWorst:
			m.config.FrontRankDownWorst = !m.config.FrontRankDownWorst
		case cfgFPLeft:
			m.config.FrontLeft = !m.config.FrontLeft
		case cfgNSToggle:
			m.config.ShowNewStories = !m.config.ShowNewStories
		}
		saveSettings(m.config)
	case msg.Type == tea.KeyEsc:
		m.configOpen = false
		return m.maybeRefreshThreads()
	}
	return nil
}

func (m *model) handleMouseMsg(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	if msg.Action == tea.MouseActionPress {
		if m.page == pageFeed {
			switch msg.Button {
			case tea.MouseButtonWheelUp:
				m.st.scroll += 4
			case tea.MouseButtonWheelDown:
				m.st.scroll -= 4
				if m.st.scroll < 0 {
					m.st.scroll = 0
				}
			}
		} else if m.page == pageThreads {
			contentH := m.contentHeight()
			switch msg.Button {
			case tea.MouseButtonWheelUp:
				m.threads.scroll -= 3
				if m.threads.scroll < 0 {
					m.threads.scroll = 0
				}
			case tea.MouseButtonWheelDown:
				m.threads.scroll += 3
				m.clampThreadScroll(contentH)
			}
		}
	}
	return m, nil
}

func (m *model) handleSeedResult(msg seedResultMsg) (tea.Model, tea.Cmd) {
	for id, rank := range msg.frontRanks {
		m.st.frontRanks[id] = rank
		m.st.frontBestRanks[id] = rank
		m.st.frontWorstRanks[id] = rank
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
			m.st.appendEntry(feedEntry{
				typ:    entryFrontEnter,
				item:   item,
				prefix: fmt.Sprintf("★ #%d  ", msg.frontRanks[item.ID]),
			})
		}
	}
	for _, item := range msg.newItems {
		if item == nil {
			continue
		}
		m.st.seenIDs[item.ID] = true
		if m.config.ShowNewStories {
			m.st.appendEntry(feedEntry{typ: entryNew, item: item})
		}
		if item.ID > m.st.maxID {
			m.st.maxID = item.ID
		}
	}

	m.lastPoll = time.Now()
	m.ready = true
	return m, nil
}

func (m *model) handleTickMsg(msg tickMsg) (tea.Model, tea.Cmd) {
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
}

func (m *model) handlePollResult(msg pollResultMsg) (tea.Model, tea.Cmd) {
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
				m.st.appendEntry(feedEntry{typ: entryNew, item: item})
			}
		}
	}

	for id := range m.st.seenIDs {
		if id <= oldMaxID {
			delete(m.st.seenIDs, id)
		}
	}

	if msg.newFrontRanks != nil {
		if m.config.ShowFrontPage && m.config.FrontLeft {
			for id, oldRank := range m.st.frontRanks {
				if _, stillOn := msg.newFrontRanks[id]; !stillOn {
					if item, ok := m.st.frontCache[id]; ok {
						m.st.appendEntry(feedEntry{
							typ:     entryFrontLeave,
							item:    item,
							oldRank: oldRank,
						})
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
					if m.config.FrontRankUpPeak {
						bestRank, hasBest := m.st.frontBestRanks[id]
						if !hasBest {
							bestRank = oldRank
						}
						if newRank < bestRank {
							oldBest := bestRank
							m.st.frontBestRanks[id] = newRank
							m.st.appendEntry(feedEntry{
								typ:    entryFrontUp,
								item:   item,
								prefix: fmt.Sprintf("↑ #%d (best #%d)  ", newRank, oldBest),
							})
						}
					} else {
						m.st.appendEntry(feedEntry{
							typ:    entryFrontUp,
							item:   item,
							prefix: fmt.Sprintf("↑ #%d (was #%d)  ", newRank, oldRank),
						})
					}
				} else if newRank > oldRank && m.config.ShowFrontPage && m.config.FrontRankDown {
					if m.config.FrontRankDownWorst {
						worstRank, hasWorst := m.st.frontWorstRanks[id]
						if !hasWorst {
							worstRank = oldRank
						}
						if newRank > worstRank {
							oldWorst := worstRank
							m.st.frontWorstRanks[id] = newRank
							m.st.appendEntry(feedEntry{
								typ:    entryFrontDown,
								item:   item,
								prefix: fmt.Sprintf("↓ #%d (worst #%d)  ", newRank, oldWorst),
							})
						}
					} else {
						m.st.appendEntry(feedEntry{
							typ:    entryFrontDown,
							item:   item,
							prefix: fmt.Sprintf("↓ #%d (was #%d)  ", newRank, oldRank),
						})
					}
				}
			} else if !m.st.seenIDs[id] {
				if m.config.ShowFrontPage && m.config.FrontEntered {
					m.st.appendEntry(feedEntry{
						typ:    entryFrontEnter,
						item:   item,
						prefix: fmt.Sprintf("★ #%d  ", newRank),
					})
				}
			}
			if _, has := m.st.frontBestRanks[id]; !has || newRank < m.st.frontBestRanks[id] {
				m.st.frontBestRanks[id] = newRank
			}
			if _, has := m.st.frontWorstRanks[id]; !has || newRank > m.st.frontWorstRanks[id] {
				m.st.frontWorstRanks[id] = newRank
			}
			m.st.frontCache[id] = item
		}

		for id := range m.st.frontRanks {
			if _, stillOn := msg.newFrontRanks[id]; !stillOn {
				delete(m.st.frontCache, id)
				delete(m.st.frontBestRanks, id)
				delete(m.st.frontWorstRanks, id)
			}
		}

		m.st.frontRanks = msg.newFrontRanks
	}

	maxEntries := 500
	if len(m.st.entries) > maxEntries {
		trim := len(m.st.entries) - maxEntries
		m.st.entries = m.st.entries[trim:]
		if m.st.scroll > 0 {
			m.st.scroll -= trim * 4
			if m.st.scroll < 0 {
				m.st.scroll = 0
			}
		}
	}

	return m, nil
}

func (m *model) handleThreadsResult(msg threadsResultMsg) (tea.Model, tea.Cmd) {
	m.threads.applyResult(msg)
	if m.threads.loaded && m.threads.forest != nil {
		w := m.threadContentWidth()
		m.threads.flatLines = flattenForest(m.threads.forest, w)
		m.threads.lastWidth = w
		for _, li := range m.threads.flatLines {
			if li.nodeIdx >= 0 {
				m.threads.cursor = li.nodeIdx
				break
			}
		}
	}
	return m, nil
}

// ── Helper methods ────────────────────────────────────────────────────────────

func (m *model) threadContentWidth() int {
	w := m.width - 4
	if w < 10 {
		w = 80
	}
	return w
}

func (m *model) contentHeight() int {
	h := m.height - 2 - 2 // minus header, status, border top/bottom
	if h < 1 {
		h = 1
	}
	return h
}

func (m *model) clampThreadScroll(contentH int) {
	maxS := len(m.threads.flatLines) - contentH
	if maxS < 0 {
		maxS = 0
	}
	if m.threads.scroll > maxS {
		m.threads.scroll = maxS
	}
}
