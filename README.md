# HN Feed

A live, unified Hacker News feed in your terminal. Merges "new" stories and
front page into one chronological stream with colour-coded events.

![terminal](https://img.shields.io/badge/terminal-TUI-blueviolet)
[![Go](https://img.shields.io/badge/Go-1.26%2B-00ADD8)](https://go.dev)

## Quick start

```bash
go install codeberg.org/jvbf/hnfeed@latest
hnfeed
```

Requires Go 1.26+. Make sure `~/go/bin` is in your `$PATH` (or wherever
`$(go env GOBIN)` points). Go doesn't add it automatically.

`Ctrl+C` to quit.

## Controls

| Key | Action |
|---|---|
| `?` / `F1` | Toggle settings |
| `↑` / `↓` | Navigate settings |
| `Space` / `Enter` | Toggle filter |
| `←` / `→` | Adjust poll interval |
| `Esc` | Close settings |
| Mouse wheel | Scroll feed |

## Settings

Settings are saved to `hnfeed-settings.json` automatically.

| Setting | Default | Description |
|---|---|---|
| Front page events | on | Master toggle for all front-page events |
| &nbsp;&nbsp; Entered front page | on | `★ #N` when an item enters the top 30 |
| &nbsp;&nbsp; Ranking up | on | `↑ #N (was #M)` on rank improvement |
| &nbsp;&nbsp; Ranking down | off | `↓ #N (was #M)` on rank drop |
| &nbsp;&nbsp; Left front page | off | `✕ #N` when an item leaves the top 30 |
| New story events | on | Newly submitted stories |
| Poll interval | 30s | Seconds between refreshes (5–300) |

## Entry types

| Prefix | Meaning | Colour |
|---|---|---|
| `[HH:mm:ss]` | New story | Yellow |
| `★ #N` | Entered front page | Orange |
| `↑ #N (was #M)` | Ranked up | Orange |
| `↓ #N (was #M)` | Ranked down | Orange |
| `✕ #N` | Left front page | Gray |

## License

MIT
