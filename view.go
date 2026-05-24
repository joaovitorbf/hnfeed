package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// ── View ──────────────────────────────────────────────────────────────────────

func (m model) View() string {
	w, h := m.width, m.height
	if w < 10 {
		w = 80
	}
	if h < 4 {
		h = 24
	}

	// Content area: between title (1) and status bar (1)
	contentH := h - 2
	if contentH < 3 {
		contentH = 3
	}

	var buf strings.Builder

	scrollHint := grayStyle.Render("(live)")
	if m.st.scroll > 0 {
		scrollHint = grayStyle.Render("[↑ scrolled]")
	}
	if m.configOpen {
		scrollHint = grayStyle.Render("(settings)")
	}
	buf.WriteString(fit(titleStyle.Render(" HN Feed ")+scrollHint, w))
	buf.WriteByte('\n')

	if m.configOpen {
		// ── Settings panel (has its own border) ──
		cfgLines := m.buildConfigLines(w)
		for row := 0; row < contentH; row++ {
			var line string
			if row < len(cfgLines) {
				line = cfgLines[row]
			}
			buf.WriteString(fit(line, w))
			buf.WriteByte('\n')
		}
	} else {
		// ── Feed panel with rounded border ──
		innerW := w - 4
		if innerW < 1 {
			innerW = 1
		}
		innerH := contentH - 2
		if innerH < 1 {
			innerH = 1
		}

		totalLines := m.st.totalLines()
		maxScroll := totalLines - innerH
		if maxScroll < 0 {
			maxScroll = 0
		}
		sc := m.st.scroll
		if sc > maxScroll {
			sc = maxScroll
		}
		startIdx := totalLines - innerH - sc
		if startIdx < 0 {
			startIdx = 0
		}

		var inner []string
		hasContent := false
		for row := 0; row < innerH; row++ {
			li := startIdx + row
			line := ""
			entryIdx := li / 4
			lineIdx := li % 4
			if entryIdx < len(m.st.entries) {
				entry := m.st.entries[entryIdx]
				lines := m.formatEntry(entry, innerW)
				line = lines[lineIdx]
				hasContent = true
			}
			inner = append(inner, fit(line, innerW))
		}

		// Show a subtle placeholder while the initial data loads
		if !hasContent && !m.ready {
			mid := innerH / 2
			inner[mid] = fit(dimStyle.Render("  Fetching data…  "), innerW)
		}

		content := strings.Join(inner, "\n")
		panel := feedBorder.Render(content)
		panelLines := strings.Split(panel, "\n")
		for _, line := range panelLines {
			buf.WriteString(fit(line, w))
			buf.WriteByte('\n')
		}
	}

	// ── Status bar ──
	status := m.statusText()
	buf.WriteString(
		statusBarStyle.Width(w).Render("  " + status),
	)

	return buf.String()
}

// formatEntry renders a feedEntry into 4 ANSI-styled lines at the given width.
func (m model) formatEntry(e feedEntry, width int) []string {
	switch e.typ {
	case entryNew:
		return formatNewItemLines(e.item, width)
	case entryFrontEnter:
		return formatFrontEventLines(e.item, e.prefix, width)
	case entryFrontUp:
		return formatFrontEventLines(e.item, e.prefix, width)
	case entryFrontDown:
		return formatFrontEventLines(e.item, e.prefix, width)
	case entryFrontLeave:
		return formatFrontLeaveLine(e.item, e.oldRank, width)
	}
	return []string{"", "", "", ""}
}

func (m model) buildConfigLines(w int) []string {
	fields := m.configFields()
	innerW := w - 4
	if innerW < 30 {
		innerW = 30
	}

	// Separate event toggles from numeric fields
	var eventFields, numericFields []cfgField
	for _, f := range fields {
		if f == cfgPollSlider || f == cfgInitItems {
			numericFields = append(numericFields, f)
		} else {
			eventFields = append(eventFields, f)
		}
	}

	fieldIdx := func(f cfgField) int {
		for i, v := range fields {
			if v == f {
				return i
			}
		}
		return -1
	}

	var inner []string

	// ── Section: Events ──
	inner = append(inner, sectionDivider("Events", innerW))
	inner = append(inner, "")

	for _, f := range eventFields {
		idx := fieldIdx(f)
		line := m.configFieldLine(f, idx, m.configCur == idx, innerW)
		inner = append(inner, line)
	}

	inner = append(inner, "")

	// ── Section: Feed Settings ──
	inner = append(inner, sectionDivider("Feed Settings", innerW))
	inner = append(inner, "")

	for _, f := range numericFields {
		idx := fieldIdx(f)
		line := m.configFieldLine(f, idx, m.configCur == idx, innerW)
		inner = append(inner, line)
	}

	inner = append(inner, "")

	// ── Help ──
	inner = append(inner, buildHelpLine(innerW))

	// Pad all lines to innerW
	for i, line := range inner {
		if w2 := lipgloss.Width(line); w2 < innerW {
			inner[i] = line + strings.Repeat(" ", innerW-w2)
		}
	}

	content := strings.Join(inner, "\n")
	panel := configBorder.Render(content)
	return strings.Split(panel, "\n")
}

