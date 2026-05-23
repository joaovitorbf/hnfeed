package main

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// ── Lipgloss styles ───────────────────────────────────────────────────────────

var (
	yellowStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	grayStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	orangeStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("208"))
	boldStyle   = lipgloss.NewStyle().Bold(true)
	titleStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6"))
)

// ── ANSI-safe text helpers ────────────────────────────────────────────────────

// fit pads or truncates s so its visible width equals exactly width.
// ANSI escape sequences are preserved and not counted toward visible width.
func fit(s string, width int) string {
	if width <= 0 {
		return ""
	}
	w := lipgloss.Width(s)
	if w < width {
		return s + strings.Repeat(" ", width-w)
	}
	if w == width {
		return s
	}
	if width <= 1 {
		return "…"
	}
	return lipgloss.NewStyle().MaxWidth(width-1).Render(s) + "…"
}

// truncPad truncates s to at most n visible cells (adding … if truncated)
// and right-pads to exactly n visible cells.
func truncPad(s string, n int) string {
	if n <= 0 {
		return ""
	}
	w := lipgloss.Width(s)
	if w <= n {
		return lipgloss.NewStyle().Width(n).Render(s)
	}
	if n <= 1 {
		return "…"
	}
	return lipgloss.NewStyle().MaxWidth(n-1).Render(s) + "…"
}
