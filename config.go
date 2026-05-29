package main

import (
	"encoding/json"
	"os"

	tea "github.com/charmbracelet/bubbletea"
)

// ── Configuration ─────────────────────────────────────────────────────────────

const settingsFile = "hnfeed-settings.json"

type feedConfig struct {
	ShowFrontPage      bool   `json:"show_front_page"`
	ShowNewStories     bool   `json:"show_new_stories"`
	FrontEntered       bool   `json:"front_entered"`
	FrontRankUp        bool   `json:"front_rank_up"`
	FrontRankUpPeak    bool   `json:"front_rank_up_peak"`
	FrontRankDown      bool   `json:"front_rank_down"`
	FrontRankDownWorst bool   `json:"front_rank_down_worst"`
	FrontLeft          bool   `json:"front_left"`
	PollSeconds        int    `json:"poll_seconds"`
	InitialItems       int    `json:"initial_items"`
	ThreadsUser        string `json:"threads_user"`
}

func loadSettings() feedConfig {
	cfg := feedConfig{
		ShowFrontPage:      true,
		ShowNewStories:     true,
		FrontEntered:       true,
		FrontRankUp:        true,
		FrontRankUpPeak:    true,
		FrontRankDownWorst: true,
		PollSeconds:        30,
		InitialItems:       5,
	}
	data, err := os.ReadFile(settingsFile)
	if err != nil {
		return cfg
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return feedConfig{
			ShowFrontPage:      true,
			ShowNewStories:     true,
			FrontEntered:       true,
			FrontRankUp:        true,
			FrontRankUpPeak:    true,
			FrontRankDownWorst: true,
			PollSeconds:        30,
			InitialItems:       5,
		}
	}
	if cfg.PollSeconds < 5 {
		cfg.PollSeconds = 30
	}
	if cfg.InitialItems < 1 {
		cfg.InitialItems = 5
	}
	return cfg
}

func saveSettings(cfg feedConfig) {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return
	}
	os.WriteFile(settingsFile, data, 0644)
}

// ── Config field descriptors ──────────────────────────────────────────────────

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
