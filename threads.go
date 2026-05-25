package main

import (
	"fmt"
	"html"
	"regexp"
	"strings"
	"sync"

	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ── Data structures ──────────────────────────────────────────────────────────

type threadNode struct {
	item      *Item
	children  []*threadNode
	depth     int
	isUser    bool
	isParent  bool
	collapsed bool
	hasKids   bool
	nodeIdx   int
}

type threadLineInfo struct {
	nodeIdx int
	text    string
}

type threadsState struct {
	forest    []*threadNode
	flatLines []threadLineInfo
	cursor    int // nodeIdx of focused node, -1 = none
	scroll    int
	loading   bool
	loaded    bool
	err       string
	lastWidth int
}

// ── Messages ─────────────────────────────────────────────────────────────────

type threadsResultMsg struct {
	forest []*threadNode
	err    error
}

// ── Fetch command ────────────────────────────────────────────────────────────

func fetchThreadsCmd(username string) tea.Cmd {
	return func() tea.Msg {
		return fetchThreads(username)
	}
}

func fetchThreads(username string) threadsResultMsg {
	throttle := 10

	user, err := fetchUser(username)
	if err != nil {
		return threadsResultMsg{err: err}
	}

	var ids []int
	for _, id := range user.Submitted {
		if id > 0 {
			ids = append(ids, id)
			if len(ids) >= 30 {
				break
			}
		}
	}
	if len(ids) == 0 {
		return threadsResultMsg{forest: nil}
	}

	items := fetchItemsParallelAny(ids, throttle)

	var userComments []*Item
	for _, item := range items {
		if item != nil && item.Type == "comment" && !item.Deleted && !item.Dead {
			userComments = append(userComments, item)
		}
	}
	if len(userComments) == 0 {
		return threadsResultMsg{forest: nil}
	}

	// Collect parent IDs and kid IDs
	parentIDs := make(map[int]bool)
	kidIDSet := make(map[int]bool)
	var allKidIDs []int

	for _, c := range userComments {
		if c.Parent > 0 && !parentIDs[c.Parent] {
			parentIDs[c.Parent] = true
		}
		for _, kidID := range c.Kids {
			if !kidIDSet[kidID] {
				kidIDSet[kidID] = true
				allKidIDs = append(allKidIDs, kidID)
			}
		}
	}

	// Fetch parents and kids concurrently (both depend only on userComments)
	var parentIDList []int
	for id := range parentIDs {
		parentIDList = append(parentIDList, id)
	}
	var (
		parents []*Item
		kids    []*Item
	)
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		parents = fetchItemsParallelAny(parentIDList, throttle)
	}()
	go func() {
		defer wg.Done()
		kids = fetchItemsParallelAny(allKidIDs, throttle)
	}()
	wg.Wait()

	parentMap := make(map[int]*Item, len(parents))
	for _, p := range parents {
		if p != nil {
			parentMap[p.ID] = p
		}
	}

	// Fetch grandkids (kids of replies)
	var gkidIDs []int
	gkidSet := make(map[int]bool)
	for _, kid := range kids {
		if kid != nil && !kid.Deleted && !kid.Dead {
			for _, gk := range kid.Kids {
				if !gkidSet[gk] {
					gkidSet[gk] = true
					gkidIDs = append(gkidIDs, gk)
				}
			}
		}
	}
	grandkids := fetchItemsParallelAny(gkidIDs, throttle)

	// Build kids map (parentID → children), filtering deleted
	kidsMap := make(map[int][]*Item)
	addKids := func(list []*Item) {
		for _, kid := range list {
			if kid != nil && !kid.Deleted && !kid.Dead {
				kidsMap[kid.Parent] = append(kidsMap[kid.Parent], kid)
			}
		}
	}
	addKids(kids)
	addKids(grandkids)

	forest := buildThreadForest(userComments, parentMap, kidsMap)
	return threadsResultMsg{forest: forest}
}

// ── Tree building ────────────────────────────────────────────────────────────

