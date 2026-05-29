package main

import (
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
