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
	if !m.configOpen && m.page == pageThreads {
		scrollHint = grayStyle.Render("(threads)")
	} else if m.st.scroll > 0 && !m.configOpen {
		scrollHint = grayStyle.Render("[↑ scrolled]")
	}
	if m.configOpen {
		scrollHint = grayStyle.Render("(settings)")
	}

	titleLeft := " HN Feed "
	if !m.configOpen && m.page == pageThreads {
		titleLeft = " HN Threads "
	}
	buf.WriteString(fit(titleStyle.Render(titleLeft)+scrollHint, w))
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
	} else if m.page == pageThreads {
		// ── Threads panel ──
		innerW := w - 4
		if innerW < 1 {
			innerW = 1
		}
		innerH := contentH - 2
		if innerH < 1 {
			innerH = 1
		}

		var inner []string
		if m.threads.loading {
			mid := innerH / 2
			for row := 0; row < innerH; row++ {
				l := ""
				if row == mid {
					l = fit(dimStyle.Render("  Loading threads…  "), innerW)
				}
				inner = append(inner, l)
			}
		} else if m.threads.err != "" {
			mid := innerH / 2
			for row := 0; row < innerH; row++ {
				l := ""
				if row == mid {
					l = fit(dimStyle.Render("  Error: "+m.threads.err+"  "), innerW)
				}
				inner = append(inner, l)
			}
		} else if !m.threads.loaded || m.threads.forest == nil || len(m.threads.flatLines) == 0 {
			if m.config.ThreadsUser == "" {
				mid := innerH / 2
				for row := 0; row < innerH; row++ {
					l := ""
					if row == mid {
						l = fit(dimStyle.Render("  No user configured — set in settings (?/F1)  "), innerW)
					} else if row == mid+1 {
						l = fit(dimStyle.Render("  Press F2 or Ctrl+T to try again  "), innerW)
					}
					inner = append(inner, l)
				}
			} else {
				mid := innerH / 2
				for row := 0; row < innerH; row++ {
					l := ""
					if row == mid {
						l = fit(dimStyle.Render("  No threads found for "+m.config.ThreadsUser+"  "), innerW)
					}
					inner = append(inner, l)
				}
			}
		} else {
			// Render visible lines
			totalLines := len(m.threads.flatLines)
			maxScroll := totalLines - innerH
			if maxScroll < 0 {
				maxScroll = 0
			}
			sc := m.threads.scroll
			if sc > maxScroll {
				m.threads.scroll = maxScroll
				sc = maxScroll
			}
			if sc < 0 {
				m.threads.scroll = 0
				sc = 0
			}

			for row := 0; row < innerH; row++ {
				li := sc + row
				line := ""
				if li < len(m.threads.flatLines) {
					info := m.threads.flatLines[li]
					line = info.text
					// Apply cursor highlight if this line belongs to the focused node
					if info.nodeIdx >= 0 && info.nodeIdx == m.threads.cursor {
						// Replace full ANSI resets with foreground-only resets so the
						// cursor background survives internal lipgloss styling.
						fixed := strings.ReplaceAll(info.text, "\033[0m", resetFgBold)
						line = "\033[48;5;237m" + fixed + "\033[49m"
					}
				}
				inner = append(inner, fit(line, innerW))
			}
		}

		content := strings.Join(inner, "\n")
		panel := feedBorder.Render(content)
		panelLines := strings.Split(panel, "\n")
		for _, line := range panelLines {
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

	// Separate event toggles from numeric/text fields
	var eventFields, numericFields, textFields []cfgField
	for _, f := range fields {
		switch f {
		case cfgPollSlider, cfgInitItems:
			numericFields = append(numericFields, f)
		case cfgThreadsUser:
			textFields = append(textFields, f)
		default:
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

	for _, f := range textFields {
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
	case cfgThreadsUser:
		left = "  " + prefix + "Threads user"
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
	} else if f == cfgThreadsUser {
		user := m.config.ThreadsUser
		if user == "" {
			user = dimStyle.Render("(not set)")
		} else {
			user = valStyle.Render(user)
		}
		body := left + "    [" + user + "]"
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
	if m.configOpen {
		return "F1/Ctrl+F feed  │  F2/Ctrl+T threads  │  Ctrl+C to quit"
	}
	if m.page == pageThreads {
		if m.config.ThreadsUser == "" {
			return "No user configured — set in settings (F10/?)  │  F1/Ctrl+F feed  │  Ctrl+C to quit"
		}
		if m.threads.loading {
			return fmt.Sprintf("Loading threads for %s…  │  F1/Ctrl+F feed  │  F10/? settings  │  Ctrl+C to quit", m.config.ThreadsUser)
		}
		if !m.threads.loaded {
			return fmt.Sprintf("Set a user in settings, then press F2/Ctrl+T  │  F1/Ctrl+F feed  │  F10/? settings  │  Ctrl+C to quit")
		}
		return fmt.Sprintf("Threads for %s  │  ↑↓ select  │  ← fold  │  → expand  │  Space toggle  │  R refresh  │  F1/Ctrl+F feed  │  F10/? settings  │  Ctrl+C to quit", m.config.ThreadsUser)
	}

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
	return fmt.Sprintf("Next refresh in %s  │  Items seen: %d  │  F2/Ctrl+T threads  │  F10/? settings  │  Ctrl+C to quit", tStr, m.st.totalItems)
}
