package main

import (
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	initialItems := flag.Int("initialItems", 5, "stories loaded from each source on startup")
	throttleLimit := flag.Int("throttleLimit", 10, "max parallel item fetches")
	flag.Parse()

	cfg := loadSettings()
	m := model{
		st: feedState{
			frontRanks: make(map[int]int),
			frontCache: make(map[int]*Item),
			seenIDs:    make(map[int]bool),
		},
		config:   cfg,
		pollSec:  cfg.PollSeconds,
		throttle: *throttleLimit,
		initial:  *initialItems,
	}

	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Stopped watching HN.")
}
