# AGENTS.md

## Project overview

A Go TUI that displays a live, unified Hacker News feed in the terminal. Merges
HN "new" and front page into one chronological stream using
[github.com/charmbracelet/bubbletea](https://github.com/charmbracelet/bubbletea).

## Files

| File | Responsibility |
|---|---|
| `hn.go` | `Item` struct and HN API fetchers |
| `main.go` | Bubbletea model, rendering, `feedState`, commands, `main()` |

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
| Front-page rank-down | `↓ #N (was #M)` | Orange |
| Front-page leave | `✕ #N` | Gray |

## Feed state (`feedState` struct)

| Field | Purpose |
|---|---|
| `buf` | Rendered ANSI lines (capped at 2000) |
| `frontRanks` | Last known rank per front-page item |
| `frontCache` | Last known `*Item` data for front-page items (used for leave events) |
| `seenIDs` | IDs already emitted as new-story entries |
| `maxID` | Watermark for incremental new-story polling |
| `scroll` | Lines scrolled up from bottom (0 = live) |
| `totalItems` | Total entries ever appended |

## Settings

Press `?` or `F1` to open the settings page (replaces the feed). Navigate with
`↑`/`↓`, toggle filters with `Space`/`Enter`, adjust numeric values with `←`/`→`, close with `Esc`.

| Field | Default | Purpose |
|---|---|---|
| `ShowFrontPage` | `true` | Master toggle for all front-page events |
| `FrontEntered` | `true` | Show `★ #N` when an item enters the front page |
| `FrontRankUp` | `true` | Show `↑ #N (was #M)` on rank improvement |
| `FrontRankDown` | `false` | Show `↓ #N (was #M)` on rank drop |
| `FrontLeft` | `false` | Show `✕ #N` when an item leaves the top 30 |
| `ShowNewStories` | `true` | Show new-story entries |
| `PollSeconds` | `30` | Seconds between refreshes |

Sub-options (`FrontEntered`, `FrontRankUp`, `FrontRankDown`, `FrontLeft`) are
indented under **Front page events** in the settings UI and are only shown
when `ShowFrontPage` is enabled.

The header shows `(settings)` while the settings page is open. Filtering only
affects newly arriving entries — existing ones in the buffer remain visible.

Settings are persisted to `hnfeed-settings.json` in the current directory.
Saved automatically on every toggle and loaded on startup. If the file is
missing or corrupt, defaults are used.

## Startup

`Init()` launches async `seedFeedCmd`. On `seedResultMsg`: populate
`frontRanks` silently, emit `initialItems` front-page entries + newest stories,
set `ready = true` to begin live polls.

## Running

```
go build -o hnfeed . && ./hnfeed [-pollSeconds 30] [-initialItems 5] [-throttleLimit 10]
```

Requires Go 1.26+. `Ctrl+C` to exit, `?`/`F1` for settings.

## Guidelines

- Keep logic in `hn.go` or `main.go`. No build tags.
- Use `measureVisible`/`fit` for ANSI-safe width accounting.
- Every entry in `buf` must be exactly 4 lines.
- Use `appendEntry()` for adding entries; decrement `scroll` when trimming.
- Populate `frontRanks` for all 30 items at startup before first live poll.
- Wrap network calls — never crash on transient API failure.
- Commands must never mutate model directly; send messages to `Update`.
- Avoid verbose comments. Keep inline comments minimal — the code should be self-documenting.
- All commits must follow [Conventional Commits](https://www.conventionalcommits.org/) and use title only (no body) unless absolutely necessary.
