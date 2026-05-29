# HN Feed

A live, unified Hacker News feed in your terminal. Merges "new" stories and
front page into one chronological stream with colour-coded events, plus a
threads view that shows a user's recent comments and replies.

![terminal](https://img.shields.io/badge/terminal-TUI-blueviolet)
[![Go](https://img.shields.io/badge/Go-1.26%2B-00ADD8)](https://go.dev)

## Quick start

```bash
go install codeberg.org/jvbf/hnfeed@latest
hnfeed
```

Requires Go 1.26+. Make sure `~/go/bin` is in your `$PATH`.

`Ctrl+C` to quit.

## Controls

| Key | Action |
|---|---|
| `F1` / `Ctrl+F` | Switch to Feed page |
| `F2` / `Ctrl+T` | Switch to Threads page |
| `F10` / `?` | Toggle settings |
| `R` | Refresh threads |
| `↑` / `↓` | Navigate settings / threads |
| `←` / `→` | Adjust numeric setting / fold or expand thread |
| `Space` / `Enter` | Toggle filter in settings / collapse thread |
| `-` | Decrement numeric setting |
| `PgUp` / `PgDown` / `Home` / `End` | Scroll threads |
| `Esc` | Close settings |

## Settings

Saved to `hnfeed-settings.json` automatically.

| Setting | Default | Description |
|---|---|---|
| Front page events | on | Master toggle for all front-page events |
| &nbsp;&nbsp; Entered front page | on | `★ #N` when an item enters the top 30 |
| &nbsp;&nbsp; Ranking up | on | `↑ #N (was #M)` on rank improvement |
| &nbsp;&nbsp;&nbsp;&nbsp; Compare to best rank | on | Only show rank-up on new best rank (`↑ #N (best #M)`) |
| &nbsp;&nbsp; Ranking down | off | `↓ #N (was #M)` on rank drop |
| &nbsp;&nbsp;&nbsp;&nbsp; Compare to worst rank | on | Only show rank-down on new worst rank (`↓ #N (worst #M)`) |
| &nbsp;&nbsp; Left front page | off | `✕ #N` when an item leaves the top 30 |
| New story events | on | Newly submitted stories |
| Poll interval | 30s | Seconds between refreshes (5–300) |
| Initial items | 5 | Stories loaded from each source on startup |
| Threads user | — | HN username for the Threads page |

## Entry types

| Prefix | Meaning | Colour |
|---|---|---|
| `[HH:mm:ss]` | New story | Yellow |
| `★ #N` | Entered front page | Orange |
| `↑ #N (was #M)` | Ranked up | Orange |
| `↑ #N (best #M)` | Ranked up to new best rank | Orange |
| `↓ #N (was #M)` | Ranked down | Orange |
| `↓ #N (worst #M)` | Ranked down to new worst rank | Orange |
| `✕ #N` | Left front page | Gray |

## License

MIT