func buildThreadForest(userComments []*Item, parentMap map[int]*Item, kidsMap map[int][]*Item) []*threadNode {
	var forest []*threadNode
	nextIdx := 0

	for _, comment := range userComments {
		parentItem := parentMap[comment.Parent]

		userNode := &threadNode{
			item:    comment,
			depth:   1,
			isUser:  true,
			nodeIdx: nextIdx,
		}
		nextIdx++

		replies := kidsMap[comment.ID]
		for _, reply := range replies {
			replyNode := buildReplyNode(reply, kidsMap, 2, &nextIdx)
			userNode.children = append(userNode.children, replyNode)
		}
		userNode.hasKids = len(userNode.children) > 0

		if parentItem != nil {
			parentNode := &threadNode{
				item:     parentItem,
				depth:    0,
				isParent: true,
				nodeIdx:  nextIdx,
			}
			nextIdx++
			parentNode.children = append(parentNode.children, userNode)
			parentNode.hasKids = true
			forest = append(forest, parentNode)
		} else {
			userNode.depth = 0
			forest = append(forest, userNode)
		}
	}
	return forest
}

func buildReplyNode(item *Item, kidsMap map[int][]*Item, depth int, nextIdx *int) *threadNode {
	node := &threadNode{
		item:      item,
		depth:     depth,
		nodeIdx:   *nextIdx,
		collapsed: depth >= 3,
	}
	*nextIdx++

	for _, gk := range kidsMap[item.ID] {
		child := buildReplyNode(gk, kidsMap, depth+1, nextIdx)
		node.children = append(node.children, child)
	}
	node.hasKids = len(node.children) > 0
	return node
}

// ── Flattening: tree → flat ANSI lines ──────────────────────────────────────

func flattenForest(forest []*threadNode, width int) []threadLineInfo {
	var lines []threadLineInfo
	for i, root := range forest {
		isLast := i == len(forest)-1
		flattenNode(root, nil, isLast, width, &lines)
		if !isLast {
			lines = append(lines, threadLineInfo{nodeIdx: -1, text: ""})
		}
	}
	return lines
}

