package main

import (
	"encoding/json"
	"os"
)

// ── Configuration ─────────────────────────────────────────────────────────────

const settingsFile = "hnfeed-settings.json"

type feedConfig struct {
	ShowFrontPage  bool `json:"show_front_page"`
	ShowNewStories bool `json:"show_new_stories"`
	FrontEntered   bool `json:"front_entered"`
	FrontRankUp    bool `json:"front_rank_up"`
	FrontRankDown  bool `json:"front_rank_down"`
	FrontLeft      bool `json:"front_left"`
	PollSeconds    int  `json:"poll_seconds"`
}

func loadSettings() feedConfig {
	cfg := feedConfig{
		ShowFrontPage:  true,
		ShowNewStories: true,
		FrontEntered:   true,
		FrontRankUp:    true,
		PollSeconds:    30,
	}
	data, err := os.ReadFile(settingsFile)
	if err != nil {
		return cfg
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return feedConfig{ShowFrontPage: true, ShowNewStories: true, PollSeconds: 30}
	}
	if cfg.PollSeconds < 5 {
		cfg.PollSeconds = 30
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
