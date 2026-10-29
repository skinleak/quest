# quest

[![ci](https://github.com/skinleak/quest/actions/workflows/ci.yml/badge.svg)](https://github.com/skinleak/quest/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/skinleak/quest.svg)](https://pkg.go.dev/github.com/skinleak/quest)

A fast, calm HTTP client for the terminal. Build a request, send it, read
the response, all without leaving the keyboard.

<!-- screenshot / GIF goes here -->

> **Status: early development (v0.x).** The core loop works: edit a
> request, send it, inspect the response. Collections, environments and
> history are next. Expect changes between versions.

## Features

Working now:

- GET, POST, PUT, PATCH, DELETE, HEAD and OPTIONS, with timeouts and
  redirects
- Response status, size and timing (DNS, connect, TLS, wait, transfer) as a
  small waterfall
- Pretty-printed, highlighted JSON. Other content types are shown as plain
  text, and binary bodies are recognized
- Smooth with multi-MB responses: only visible lines are rendered and
  highlighted
- Cancel a request in flight with `esc`
- A help overlay, and key hints that change with the focused pane
- Works in any terminal size, follows your terminal's colors, and honors
  `NO_COLOR`

Planned:

- Query params, headers and request body in the editor
- Collections stored as human-readable, git-friendly YAML in
  `~/.config/quest/`
- Environments with `{{variable}}` interpolation
- Request history

## Install

```sh
go install github.com/skinleak/quest/cmd/quest@latest
```

Or build from source (Go 1.26+):

```sh
git clone https://github.com/skinleak/quest
cd quest
make build    # writes bin/quest
```

`quest --version` prints the installed version.

## Keys

| Key | Action |
|---|---|
| `enter` (in the URL) / `ctrl+s` | send |
| `esc` / `ctrl+c` while sending | cancel the request |
| `ctrl+t` | next method |
| `tab` / `shift+tab` | next / previous pane |
| `ctrl+b` | show or hide the sidebar |
| `j` `k` / `↓` `↑` | scroll |
| `ctrl+d` `ctrl+u` / `pgdn` `pgup` | page down / up |
| `g` `G` | top / bottom |
| `l` `h` / `→` `←` | next / previous response tab |
| `?` (`f1` while typing) | help |
| `q` (`ctrl+c` while typing) | quit |

Planned: `ctrl+n` new request, `e` environments, `/` filter collections.

## Logs

quest never writes to the terminal it draws on. Diagnostics go to
`~/.cache/quest/quest.log`; set `QUEST_LOG=debug` for more detail. URLs,
headers and bodies are never logged.

## License

[MIT](LICENSE)