func flattenNode(node *threadNode, ancestors []bool, isLast bool, width int, lines *[]threadLineInfo) {
	hasMoreSiblings := !isLast

	// ── Prefix (ancestor bars) ──
	var prefix strings.Builder
	for _, hasMore := range ancestors {
		if hasMore {
			prefix.WriteString("│  ")
		} else {
			prefix.WriteString("   ")
		}
	}

	// ── Connector ──
	if hasMoreSiblings {
		if node.hasKids && node.collapsed {
			prefix.WriteString("├─▶")
		} else if node.hasKids {
			prefix.WriteString("├─▼")
		} else {
			prefix.WriteString("├──")
		}
	} else {
		if node.hasKids && node.collapsed {
			prefix.WriteString("└─▶")
		} else if node.hasKids {
			prefix.WriteString("└─▼")
		} else {
			prefix.WriteString("└──")
		}
	}

	prefixStr := prefix.String()
	prefixW := lipgloss.Width(prefixStr)

	// ── Continuation prefix (for wrapped lines) ──
	var contPrefixStr string
	{
		var b strings.Builder
		for _, hasMore := range ancestors {
			if hasMore {
				b.WriteString("│  ")
			} else {
				b.WriteString("   ")
			}
		}
		if hasMoreSiblings {
			b.WriteString("│  ")
		} else {
			b.WriteString("   ")
		}
		contPrefixStr = b.String()
	}

	text := stripHTML(node.item.Text)
	availW := width - prefixW - 1
	if availW < 10 {
		availW = 10
	}

	if node.isParent {
		// ── Parent context ──
		var contextLine string
		if node.item.Title != "" {
			contextLine = fmt.Sprintf("%s  (by %s)", node.item.Title, node.item.By)
		} else {
			firstLine := strings.SplitN(text, "\n", 2)[0]
			if len(firstLine) > 80 {
				firstLine = firstLine[:77] + "..."
			} else if firstLine == "" {
				firstLine = "(comment)"
			}
			contextLine = fmt.Sprintf("@%s: %s", node.item.By, firstLine)
		}
		contextLine = truncPad(contextLine, availW)
		rendered := threadConnStyle.Render(prefixStr) + " " + threadParentStyle.Render(contextLine)
		*lines = append(*lines, threadLineInfo{nodeIdx: node.nodeIdx, text: rendered})
		// fall through to child recursion below, don't return
	} else {
		// ── Comment node ──
		authorStr := node.item.By + ":"
		bodyW := availW - lipgloss.Width(authorStr) - 1
		if bodyW < 5 {
			bodyW = 5
		}

		var bodyLines []string
		if text != "" {
			bodyLines = wordWrap(text, bodyW)
		}
		if len(bodyLines) == 0 {
			bodyLines = []string{""}
		}

		// Expand indicator
		var indicator string
		if node.hasKids {
			kidsSuffix := fmt.Sprintf("(%d)", len(node.children))
			if node.collapsed {
				indicator = " [+" + kidsSuffix + "]"
			} else {
				indicator = " [-" + kidsSuffix + "]"
			}
		}

		// ── First line ──
		{
			var b strings.Builder
			b.WriteString(threadConnStyle.Render(prefixStr))
			b.WriteString(" ")
			if node.isUser {
				b.WriteString(threadUserStyle.Render(authorStr))
			} else {
				b.WriteString(authorStr)
			}
			b.WriteString(" ")
			b.WriteString(bodyLines[0])
			if indicator != "" {
				b.WriteString(" ")
				b.WriteString(threadIndicatorStyle.Render(indicator))
			}
			rendered := fit(b.String(), width)
			*lines = append(*lines, threadLineInfo{nodeIdx: node.nodeIdx, text: rendered})
		}

		// ── Subsequent lines ──
		for _, bodyLine := range bodyLines[1:] {
			var b strings.Builder
			b.WriteString(threadConnStyle.Render(contPrefixStr))
			b.WriteString(" ")
			b.WriteString(bodyLine)
			rendered := fit(b.String(), width)
			*lines = append(*lines, threadLineInfo{nodeIdx: node.nodeIdx, text: rendered})
		}
	}

	// ── Recurse into children (if expanded) ──
	if !node.collapsed {
		for i, child := range node.children {
			childIsLast := i == len(node.children)-1
			newAncestors := make([]bool, len(ancestors)+1)
			copy(newAncestors, ancestors)
			newAncestors[len(ancestors)] = hasMoreSiblings
			flattenNode(child, newAncestors, childIsLast, width, lines)
		}
	}
}

// ── Initialise / clear state ─────────────────────────────────────────────────

func (ts *threadsState) reset() {
	ts.forest = nil
	ts.flatLines = nil
	ts.cursor = 0
	ts.scroll = 0
	ts.loading = false
	ts.loaded = false
	ts.err = ""
	ts.lastWidth = 0
}

func (ts *threadsState) applyResult(msg threadsResultMsg) {
	ts.loading = false
	if msg.err != nil {
		ts.err = msg.err.Error()
		ts.loaded = false
		return
	}
	ts.err = ""
	ts.forest = msg.forest
	ts.loaded = true
	ts.cursor = 0
	ts.scroll = 0
	ts.lastWidth = 0
}

// ── HTML stripping ───────────────────────────────────────────────────────────

var htmlTagRe = regexp.MustCompile(`<[^>]*>`)
var multiSpaceRe = regexp.MustCompile(`[ \t]+`)

func stripHTML(s string) string {
	s = strings.ReplaceAll(s, "<p>", "\n")
	s = strings.ReplaceAll(s, "</p>", "")
	s = htmlTagRe.ReplaceAllString(s, "")
	s = html.UnescapeString(s)
	s = multiSpaceRe.ReplaceAllString(s, " ")
	for strings.Contains(s, "\n ") {
		s = strings.ReplaceAll(s, "\n ", "\n")
	}
	for strings.Contains(s, " \n") {
		s = strings.ReplaceAll(s, " \n", "\n")
	}
	for strings.Contains(s, "\n\n\n") {
		s = strings.ReplaceAll(s, "\n\n\n", "\n\n")
	}
	return strings.TrimSpace(s)
}

