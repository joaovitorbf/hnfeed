package main

import (
	"encoding/json"
	"os"
)

// ── Configuration ─────────────────────────────────────────────────────────────

const settingsFile = "hnfeed-settings.json"

type feedConfig struct {
	ShowFrontPage      bool `json:"show_front_page"`
	ShowNewStories     bool `json:"show_new_stories"`
	FrontEntered       bool `json:"front_entered"`
	FrontRankUp        bool `json:"front_rank_up"`
	FrontRankUpPeak    bool `json:"front_rank_up_peak"`
	FrontRankDown      bool `json:"front_rank_down"`
	FrontRankDownWorst bool `json:"front_rank_down_worst"`
	FrontLeft          bool `json:"front_left"`
	PollSeconds        int  `json:"poll_seconds"`
	InitialItems       int  `json:"initial_items"`
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
