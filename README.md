# quest

[![ci](https://github.com/skinleak/quest/actions/workflows/ci.yml/badge.svg)](https://github.com/skinleak/quest/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/skinleak/quest.svg)](https://pkg.go.dev/github.com/skinleak/quest)

A fast, calm HTTP client for the terminal. Build a request, send it, read
the response, all without leaving the keyboard.

<!-- screenshot / GIF goes here -->

> **Status: early development (v0.x).** The core loop works: edit a
> request with its params, headers and body (or pick one of your
> project's endpoints), send it, inspect the response. History and
> environments are next. Expect changes between versions.

## Features

Working now:

- Run quest in a project and press `ctrl+p` to pick from its endpoints,
  found in OpenAPI specs (2.0 and 3.x, JSON or YAML) and `.http`/`.rest`
  files. Filter by typing; `enter` loads the method, URL, params, headers
  and an example body into the editor. Typing any URL works as before
- GET, POST, PUT, PATCH, DELETE, HEAD and OPTIONS, always in view and
  picked with the arrow keys, with timeouts and redirects
- Query params, headers and a request body, each in its own tab. Params
  stay in sync with the URL, any row can be switched off without deleting
  it, and a JSON body is highlighted as you type
- Response status, time, size and content type
- Timing (DNS, connect, TLS, wait, transfer) as a small waterfall, with the
  server address, protocol, TLS version, certificate and its expiry, and
  redirects followed
- Response headers, with long values (cookies, policies) wrapped in full
- Pretty-printed, highlighted JSON, also when it is sent as `text/plain`
  or without a content type. Other content types are shown as plain text,
  and binary bodies are recognized
- Smooth with multi-MB responses: only visible lines are rendered and
  highlighted
- Cancel a request in flight with `esc`
- A help overlay, and key hints that change with the focused pane
- Works in any terminal size, follows your terminal's colors, and honors
  `NO_COLOR`

Planned:

- Request history, with each endpoint's last response time in the list
- Environments with `{{variable}}` interpolation

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
| `ctrl+p` | endpoints of the project, from anywhere |
| `enter` / `↑` `↓` / `esc` (in the endpoint list) | load / select / close; typing filters |
| `ctrl+t` | cycle the method, from anywhere |
| `←` `→` / `h` `l` (on the method row) | pick the method |
| `enter` (on the method row) | edit the url |
| `tab` / `shift+tab` | next / previous pane: method row, url, params/headers/body, response |
| `l` `h` / `→` `←` (on params/headers/body) | next / previous request tab |
| `j` `k` / `↓` `↑` (on params/headers) | select a row |
| `enter` (on params/headers/body) | edit the row (`name: value` or `name=value`) or the body |
| `a` | add a param or header |
| `space` | switch a param or header on or off |
| `d` | delete a param or header |
| `esc` (while editing) | done editing |
| `j` `k` / `↓` `↑` | scroll |
| `ctrl+d` `ctrl+u` / `pgdn` `pgup` | page down / up |
| `g` `G` | top / bottom |
| `l` `h` / `→` `←` | next / previous response tab |
| `?` (`f1` while typing) | help |
| `q` (`ctrl+c` while typing) | quit |

Planned: `e` environments.

## Endpoints in your project

quest looks for endpoints in the directory it starts in and below,
skipping dot-directories, `node_modules`, `vendor`, `dist`, `build` and
`target`:

- **OpenAPI / Swagger specs**, by name (`openapi.yaml`, `swagger.json`,
  `petstore.openapi.yml`) or by an `openapi:` / `swagger:` key at the top.
  Required query and header params are switched on, optional ones off;
  the body comes from the spec's example or is built from its schema.
- **`.http` and `.rest` files**, as used by the VS Code REST Client and
  JetBrains: requests separated by `###`, with `@name = value` variables.

A spec without an absolute server URL gets `{{baseUrl}}`, and path
parameters stay as `{id}`: quest asks you to fill them in before sending.
Nothing is sent until you press `enter`.

## Logs

quest never writes to the terminal it draws on. Diagnostics go to
`~/.cache/quest/quest.log`; set `QUEST_LOG=debug` for more detail. URLs,
headers and bodies are never logged.

## License

[MIT](LICENSE)
