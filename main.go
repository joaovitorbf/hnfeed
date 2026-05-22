package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"
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

// ── Entry formatters ──────────────────────────────────────────────────────────

func itemURL(item *Item) string {
	if item.URL != "" {
		return item.URL
	}
	return fmt.Sprintf("https://news.ycombinator.com/item?id=%d", item.ID)
}

func commentURL(id int) string {
	return fmt.Sprintf("https://news.ycombinator.com/item?id=%d", id)
}

// formatNewItemLines returns the 4 feed lines for a new story.
//
//	[HH:mm:ss] Title
//	  article URL
//	  Comments: HN URL
//	(blank)
func formatNewItemLines(item *Item, width int) []string {
	t := time.Unix(item.Time, 0).Local().Format("15:04:05")
	meta := gry + "[NEW]" + rst
	const timeVisible = 11 // "[HH:mm:ss] "
	avail := width - timeVisible - len([]rune("[NEW]")) - 1
	if avail < 1 {
		avail = 1
	}
	title := padRight(truncRunes(item.Title, avail), avail)
	return []string{
		gry + "[" + t + "]" + rst + " " + yel + title + rst + " " + meta,
		"  " + gry + itemURL(item) + rst,
		"  " + gry + "Comments: " + commentURL(item.ID) + rst,
		"",
	}
}

// formatFrontEventLines returns the 4 feed lines for a front-page event.
// prefix is e.g. "★ #3  " for a new entry or "↑ #3 (was #7)  " for a rank-up.
//
//	[HH:mm:ss] <prefix> Title …   [score▲ Nc]
//	  article URL
//	  Comments: HN URL
//	(blank)
func formatFrontEventLines(item *Item, prefix string, width int) []string {
	t := time.Now().Format("15:04:05")
	metaPlain := fmt.Sprintf("[%d▲ %dc]", item.Score, item.Descendants)
	const timeVisible = 11
	avail := width - timeVisible - len([]rune(prefix)) - len([]rune(metaPlain)) - 1
	if avail < 1 {
		avail = 1
	}
	title := padRight(truncRunes(item.Title, avail), avail)
	return []string{
		gry + "[" + t + "]" + rst + " " + org + prefix + title + rst + " " + gry + metaPlain + rst,
		"  " + gry + itemURL(item) + rst,
		"  " + gry + "Comments: " + commentURL(item.ID) + rst,
		"",
	}
}

// appendEntry appends a 4-line entry to feedBuf, increments totalItems, and
// advances scroll by 4 when the user is scrolled up, keeping the viewport stable.
func appendEntry(feedBuf *[]string, lines []string, scroll *int, totalItems *int) {
	*feedBuf = append(*feedBuf, lines...)
	*totalItems++
	if *scroll > 0 {
		*scroll += 4
	}
}

// ── Feed state ────────────────────────────────────────────────────────────────

type feedState struct {
	buf        []string
	frontRanks map[int]int  // id → last known rank (1-based)
	seenIDs    map[int]bool // ids already emitted as new-story entries
	maxID      int          // highest new-story ID seen; watermark for incremental polling
	scroll     int          // lines scrolled up from the bottom (0 = live)
	totalItems int          // total entries ever appended
}

// ── Messages ──────────────────────────────────────────────────────────────────

// tickMsg fires every 100 ms to drive the poll timer.
type tickMsg struct{}

// seedResultMsg carries the result of the initial data load.
type seedResultMsg struct {
	frontItems []*Item
	frontRanks map[int]int
	newItems   []*Item
}

// pollResultMsg carries the result of a periodic refresh.
type pollResultMsg struct {
	newItems      []*Item
	frontItems    []*Item
	newFrontRanks map[int]int
	newMaxID      int
}

// ── Model ─────────────────────────────────────────────────────────────────────

type model struct {
	st       feedState
	width    int
	height   int
	pollSec  int
	throttle int
	initial  int
	lastPoll time.Time
	ready    bool
}

func (m model) Init() tea.Cmd {
	return tea.Batch(
		m.seedFeedCmd(),
		tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
			return tickMsg{}
		}),
	)
}

// ── Commands ──────────────────────────────────────────────────────────────────

// seedFeedCmd fetches the initial front page and newest stories.
func (m model) seedFeedCmd() tea.Cmd {
	return func() tea.Msg {
		throttle := m.throttle
		initial := m.initial

		var frontItems []*Item
		var frontRanks map[int]int
		var newItems []*Item

		var wg sync.WaitGroup
		wg.Add(2)

		go func() {
			defer wg.Done()
			if items, ranks, err := fetchFrontPage(throttle); err == nil {
				frontItems = items
				frontRanks = ranks
			}
		}()

		go func() {
			defer wg.Done()
			if ids, err := fetchNewStoryIDs(); err == nil && len(ids) > 0 {
				count := initial
				if count > len(ids) {
					count = len(ids)
				}
				newItems = fetchItemsParallel(ids[:count], throttle)
			}
		}()

		wg.Wait()

		return seedResultMsg{
			frontItems: frontItems,
			frontRanks: frontRanks,
			newItems:   newItems,
		}
	}
}