// ── Word wrapping ────────────────────────────────────────────────────────────

func wordWrap(s string, maxWidth int) []string {
	if maxWidth <= 0 {
		return []string{s}
	}
	var lines []string
	for _, paragraph := range strings.Split(s, "\n") {
		if paragraph == "" {
			lines = append(lines, "")
			continue
		}
		words := strings.Fields(paragraph)
		if len(words) == 0 {
			lines = append(lines, "")
			continue
		}
		current := words[0]
		for _, word := range words[1:] {
			candidate := current + " " + word
			if len(candidate) <= maxWidth {
				current = candidate
			} else {
				lines = append(lines, current)
				current = word
				for len(current) > maxWidth {
					lines = append(lines, current[:maxWidth-1]+"…")
					current = current[maxWidth-1:]
				}
			}
		}
		lines = append(lines, current)
	}
	return lines
}

// ── Cursor / navigation helpers ──────────────────────────────────────────────

// findNodeLine returns the flatLines index of the first line for the given nodeIdx.
func findNodeLine(flatLines []threadLineInfo, nodeIdx int) int {
	for i, li := range flatLines {
		if li.nodeIdx == nodeIdx {
			return i
		}
	}
	return -1
}

// prevVisibleNode finds the nodeIdx of the node just before the given flatLineIdx.
func prevVisibleNode(flatLines []threadLineInfo, flatIdx int) int {
	if flatIdx <= 0 {
		return -1
	}
	target := flatLines[flatIdx].nodeIdx
	for i := flatIdx - 1; i >= 0; i-- {
		if flatLines[i].nodeIdx != target && flatLines[i].nodeIdx >= 0 {
			return flatLines[i].nodeIdx
		}
	}
	return -1
}

// nextVisibleNode finds the nodeIdx of the node just after the given flatLineIdx.
func nextVisibleNode(flatLines []threadLineInfo, flatIdx int) int {
	if flatIdx < 0 || flatIdx >= len(flatLines)-1 {
		return -1
	}
	target := flatLines[flatIdx].nodeIdx
	for i := flatIdx + 1; i < len(flatLines); i++ {
		if flatLines[i].nodeIdx != target && flatLines[i].nodeIdx >= 0 {
			return flatLines[i].nodeIdx
		}
	}
	return -1
}

// findNode finds a threadNode by nodeIdx in the forest.
func findNode(nodes []*threadNode, nodeIdx int) *threadNode {
	for _, n := range nodes {
		if n.nodeIdx == nodeIdx {
			return n
		}
		if found := findNode(n.children, nodeIdx); found != nil {
			return found
		}
	}
	return nil
}

// toggleCollapse toggles the collapsed state of the node with the given nodeIdx.
// Returns the new flatLines and the new flatLines index for the same node.
func (ts *threadsState) toggleCollapse(nodeIdx int, width int) {
	n := findNode(ts.forest, nodeIdx)
	if n == nil || !n.hasKids {
		return
	}
	n.collapsed = !n.collapsed
	ts.flatLines = flattenForest(ts.forest, width)
}

// setCollapse sets the collapsed state of the node with the given nodeIdx.
func (ts *threadsState) setCollapse(nodeIdx int, collapsed bool, width int) {
	n := findNode(ts.forest, nodeIdx)
	if n == nil || !n.hasKids || n.collapsed == collapsed {
		return
	}
	n.collapsed = collapsed
	ts.flatLines = flattenForest(ts.forest, width)
}

// ── Keys ─────────────────────────────────────────────────────────────────────

var (
	threadConnStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	threadUserStyle      = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6"))
	threadParentStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	threadIndicatorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
)
