package main

import (
	"strings"
	"unicode/utf8"
)

// ── ANSI colour constants ──────────────────────────────────────────────────────

const (
	rst  = "\x1b[0m"
	bold = "\x1b[1m"
	yel  = "\x1b[33m"
	cyn  = "\x1b[36m"
	gry  = "\x1b[90m"
	org  = "\x1b[38;5;208m"
)

// ── ANSI text helpers ──────────────────────────────────────────────────────────

// measureVisible returns the number of visible (non-ANSI-escape) runes in s.
func measureVisible(s string) int {
	count, inEsc := 0, false
	for i := 0; i < len(s); {
		b := s[i]
		if inEsc {
			if (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z') {
				inEsc = false
			}
			i++
			continue
		}
		if b == 0x1b && i+1 < len(s) && s[i+1] == '[' {
			inEsc = true
			i += 2
			continue
		}
		_, size := utf8.DecodeRuneInString(s[i:])
		count++
		i += size
	}
	return count
}

// fit pads or truncates s so its visible width equals exactly width.
// ANSI escape sequences are preserved and not counted toward visible width.
func fit(s string, width int) string {
	if width <= 0 {
		return rst
	}
	vl := measureVisible(s)
	if vl < width {
		return s + strings.Repeat(" ", width-vl) + rst
	}
	if vl == width {
		return s + rst
	}
	target := width - 1
	var sb strings.Builder
	vis := 0
	for i := 0; i < len(s); {
		b := s[i]
		if b == 0x1b && i+1 < len(s) && s[i+1] == '[' {
			j := i + 2
			for j < len(s) && !((s[j] >= 'A' && s[j] <= 'Z') || (s[j] >= 'a' && s[j] <= 'z')) {
				j++
			}
			if j < len(s) {
				j++
			}
			sb.WriteString(s[i:j])
			i = j
			continue
		}
		if vis >= target {
			sb.WriteRune('…')
			break
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		sb.WriteRune(r)
		vis++
		i += size
	}
	sb.WriteString(rst)
	return sb.String()
}

// truncRunes truncates s to at most n runes, appending '…' if truncated.
func truncRunes(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	if n <= 0 {
		return ""
	}
	return string(runes[:n-1]) + "…"
}

// padRight right-pads s with spaces to exactly n visible runes.
func padRight(s string, n int) string {
	runes := []rune(s)
	if len(runes) >= n {
		return s
	}
	return s + strings.Repeat(" ", n-len(runes))
}
