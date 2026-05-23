package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {

	cfg := loadSettings()
	m := model{
		st: feedState{
			frontRanks:      make(map[int]int),
			frontBestRanks:  make(map[int]int),
			frontWorstRanks: make(map[int]int),
			frontCache:      make(map[int]*Item),
			seenIDs:         make(map[int]bool),
		},
		config:  cfg,
		pollSec: cfg.PollSeconds,
	}

	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Stopped watching HN.")
}
