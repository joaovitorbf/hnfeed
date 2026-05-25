# AGENTS.md

## Project overview

A Go TUI that displays a live, unified Hacker News feed in the terminal. Merges
HN "new" and front page into one chronological stream using
[github.com/charmbracelet/bubbletea](https://github.com/charmbracelet/bubbletea).

## Files

| File | Responsibility |
|---|---|
| `hn.go` | `Item`/`User` structs and HN API fetchers (stories, comments, users) |
| `style.go` | Lipgloss styles, ANSI-safe text helpers (`fit`, `truncPad`), settings-panel styles (`configBorder`, `cursorStyle`, etc.), feed/threads styles, and manual ANSI checkbox constants (`greenCheck`, `grayCheck`, `resetFgBold`) |
| `config.go` | Configuration struct and JSON persistence |
| `feed.go` | Entry type constants (`entryType`), `feedEntry` struct, `feedState` struct, `appendEntry`, `totalLines` |
| `format.go` | Entry formatters (`formatNewItemLines`, `formatFrontEventLines`, `formatFrontLeaveLine`) |
| `threads.go` | Thread tree state (`threadNode`, `threadLineInfo`, `threadsState`), tree building (`buildThreadForest`, `buildReplyNode`), flattening with word-wrap and tree connectors (`flattenForest`, `flattenNode`), HTML stripping, word wrapping, cursor navigation helpers, and the threads fetch command |
| `main.go` | Entry point, program setup |
| `model.go` | Bubbletea model, page enum (`pageFeed`, `pageThreads`), messages, commands, update loop including page switching and threads navigation |
| `view.go` | Rendering: header (page-aware), feed panel, threads panel (tree with cursor highlight), settings overlay (`buildConfigLines`, `configFieldLine`, `sectionDivider`, `buildHelpLine`, `checkboxStr`), `formatEntry` dispatch, status bar (context-dependent) |

## API

`https://hacker-news.firebaseio.com/v0` — `/newstories.json` (newest IDs),
`/topstories.json` (front-page rank order), `/item/<id>.json` (details),
`/user/<username>.json` (user profile + submitted IDs).
Fetches parallelised via goroutines + semaphore channel.

### Item struct

Extended to support comments in addition to stories:

| Field | Type | Description |
|---|---|---|
| `ID` | `int` | Unique identifier |
| `Title` | `string` | Story/poll title (empty for comments) |
| `URL` | `string` | Story URL |
| `Score` | `int` | Story score or pollopt votes |
| `Descendants` | `int` | Total comment count |
| `Time` | `int64` | Unix timestamp |
| `Type` | `string` | `"story"`, `"comment"`, `"job"`, `"poll"`, `"pollopt"` |
| `By` | `string` | Author username |
| `Text` | `string` | Comment/story text (HTML) |
| `Parent` | `int` | Parent item ID (comments only) |
| `Kids` | `[]int` | Child comment IDs |
| `Deleted` | `bool` | `true` if deleted |
| `Dead` | `bool` | `true` if dead |

### Fetch helpers

- `fetchItem(id)` — fetches a story by ID (filters items without title)
- `fetchItemByID(id)` — fetches any item type without filtering (used for comments)
- `fetchItemsParallel(ids, throttle)` — parallel story fetcher (sorts by time)
- `fetchItemsParallelAny(ids, throttle)` — parallel fetcher for any item type
- `fetchUser(username)` — returns `User` struct with `Submitted` IDs

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

## Pages

The app has two pages and a settings overlay:
- **Feed** (`pageFeed`): The live unified HN feed (default on startup)
- **Threads** (`pageThreads`): Tree view of a user's recent comments with replies.
  Each thread is built from one user comment. The tree structure is:
  parent context (what the user replied to, depth 0, `isParent`) →
  user's comment (depth 1, `isUser`) → nested replies (depth 2+).
  This matches [HN's own threads view](https://news.ycombinator.com/threads).
  Every user comment gets its own thread entry in the forest, even if
  one comment replies to another user comment — the parent appears as
  the context line of the child's thread.
- **Settings**: Overlay rendered on top of the current page

Navigation:
| Key | Action |
|---|---|
| `F1` / `Ctrl+F` | Switch to Feed page (closes settings if open) |
| `F2` / `Ctrl+T` | Switch to Threads page (closes settings if open; triggers fetch if not loaded or if `ThreadsUser` changed) |
| `F10` / `?` | Toggle settings overlay |
| `R` | Refresh threads (re-fetches all user comments) |
| `←` | Fold comment on Threads page |
| `→` | Expand comment on Threads page |
| `↑` `↓` | Navigate between entries on Threads page |
| `Space` / `Enter` | Toggle collapse on Threads page |

## Settings

Press `F10` or `?` to open the settings page (overlays the current page). Navigate with
`↑`/`↓`, toggle filters with `Space`/`Enter`, adjust numeric values with `←`/`→`, type text for the Threads user field, close with `Esc`.

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
│    Threads user  [joaof____]                   │
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
| `ThreadsUser` | `""` | HN username for the Threads page |

Sub-options (`FrontEntered`, `FrontRankUp`, `FrontRankUpPeak`, `FrontRankDown`, `FrontRankDownWorst`, `FrontLeft`) are
indented under **Front page events** in the settings UI and are only shown
when `ShowFrontPage` is enabled. `FrontRankUpPeak` is only shown when
`FrontRankUp` is enabled; `FrontRankDownWorst` is only shown when
`FrontRankDown` is enabled.

The header shows `(settings)` while the settings page is open. Filtering only
affects newly arriving entries — existing ones in the buffer remain visible.

**Auto-refresh on user change:** When `ThreadsUser` is modified in settings and
the user leaves the settings overlay (via `Esc`, `F1`/`Ctrl+F`, or `F2`/`Ctrl+T`),
threads data is immediately refreshed for the new user, even if threads were
previously loaded for a different user. The model tracks the last-fetched user
in `lastThreadsUser` (initialised from config on startup) and compares against
the current setting to decide whether a re-fetch is needed.

## Visual design

All panels use rounded-border panels with a cyan (`lipgloss.Color("6")`) border,
creating a consistent Charmbracelet-style look.

- **Feed panel**: Wrapped in `feedBorder` (rounded, cyan). Content inside is `innerW = w - 4`.
- **Threads panel**: Same border as feed. Content rendered from pre-flattened `flatLines` with cursor highlight.
- **Settings panel**: Same border, with sections and text input for username.
- **Status bar**: `statusBarStyle` (cyan background, white bold text). Context-dependent — shows different hints on feed, threads, and settings pages. Shortcuts are shown as `F1/Ctrl+F` format (function key first, Ctrl shortcut second).
- **Header**: Bold cyan, shows ` HN Feed ` or ` HN Threads ` depending on current page, with a gray scroll hint.

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
set `ready = true` to begin live polls. Threads page is loaded lazily when the
user navigates to it, and eagerly re-fetched whenever `ThreadsUser` is changed
in settings and the user closes the overlay.

## Running

```
go build -o hnfeed .
```

Requires Go 1.26+. `Ctrl+C` to exit, `F1`/`Ctrl+F` for feed, `F2`/`Ctrl+T` for threads,
`F10`/`?` for settings. On the Threads page: `↑`/`↓` to select, `←` to fold,
`→` to expand, `Space`/`Enter` to toggle collapse, `R` to refresh.

## Guidelines

- Split logic across `hn.go` (API), `model.go` (tea model/update), `view.go` (rendering), `format.go` (entry formatting), `config.go` (settings), `style.go` (lipgloss styles/helpers), `feed.go` (state/entry structs), `threads.go` (thread tree state & flattening), `main.go` (entry point). No build tags.
- Use `fit`/`truncPad` from `style.go` for ANSI-safe width accounting.
- Every feed entry produces exactly 4 lines. The `totalLines()` helper computes `len(entries) * 4`.
- Thread entries have variable height (word-wrapped comment text). Flat lines are pre-computed during `Update()` (not in `View()`) via `flattenForest()` and stored in `threadsState.flatLines`. `View()` only reads and renders them.
- Use `m.st.appendEntry(feedEntry{...})` to add feed entries. Do **not** call formatters at event time — create a `feedEntry` and let the view format it at render time.
- Trimming (capped at 500 entries / ~2000 lines) operates on `entries`, not lines: `trim * 4` when adjusting scroll.
- Populate `frontRanks` for all 30 items at startup before first live poll.
- Wrap network calls — never crash on transient API failure.
- Commands must never mutate model directly; send messages to `Update`.
- Model state must be mutated in `Update()` and returned — never in `View()` (value copy). The threads tree is flattened in `Update()` handlers (`threadsResultMsg`, `WindowSizeMsg`, `toggleCollapse`), not in `View()`.
- Model helper methods that need to mutate state and return a `tea.Cmd` in a single call use pointer receiver (`*model`), e.g. `maybeRefreshThreads()`. Normal model methods on `Init`/`Update` use value receiver per bubbletea convention.
- ANSI cursor highlighting on thread lines uses manual escape codes (`\033[48;5;237m` / `\033[49m`) and replaces lipgloss's full resets (`\033[0m`) with foreground-only resets (`resetFgBold = \033[39;22m`) to preserve the cursor background across styled segments.
- Avoid verbose comments. Keep inline comments minimal — the code should be self-documenting.
- All commits must follow [Conventional Commits](https://www.conventionalcommits.org/) and use title only (no body).