// sectionDivider returns a centered section label flanked by dashes.
func sectionDivider(label string, width int) string {
	label = " " + label + " "
	labelW := lipgloss.Width(label)
	avail := width - 2 // "  " prefix
	dashes := avail - labelW
	if dashes < 0 {
		dashes = 0
	}
	left := dashes / 2
	right := dashes - left
	return "  " + dimStyle.Render(strings.Repeat("─", left)) + configSection.Render(label) + dimStyle.Render(strings.Repeat("─", right))
}

// buildHelpLine returns the styled help bar at the bottom of settings.
func buildHelpLine(width int) string {
	parts := []string{
		helpKeyStyle.Render("↑↓") + " " + helpTextStyle.Render("navigate"),
		helpKeyStyle.Render("Space") + " " + helpTextStyle.Render("toggle"),
		helpKeyStyle.Render("←→") + " " + helpTextStyle.Render("adjust"),
		helpKeyStyle.Render("Esc") + " " + helpTextStyle.Render("close"),
	}
	line := "  " + strings.Join(parts, "  │  ")
	if w := lipgloss.Width(line); w < width {
		line += strings.Repeat(" ", width-w)
	}
	return line
}

// configFieldLine renders a single config field with cursor, checkbox, and padding.
func (m model) configFieldLine(f cfgField, idx int, isCursor bool, width int) string {
	// 2-char prefix matching the original layout so cursor/non-cursor align
	prefix := "  "
	if isCursor {
		prefix = "▸ "
	}

	var left string
	switch f {
	case cfgFPToggle:
		chk := checkboxStr(m.config.ShowFrontPage)
		left = "  " + prefix + chk + "  Front page events"
	case cfgFPEntered:
		chk := checkboxStr(m.config.FrontEntered)
		left = "    " + prefix + chk + "  Entered front page"
	case cfgFPRankUp:
		chk := checkboxStr(m.config.FrontRankUp)
		left = "    " + prefix + chk + "  Ranking up"
	case cfgFrontRankUpPeak:
		chk := checkboxStr(m.config.FrontRankUpPeak)
		left = "      " + prefix + chk + "  Compare to best rank"
	case cfgFPRankDown:
		chk := checkboxStr(m.config.FrontRankDown)
		left = "    " + prefix + chk + "  Ranking down"
	case cfgFrontRankDownWorst:
		chk := checkboxStr(m.config.FrontRankDownWorst)
		left = "      " + prefix + chk + "  Compare to worst rank"
	case cfgFPLeft:
		chk := checkboxStr(m.config.FrontLeft)
		left = "    " + prefix + chk + "  Left front page"
	case cfgNSToggle:
		chk := checkboxStr(m.config.ShowNewStories)
		left = "  " + prefix + chk + "  New story events"
	case cfgPollSlider:
		left = "  " + prefix + "Poll interval"
	case cfgInitItems:
		left = "  " + prefix + "Initial items"
	}

	var full string
	if f == cfgPollSlider || f == cfgInitItems {
		val := fmt.Sprintf("%d", m.config.PollSeconds)
		if f == cfgInitItems {
			val = fmt.Sprintf("%d", m.config.InitialItems)
		} else {
			val += "s"
		}
		value := valStyle.Render(val)
		arrows := "  " + arrowStyle.Render("◀") + "  " + arrowStyle.Render("▶")

		// Right-align styled value within a minimum 4-wide field so the
		// arrows start at the same column on both rows ("30s" vs "5").
		minValW := 4
		if vw := lipgloss.Width(value); vw < minValW {
			value = strings.Repeat(" ", minValW-vw) + value
		}

		// Small fixed gap between label and value; push remaining padding
		// to the right so the value doesn't drift far right on wide terminals.
		gap := 3
		body := left + strings.Repeat(" ", gap) + value + arrows
		if w := lipgloss.Width(body); w > width {
			excess := w - width
			if gap > excess {
				gap -= excess
			} else {
				gap = 0
			}
			body = left + strings.Repeat(" ", gap) + value + arrows
		}
		pad := width - lipgloss.Width(body)
		if pad < 0 {
			pad = 0
		}
		full = body + strings.Repeat(" ", pad)
	} else {
		pad := width - lipgloss.Width(left)
		if pad < 0 {
			pad = 0
		}
		full = left + strings.Repeat(" ", pad)
	}

	if isCursor {
		full = cursorStyle.Render(full)
	}

	return full
}

// checkboxStr returns a styled checkbox string using manual ANSI that only
// sets/resets foreground and bold, so any outer background (cursor highlight)
// is preserved rather than wiped by a full \033[0m reset.
func checkboxStr(on bool) string {
	if on {
		return greenCheck + "[✓]" + resetFgBold
	}
	return grayCheck + "[✗]" + resetFgBold
}

func (m model) statusText() string {
	if !m.ready {
		return "Fetching data…  │  Ctrl+C to quit"
	}
	next := m.lastPoll.Add(time.Duration(m.pollSec) * time.Second)
	remaining := int(time.Until(next).Seconds())
	if remaining < 1 {
		remaining = 1
	}
	var tStr string
	if remaining > 60 {
		tStr = fmt.Sprintf("%dm %ds", remaining/60, remaining%60)
	} else {
		tStr = fmt.Sprintf("%ds", remaining)
	}
	return fmt.Sprintf("Next refresh in %s  │  Items seen: %d  │  ?/F1 settings  │  Ctrl+C to quit", tStr, m.st.totalItems)
}