// pollNowCmd fetches new stories and the current front page for a periodic refresh.
func (m model) pollNowCmd() tea.Cmd {
	return func() tea.Msg {
		throttle := m.throttle

		var newItems []*Item
		newMaxID := m.st.maxID
		if ids, err := fetchNewStoryIDs(); err == nil {
			if len(ids) > 0 && ids[0] > newMaxID {
				newMaxID = ids[0]
			}
			var pendingIDs []int
			for _, id := range ids {
				if id > m.st.maxID {
					pendingIDs = append(pendingIDs, id)
				}
			}
			newItems = fetchItemsParallel(pendingIDs, throttle)
		}

		var frontItems []*Item
		var newFrontRanks map[int]int
		if items, ranks, err := fetchFrontPage(throttle); err == nil {
			frontItems = items
			newFrontRanks = ranks
		}

		return pollResultMsg{
			newItems:      newItems,
			frontItems:    frontItems,
			newFrontRanks: newFrontRanks,
			newMaxID:      newMaxID,
		}
	}
}

// ── Update ────────────────────────────────────────────────────────────────────

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		return m, nil

	case tea.MouseMsg:
		if msg.Action == tea.MouseActionPress {
			switch msg.Button {
			case tea.MouseButtonWheelUp:
				m.st.scroll += 4
			case tea.MouseButtonWheelDown:
				m.st.scroll -= 4
				if m.st.scroll < 0 {
					m.st.scroll = 0
				}
			}
		}
		return m, nil

	case seedResultMsg:
		w := m.width
		if w < 10 {
			w = 80
		}

		for id, rank := range msg.frontRanks {
			m.st.frontRanks[id] = rank
		}
		for i, item := range msg.frontItems {
			if i >= m.initial {
				break
			}
			appendEntry(&m.st.buf, formatFrontEventLines(item, fmt.Sprintf("★ #%d  ", msg.frontRanks[item.ID]), w), &m.st.scroll, &m.st.totalItems)
		}
		for _, item := range msg.newItems {
			if item == nil {
				continue
			}
			m.st.seenIDs[item.ID] = true
			appendEntry(&m.st.buf, formatNewItemLines(item, w), &m.st.scroll, &m.st.totalItems)
			if item.ID > m.st.maxID {
				m.st.maxID = item.ID
			}
		}

		m.lastPoll = time.Now()
		m.ready = true
		return m, nil

	case tickMsg:
		if !m.ready {
			return m, tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
				return tickMsg{}
			})
		}

		cmds := []tea.Cmd{
			tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
				return tickMsg{}
			}),
		}
		if time.Since(m.lastPoll) >= time.Duration(m.pollSec)*time.Second {
			m.lastPoll = time.Now()
			cmds = append(cmds, m.pollNowCmd())
		}
		return m, tea.Batch(cmds...)

	case pollResultMsg:
		w := m.width
		if w < 10 {
			w = 80
		}

		// New stories
		oldMaxID := m.st.maxID
		if msg.newMaxID > m.st.maxID {
			m.st.maxID = msg.newMaxID
		}
		for _, item := range msg.newItems {
			if item == nil {
				continue
			}
			if !m.st.seenIDs[item.ID] {
				m.st.seenIDs[item.ID] = true
				appendEntry(&m.st.buf, formatNewItemLines(item, w), &m.st.scroll, &m.st.totalItems)
			}
		}

		for id := range m.st.seenIDs {
			if id <= oldMaxID {
				delete(m.st.seenIDs, id)
			}
		}

		// Front page
		if msg.newFrontRanks != nil {
			for _, item := range msg.frontItems {
				if item == nil {
					continue
				}
				id := item.ID
				newRank := msg.newFrontRanks[id]
				if oldRank, exists := m.st.frontRanks[id]; exists {
					if newRank < oldRank {
						appendEntry(&m.st.buf, formatFrontEventLines(item, fmt.Sprintf("↑ #%d (was #%d)  ", newRank, oldRank), w), &m.st.scroll, &m.st.totalItems)
					}
				} else if !m.st.seenIDs[id] {
					appendEntry(&m.st.buf, formatFrontEventLines(item, fmt.Sprintf("★ #%d  ", newRank), w), &m.st.scroll, &m.st.totalItems)
				}
			}
			m.st.frontRanks = msg.newFrontRanks
		}

		if len(m.st.buf) > 2000 {
			trim := len(m.st.buf) - 2000
			m.st.buf = m.st.buf[trim:]
			if m.st.scroll > 0 {
				m.st.scroll -= trim
				if m.st.scroll < 0 {
					m.st.scroll = 0
				}
			}
		}

		return m, nil
	}

	return m, nil
}

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
	buf.WriteString(fit(" "+bold+cyn+"HN Feed"+rst+scrollHint, w))
	buf.WriteByte('\n')

	buf.WriteString(gry + strings.Repeat("─", w) + rst)
	buf.WriteByte('\n')

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

	status := m.statusText()
	buf.WriteString("\x1b[44;97m" + fit("  "+status+"  ", w))

	return buf.String()
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
	return fmt.Sprintf("Next refresh in %s  │  Items seen: %d  │  Ctrl+C to quit", tStr, m.st.totalItems)
}

// ── Main ──────────────────────────────────────────────────────────────────────

func main() {
	pollSeconds := flag.Int("pollSeconds", 30, "seconds between refreshes")
	initialItems := flag.Int("initialItems", 5, "stories loaded from each source on startup")
	throttleLimit := flag.Int("throttleLimit", 10, "max parallel item fetches")
	flag.Parse()

	m := model{
		st: feedState{
			frontRanks: make(map[int]int),
			seenIDs:    make(map[int]bool),
		},
		pollSec:  *pollSeconds,
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
