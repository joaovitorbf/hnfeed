# AGENTS.md

## Project overview

A Go TUI that displays a live, unified Hacker News feed in the terminal. Merges
HN "new" and front page into one chronological stream using
[github.com/charmbracelet/bubbletea](https://github.com/charmbracelet/bubbletea).

## Files

| File | Responsibility |
|---|---|
| `hn.go` | `Item` struct and HN API fetchers |
| `style.go` | Lipgloss styles, ANSI-safe text helpers (`fit`, `truncPad`), settings-panel styles (`configBorder`, `cursorStyle`, etc.), feed styles (`feedBorder`, `statusBarStyle`), and manual ANSI checkbox constants (`greenCheck`, `grayCheck`, `resetFgBold`) |
| `config.go` | Configuration struct and JSON persistence |
| `feed.go` | Entry type constants (`entryType`), `feedEntry` struct, `feedState` struct, `appendEntry`, `totalLines` |
| `format.go` | Entry formatters (`formatNewItemLines`, `formatFrontEventLines`, `formatFrontLeaveLine`) |
| `main.go` | Entry point, program setup |
| `model.go` | Bubbletea model, messages, commands, update loop |
| `view.go` | Rendering: header, feed panel (bordered), settings overlay (`buildConfigLines`, `configFieldLine`, `sectionDivider`, `buildHelpLine`, `checkboxStr`), `formatEntry` dispatch, status bar |

## API

`https://hacker-news.firebaseio.com/v0` — `/newstories.json` (newest IDs),
`/topstories.json` (front-page rank order), `/item/<id>.json` (details).
Fetches parallelised via goroutines + semaphore channel.

## Poll cycle

`tea.Tick` (100ms) drives the loop. Every `pollSeconds`, a command fetches new
IDs above `maxID` watermark → new-story entries; fetches top 30 and diffs
against `frontRanks` → front-page events (new entries, rank improvements,
rank drops, and items that left the top 30).

## Entry types (4 lines each)

| Entry | Prefix | Colour |
|---|---|---|
| New story | `[HH:mm:ss]` | Yellow |
| Front-page entry | `★ #N` | Orange |
| Front-page rank-up | `↑ #N (was #M)` | Orange |
| Front-page rank-up (peak) | `↑ #N (best #M)` | Orange |
| Front-page rank-down | `↓ #N (was #M)` | Orange |
| Front-page rank-down (worst) | `↓ #N (worst #M)` | Orange |
| Front-page leave | `✕ #N` | Gray |

## Feed state (`feedState` struct)

| Field | Purpose |
|---|---|
| `entries` | Raw `[]feedEntry` structs (capped at 500 entries / ~2000 lines); formatted at render time |
| `frontRanks` | Last known rank per front-page item |
| `frontBestRanks` | Best (lowest-number) rank ever seen per item |
| `frontWorstRanks` | Worst (highest-number) rank ever seen per item |
| `frontCache` | Last known `*Item` data for front-page items (used for leave events) |
| `seenIDs` | IDs already emitted as new-story entries |
| `maxID` | Watermark for incremental new-story polling |
| `scroll` | Lines scrolled up from bottom (0 = live) |
| `totalItems` | Total entries ever appended |

Helper methods:
- `appendEntry(feedEntry)` — appends entry, increments `totalItems`, advances `scroll` by 4.
- `totalLines()` — returns `len(entries) * 4` for scroll/viewport calculations.

See also `feedEntry` struct and `entryType` constants in `feed.go`. New entries are created with a `feedEntry{typ: ..., item: ..., prefix: ...}` literal and passed to `appendEntry`. Formatting is deferred to the view (see below).

## Settings

Press `?` or `F1` to open the settings page (replaces the feed). Navigate with
`↑`/`↓`, toggle filters with `Space`/`Enter`, adjust numeric values with `←`/`→`, close with `Esc`.

### Layout

The settings page is rendered inside a rounded-border panel:

```
╭──────────────────────────────────────────────╮
│  ─────────────── Events ──────────────────── │
│                                                │
│    ✓ Front page events                        │
│      ✓ Entered front page                     │
│    ▸ ✓ Ranking up                   ← cursor  │
│        ✓ Compare to best rank                 │
│      ✗ Ranking down                           │
│      ✗ Left front page                        │
│    ✓ New story events                         │
│                                                │
│  ──────────── Feed Settings ───────────────── │
│                                                │
│    Poll interval     30s  ◀  ▶                │
│    Initial items       5  ◀  ▶                │
│                                                │
│  ↑↓ navigate  │  Space toggle  │  ←→ adjust  │  Esc close │
╰──────────────────────────────────────────────╯
```

- **Bordered panel**: Rounded corners with cyan border.
- **Section dividers**: Labels centered and flanked by dashes.
- **Cursor row**: Full-row dark gray background (`cursorStyle`).
- **Checkboxes**: Green `[✓]` when enabled, gray `[✗]` when disabled. Uses
  manual ANSI sequences (`greenCheck`, `grayCheck`, `resetFgBold`) that only
  toggle foreground and bold, preserving any outer cursor background.
- **Numeric values**: Yellow bold, right-aligned in a 4-wide field so the
  adjustment arrows (`◀  ▶`) align vertically across rows.
- **Help bar**: Key bindings in bold cyan separated by `│`.

| Field | Default | Purpose |
|---|---|---|
| `ShowFrontPage` | `true` | Master toggle for all front-page events |
| `FrontEntered` | `true` | Show `★ #N` when an item enters the front page |
| `FrontRankUp` | `true` | Show `↑ #N (was #M)` on rank improvement |
| `FrontRankUpPeak` | `true` | Only show rank-up on new best rank (`↑ #N (best #M)`); regular changes hidden |
| `FrontRankDown` | `false` | Show `↓ #N (was #M)` on rank drop |
| `FrontRankDownWorst` | `true` | Only show rank-down on new worst rank (`↓ #N (worst #M)`); regular changes hidden |
| `FrontLeft` | `false` | Show `✕ #N` when an item leaves the top 30 |
| `ShowNewStories` | `true` | Show new-story entries |
| `PollSeconds` | `30` | Seconds between refreshes |
| `InitialItems` | `5` | Stories loaded from each source on startup |

Sub-options (`FrontEntered`, `FrontRankUp`, `FrontRankUpPeak`, `FrontRankDown`, `FrontRankDownWorst`, `FrontLeft`) are
indented under **Front page events** in the settings UI and are only shown
when `ShowFrontPage` is enabled. `FrontRankUpPeak` is only shown when
`FrontRankUp` is enabled; `FrontRankDownWorst` is only shown when
`FrontRankDown` is enabled.

The header shows `(settings)` while the settings page is open. Filtering only
affects newly arriving entries — existing ones in the buffer remain visible.

## Visual design

Both the main feed and the settings panel use rounded-border panels with a
cyan (`lipgloss.Color("6")`) border, creating a consistent Charmbracelet-style
look.

- **Feed panel**: Wrapped in a `feedBorder` (rounded, cyan) that replaces the
  old plain divider line. Content width inside the border is `innerW = w - 4`.
- **Status bar**: Uses `statusBarStyle` (cyan background, white bold text)
  instead of the old plain-blue bar.
- **Header**: Bold cyan ` HN Feed ` with a gray scroll hint on the right.

### Entry formatting — render-time

Entries are stored as `feedEntry` structs (raw data), not pre-rendered ANSI
lines. At each frame, `View()` calls `m.formatEntry(entry, innerW)` which
dispatches to the appropriate formatter (`formatNewItemLines`,
`formatFrontEventLines`, `formatFrontLeaveLine`) with the **current** inner
panel width. This ensures:

- Tags (`[NEW]`, `[42▲ 7c]`) are always right-aligned to the container edge.
- Resizing the window instantly reflows all visible entries — no stale widths.
- The `fit(line, innerW)` call in the rendering loop is a safety no-op
  (lines are already the correct width).

Settings are persisted to `hnfeed-settings.json` in the current directory.
Saved automatically on every toggle and loaded on startup. If the file is
missing or corrupt, defaults are used.

## Startup

`Init()` launches async `seedFeedCmd`. On `seedResultMsg`: populate
`frontRanks` silently, emit `InitialItems` front-page entries + newest stories,
set `ready = true` to begin live polls.

## Running

```
go build -o hnfeed .
```

Requires Go 1.26+. `Ctrl+C` to exit, `?`/`F1` for settings.

## Guidelines

- Split logic across `hn.go` (API), `model.go` (tea model/update), `view.go` (rendering), `format.go` (entry formatting), `config.go` (settings), `style.go` (lipgloss styles/helpers), `feed.go` (state/entry structs), `main.go` (entry point). No build tags.
- Use `fit`/`truncPad` from `style.go` for ANSI-safe width accounting.
- Every entry produces exactly 4 lines. The `totalLines()` helper computes `len(entries) * 4`.
- Use `m.st.appendEntry(feedEntry{...})` to add entries. Do **not** call formatters at event time — create a `feedEntry` and let the view format it at render time.
- Trimming (capped at 500 entries / ~2000 lines) operates on `entries`, not lines: `trim * 4` when adjusting scroll.
- Populate `frontRanks` for all 30 items at startup before first live poll.
- Wrap network calls — never crash on transient API failure.
- Commands must never mutate model directly; send messages to `Update`.
- Avoid verbose comments. Keep inline comments minimal — the code should be self-documenting.
- All commits must follow [Conventional Commits](https://www.conventionalcommits.org/) and use title only (no body) unless absolutely necessary.
