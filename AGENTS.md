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
against `frontRanks` → front-page events (new entries and rank improvements).

## Entry types (4 lines each)

| Entry | Prefix | Colour |
|---|---|---|
| New story | `[HH:mm:ss]` | Yellow |
| Front-page entry | `★ #N` | Orange |
| Front-page rank-up | `↑ #N (was #M)` | Orange |

## Feed state (`feedState` struct)

| Field | Purpose |
|---|---|
| `buf` | Rendered ANSI lines (capped at 2000) |
| `frontRanks` | Last known rank per front-page item |
| `seenIDs` | IDs already emitted as new-story entries |
| `maxID` | Watermark for incremental new-story polling |
| `scroll` | Lines scrolled up from bottom (0 = live) |
| `totalItems` | Total entries ever appended |

## Startup

`Init()` launches async `seedFeedCmd`. On `seedResultMsg`: populate
`frontRanks` silently, emit `initialItems` front-page entries + newest stories,
set `ready = true` to begin live polls.

## Running

```
go build -o hnfeed . && ./hnfeed [-pollSeconds 30] [-initialItems 5] [-throttleLimit 10]
```

Requires Go 1.26+. Ctrl+C to exit.

## Guidelines

- Keep logic in `hn.go` or `main.go`. No build tags.
- Use `measureVisible`/`fit` for ANSI-safe width accounting.
- Every entry in `buf` must be exactly 4 lines.
- Use `appendEntry()` for adding entries; decrement `scroll` when trimming.
- Populate `frontRanks` for all 30 items at startup before first live poll.
- Wrap network calls — never crash on transient API failure.
- Commands must never mutate model directly; send messages to `Update`.
- All commits must follow [Conventional Commits](https://www.conventionalcommits.org/) and use title only (no body) unless absolutely necessary.
