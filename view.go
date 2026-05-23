package main

import (
	"fmt"
	"strings"
	"time"
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

	ph := h - 3 // rows available for the feed panel
	if ph < 1 {
		ph = 1
	}

	var buf strings.Builder

	scrollHint := gry + " (live)" + rst
	if m.st.scroll > 0 {
		scrollHint = gry + " [↑ scrolled]" + rst
	}
	if m.configOpen {
		scrollHint = gry + " (settings)" + rst
	}
	buf.WriteString(fit(" "+bold+cyn+"HN Feed"+rst+scrollHint, w))
	buf.WriteByte('\n')

	buf.WriteString(gry + strings.Repeat("─", w) + rst)
	buf.WriteByte('\n')

	if m.configOpen {
		cfgLines := m.buildConfigLines(w)
		cfgTop := 0
		for row := 0; row < ph; row++ {
			var line string
			if row >= cfgTop && row < cfgTop+len(cfgLines) {
				line = cfgLines[row-cfgTop]
			}
			buf.WriteString(rst + fit(line, w))
			buf.WriteByte('\n')
		}
	} else {
		maxScroll := len(m.st.buf) - ph
		if maxScroll < 0 {
			maxScroll = 0
		}
		sc := m.st.scroll
		if sc > maxScroll {
			sc = maxScroll
		}
		startIdx := len(m.st.buf) - ph - sc
		if startIdx < 0 {
			startIdx = 0
		}
		for row := 0; row < ph; row++ {
			li := startIdx + row
			line := ""
			if li < len(m.st.buf) {
				line = m.st.buf[li]
			}
			buf.WriteString(rst + fit(line, w))
			buf.WriteByte('\n')
		}
	}

	status := m.statusText()
	buf.WriteString("\x1b[44;97m" + fit("  "+status+"  ", w))

	return buf.String()
}

func (m model) buildConfigLines(w int) []string {
	fields := m.configFields()

	var raw []string
	raw = append(raw, bold+"  Configuration"+rst, "")

	for i, f := range fields {
		cursor := "  "
		if i == m.configCur {
			cursor = "▸ "
		}
		switch f {
		case cfgFPToggle:
			chk := "[ ]"
			if m.config.ShowFrontPage {
				chk = "[x]"
			}
			raw = append(raw, fmt.Sprintf("  %s%s  Front page events", cursor, chk))
		case cfgFPEntered:
			chk := "[ ]"
			if m.config.FrontEntered {
				chk = "[x]"
			}
			raw = append(raw, fmt.Sprintf("    %s%s  Entered front page", cursor, chk))
		case cfgFPRankUp:
			chk := "[ ]"
			if m.config.FrontRankUp {
				chk = "[x]"
			}
			raw = append(raw, fmt.Sprintf("    %s%s  Ranking up", cursor, chk))
		case cfgFPRankDown:
			chk := "[ ]"
			if m.config.FrontRankDown {
				chk = "[x]"
			}
			raw = append(raw, fmt.Sprintf("    %s%s  Ranking down", cursor, chk))
		case cfgFPLeft:
			chk := "[ ]"
			if m.config.FrontLeft {
				chk = "[x]"
			}
			raw = append(raw, fmt.Sprintf("    %s%s  Left front page", cursor, chk))
		case cfgNSToggle:
			chk := "[ ]"
			if m.config.ShowNewStories {
				chk = "[x]"
			}
			raw = append(raw, fmt.Sprintf("  %s%s  New story events", cursor, chk))
		case cfgPollSlider:
			raw = append(raw, fmt.Sprintf("  %sPoll interval: %ds", cursor, m.config.PollSeconds))
		}
	}

	raw = append(raw, "")
	raw = append(raw, gry+"  ↑↓ Navigate          Space toggle"+rst)
	raw = append(raw, gry+"  ← → adjust value     ?/F1/Esc close"+rst)

	lines := make([]string, len(raw))
	for i, line := range raw {
		vl := measureVisible(line)
		if vl < w {
			lines[i] = line + strings.Repeat(" ", w-vl) + rst
		} else {
			lines[i] = line + rst
		}
	}

	return lines
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
