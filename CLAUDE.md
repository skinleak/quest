# CLAUDE.md

quest is a terminal HTTP client in Go: what Postman does for everyday work
(build requests, send them, inspect responses, environments),
with a calm, keyboard-first TUI. It belongs with the owner's other tools
(`skinleak/qitlog`, `skinleak/gloq`) and should look and feel like qitlog.
Priorities, in order: **a clean, consistent UI; performance; a small,
solid core**. Do not add features, options or dependencies that were not
asked for.

## How we work

- **Never commit or push.** The owner handles git; leave changes in the
  working tree and list new and changed files at the end.
- Decide design details yourself within the rules below and say in a
  sentence or two what you chose and why. Ask only for real product
  choices (what a key does, where a feature lives), with a recommendation.
- A change is finished when `make check` passes (gofmt, vet, build, test,
  golangci-lint), every bug fix and parser has a test that fails without
  it, hot-path benchmarks stay in the budget below, and the docs match the
  code: README keys table and status, this file, and `keys.FullHelp()`.
- Report at the end: what changed, bugs found on the way, what you chose
  not to do and why.

## Commands

```sh
make build        # bin/quest
make run          # go run ./cmd/quest
make test         # go test ./...
make race         # go test -race ./...
make bench        # hot-path benchmarks
make lint         # golangci-lint (v2 config in .golangci.yml)
make check        # the definition of done
go run ./cmd/quest --version
```

CI (`.github/workflows/ci.yml`) runs the same checks on Linux and macOS
(tests with `-race`), cross-compiles for Windows, and runs golangci-lint,
on every push to main and every pull request.

Logs go to `$XDG_CACHE_HOME/quest/quest.log` (`~/.cache/quest/quest.log`);
`QUEST_LOG=debug` raises the level.

## Stack

Go 1.26+, Bubble Tea v1, Bubbles v1 (`key`, `help`, `textinput`), Lip Gloss
v1, `charmbracelet/x/ansi`, `skinleak/gloq` for logging, `net/http` for the
engine, `go.yaml.in/yaml/v3` for OpenAPI specs. Every new dependency needs
a reason written here.

- **`go.yaml.in/yaml/v3`**: most OpenAPI specs are YAML. It is the YAML
  org's maintained continuation of `gopkg.in/yaml.v3`, has no
  dependencies, adds 0.05 ms of init, and parses JSON specs too, so both
  go through one `yaml.Node` path that keeps the spec's key order.

## Architecture

```
cmd/quest/main.go           --version, open the log, query the terminal background, run tea.Program
internal/core/              no imports from internal/tui, ever; it will also power a headless CLI
  request/                  Request (method, URL, params, headers, body, timeout, redirects),
                            Validate, FullURL; Response (with remote address, TLS, redirects)
                            and Timing
  engine/                   Engine.Do: net/http with a shared pooled transport, per-request
                            client for the redirect policy, capped body read, cancellation
    trace.go                httptrace hooks → DNS/connect/TLS/TTFB/transfer/total
  discover/                 Find: walks a project for OpenAPI specs and .http files → Endpoints
    openapi.go              OpenAPI 2/3 on yaml.Node: servers, params, $ref, example bodies
    httpfile.go             .http/.rest files: ### blocks, @variables, headers, body
    goroutes.go             Go routes via go/parser: gin, echo, chi, ServeMux; groups, listen addr
internal/tui/
  app/                      root Model: key routing, focus (method row, URL, request tabs,
                            response), frame assembly
    msgs.go                 tea.Msg types; the tea.Cmds that send a request and search
                            for endpoints
  components/               panes and widgets; each renders lines already cut to its width
    editor.go               method row (every method, active one colored), URL input, and
                            params/headers/body tabs; params are synced with the URL's query
    textarea.go             multi-line body editor: no wrapping, renders visible lines only
    picker.go               the ctrl+p endpoint list: filter input, aligned rows
    response.go             status line, body/headers/timing tabs, windowed scrolling
    help.go                 status-bar hints and the ? overlay, generated from keys
    text.go, units.go       Fit/Lines/Spread; duration and size formatting
  format/                   body → display lines: JSON indent + per-line highlighting, sanitize
  theme/                    the ONLY place colors, styles and icons are defined
  keys/                     the ONLY place keybindings are defined
```

### Data flow of a request

1. `enter` in the URL or `ctrl+s` anywhere (also while editing a row or
   the body) → `Model.send()` validates the edited request (errors go to
   the footer), cancels any request in flight, bumps `seq`, and returns
   `send(...)` as a `tea.Cmd`.
2. The command runs `engine.Do`, then `format.New` on the body and
   highlights the first chunk, all off the UI goroutine, and returns a
   `responseMsg`.
3. `applyResponse` drops results whose `seq` is stale. A cancelled request
   brings back what the pane showed before and says "request cancelled".
4. Scrolling calls `Response.prepare()`, which highlights only the chunks
   about to be shown; `View` only reads.

## Rules

- **`internal/core` never imports `internal/tui`.**
- **Colors, styles and icons live only in `theme`; bindings only in
  `keys`.** Add a binding to `KeyMap` and to `FullHelp()` and the overlay
  shows it; `TestHelpShowsEveryKey` checks that.
- **Keep the look calm, like qitlog:** no borders or boxes, spacing instead
  of separators, one accent (blue). Colors are bright ANSI indices
  (SGR 90–97), so the terminal theme picks the shades and 16-color,
  256-color and truecolor terminals all work; `NO_COLOR` is honored by
  Lip Gloss. Other colors carry meaning only: status classes (2xx green,
  3xx cyan, 4xx yellow, 5xx red), methods (GET green, POST yellow, PUT
  cyan, PATCH purple, DELETE red, others muted) and JSON tokens (keys
  blue, strings green, numbers cyan, literals purple, punctuation muted).
- **Focus never relies on color alone.** The focused pane's title and the
  active tab are bold, and the active method is underlined while the
  method row has focus, so focus stays visible with `NO_COLOR`.
- **Microcopy is short and lowercase**: "nothing sent yet", "request
  cancelled", "window too small".
- **Plain Unicode icons only** (`· ━ … →`, in `theme`); no Nerd Font.
- **Untrusted text goes through `format.Sanitize`** before display: bodies,
  header names and values, status text, error messages. Responses can
  carry escape sequences, including C1 controls inside valid JSON strings.
- **Network and disk work runs in a `tea.Cmd`.** Never in `Update` or `View`.
- **Errors never crash the app.** They go to the footer (`setMessage`) or
  the response pane, and clear on the next key.
- **Log diagnostics, not data.** Requests log method, host and outcome;
  never full URLs, headers or bodies, which carry tokens. Never log to
  stdout or stderr while the UI runs.
- **No global mutable state.** Everything hangs off `Model` or `Engine`.

## Decisions (and why)

- **Borderless, qitlog style**, though the first sketch had boxes; chosen
  with the owner.
- **Bubble Tea v1, not v2:** same as qitlog, so its lessons apply as is.
- **No chroma.** It was in the original plan, but its JSON lexer is
  stateful (key vs value depends on the object/array stack), so it can't
  highlight a chunk in the middle of a document, and it was 36× slower
  (415 ms vs 11.5 ms for 0.5 MB, 30× the memory). Importing its `lexers`
  package also parses ~250 lexers at init (4.6 ms of startup). Indented
  JSON never splits a token across lines, so `format` colors each line on
  its own (a string followed by `:` is a key). Revisit only if HTML/XML
  highlighting is asked for, and then load a single lexer lazily.
- **Every method is always in view**, in a row above the URL with the
  active one in its color (underlined while the row has focus). The row is
  a focus stop before the URL (`shift+tab` from the URL): `←`/`→` pick,
  `enter` goes to the URL. `ctrl+t` cycles it from anywhere, because the
  URL input takes printable keys. `f1` is help and `ctrl+c` quits while
  typing, because `?` and `q` are URL characters.
- **No collections and no sidebar**: dropped by the owner; the request
  and response take the full width. A project's own endpoints (found in
  OpenAPI specs and `.http` files) are picked from the `ctrl+p` list
  instead; chosen with the owner.
- **Endpoints come from the project**, never from a quest file: OpenAPI
  specs (found by name, or by an `openapi:`/`swagger:` key in their first
  KiB) and `.http`/`.rest` files under the directory quest starts in.
  `Init` starts the search, and every `ctrl+p` searches again (the last
  list shows meanwhile), so edited specs show up. The walk skips
  dot-directories and dependency/build dirs and stops after 20,000
  entries or 10 s ("search stopped early"); files over 16 MiB are skipped.
  Go source is read too (below); other languages were left out: each
  needs its own regex-based reader and breaks easily.
- **Go routes come from the AST, never from running code.** Only files
  importing `net/http`, gin, echo or chi are parsed (a byte search skips
  the rest, and `_test.go`). A route needs a literal path (`/…`, or `""`
  under a group) and a handler argument, which keeps `http.Get(url)` and
  `c.Get("key")` out. Group prefixes are tracked per variable or field
  (`api`, `s.router`), through chained `Group` calls and chi's `Route`,
  and across functions: a group passed as an argument prefixes the
  callee's routes, matched by function name and resolved over up to four
  passes (a parameter passed different prefixes gets none). Path params
  become `{name}` so "fill in" applies. The base URL is the first literal
  listen address (`Run`, `Start`, `ListenAndServe`, `http.Server{Addr}`;
  a bare gin `Run()` is `:8080`), else `{{baseUrl}}`. Handlers are named
  by identifier; an adapter (`gin.WrapF`, `http.HandlerFunc`) by what it
  wraps.
- **Picked endpoints are filled in, not sent**: required params and
  headers on, optional ones off; the body is the spec's example or one
  built from its schema (spec key order, zero values, `$ref` and `allOf`
  followed, recursion cut at depth 8). A non-JSON body only sets
  Content-Type. A spec without an absolute server gets `{{baseUrl}}` for
  environments to fill in.
- **Placeholders block sending**: a `{{variable}}` anywhere in the URL or
  a `{name}` in its path gives "fill in {id}" instead of a parse error or
  a request to `/users/%7Bid%7D`. Braces in the query (JSON filters) are
  fine.
- **The list filters by words**: every word typed must appear in the
  method, path, name or file; predictable, unlike fuzzy matching.
- **Params are the URL's query**, as in Postman: the params tab lists it
  raw (nothing decoded), editing a param rewrites it, and typing in the
  URL updates the list. A disabled param keeps its row but leaves the URL.
  A typed param only escapes what would break the query (`& # =` and
  space), so the URL stays readable.
- **Rows are edited as one line**, `name: value` for headers and
  `name=value` for params, so a pasted header just works. Edits apply as
  you type; `esc` (or `enter`) ends editing, and a row left empty goes.
- **Own body editor, not `bubbles/textarea`**: that one renders and wraps
  the whole buffer on every key and caps input at 10,000 lines of 500
  columns. `TextArea` never wraps and renders only visible lines; tabs
  become two spaces so every rune is one cell. A body that starts with
  `{` or `[` is highlighted line by line with `format.Highlighter`.
- **The request area grows with its content**: as many lines as its
  longest tab (so switching tabs doesn't move the response), up to two
  fifths of the free height and at most 12 lines.
- **Footer hints leave out self-explanatory keys** (arrows, page up and
  down, top and bottom); the `?` overlay lists every binding.
- **Timing tab** shows only the phases that happened (a reused connection
  has no dns/connect/tls rows), then server address, protocol and TLS
  version, certificate subject, issuer and expiry (yellow within 14 days,
  red once expired), and redirects with the final URL.
- **Header values wrap** under their column instead of being cut; names
  longer than 28 cells are cut so the values stay aligned.
- **JSON is sniffed** in `text/plain` and untyped bodies that start with
  `{` or `[`, since many APIs send JSON that way; if it doesn't parse it
  stays plain text.
- **A URL without a scheme gets `http://`**, like curl.
- **A body without a Content-Type** gets `application/json` when it parses
  as JSON, else `text/plain; charset=utf-8`. The User-Agent defaults to
  `quest`.
- **Limits:** 30 s default timeout, 10 redirects, bodies read up to
  10 MiB (the status line says when a body was cut), display lines cut at
  4 KiB, bodies with a NUL in the first 512 bytes are shown as binary.
- **Timing with redirects** shows the phases of the last hop; total covers
  all hops. The waterfall anchors wait and transfer at the end.
- **Narrow windows**: the method row closes up to single spaces, then shows
  only the active method. Below 40×10 a "window too small" note replaces
  the frame.

## Gotchas

- **`lipgloss.HasDarkBackground()` must run in `main` before
  `tea.NewProgram`**, or Bubble Tea's input reader can swallow the reply
  and startup stalls.
- **Don't use `bubbles/viewport` for bodies.** It wraps long lines and
  measures every line; `Response` renders only visible lines, truncated.
- **`View` assembles the frame line by line** from parts already cut to
  width. Lip Gloss's `Width`, `Height`, `Join*` re-measure every cell and
  made qitlog's frames 3× slower. Lines are not padded on the right.
- **`theme.Styles` and `keys.KeyMap` are shared by pointer.** Bubble Tea
  copies the `Model` on every message and `View`; with styles inside it
  was 57 KB per copy and frames were 2× slower. Keep big immutable things
  behind pointers and pass `*theme.Styles` to render helpers.
- **Hot paths color text with `theme.Paint`** (escape codes taken from one
  `Render`), not a `Render` per token.
- **Lip Gloss renders no colors in tests** (no terminal), so a style
  assertion passes trivially. Use `withColors(t)` (see
  `format/format_test.go`) and check the test fails without its fix.
- **The textinput cursor is static**: calmer, no redraw every 530 ms.
- **`tracer` must not be reassigned while its mutex is held**; reset the
  fields one by one (`GetConn` starts each hop).
- **`help.ShortHelpView` can exceed its width** when not even "…" fits,
  so `Help.Hints` cuts the result explicitly.
- **Tests never reach the network.** Send only to `httptest` servers; a
  spec or URL that a test only loads must point at `127.0.0.1:1`, so a
  broken assertion can't send a request to a real host.
- **`TestViewFitsWindow`** checks every mode at many sizes, down to 1×1:
  exactly the window's height, never wider. Keep it passing on any layout
  change, and add new modes to it.

## Performance budget

Measured with `make bench` (Intel Core Ultra 7 268V). Keep within ~2×.

| Path | Cost | Runs on |
|---|---|---|
| Startup to first frame (incl. background query) | ~22 ms | — |
| Package init, whole binary | ~1 ms | — |
| `format.New`, 1 MiB compact JSON (indent, split, sanitize) | ~13 ms | background |
| Highlight one 256-line chunk | ~20 µs | UI goroutine |
| Keypress + full 220×60 frame, 2 MiB JSON response | ~48 µs | UI goroutine |
| `Engine.Do`, 64 KiB body, pooled local connection | ~0.2 ms | background |
| Parse an OpenAPI spec, 1,000 operations (~190 KiB YAML) | ~13 ms | background |
| Search a home directory (stops at 20,000 entries) | ~80 ms | background |
| Search Go's own source tree for routes (~10k .go files) | ~190 ms | background |

Anything proportional to body size belongs in the `tea.Cmd`, not in
`Update` or `View`.

## Releasing

The module lives at `github.com/skinleak/quest`. Users install with
`go install github.com/skinleak/quest/cmd/quest@latest`, which resolves to
the highest `vX.Y.Z` tag; untagged commits on main never reach them. A
release is a tag on a merged, CI-green commit of `main`:

```sh
git pull
git tag -a v0.2.0 -m "v0.2.0: <what changed>"
git push origin v0.2.0
```

`.github/workflows/release.yml` then asks proxy.golang.org for the tag,
which makes it `@latest` at once. Check from outside the repo with
`go list -m github.com/skinleak/quest@latest`. Bump the minor version for
features and the patch version for fixes; `v0.x` means no compatibility
promises. Tags are permanent in the Go proxy: never move or delete one,
release a new patch instead. The README install command stays `@latest`.
`--version` reads the version Go stamps into the binary, so there is
nothing to bump in code.

## Out of scope (do not build unless the owner asks)

Scripting and tests, auth helpers (OAuth etc.), WebSocket, gRPC, GraphQL,
Postman import/export, collections (beyond endpoints found in the
project), cloud sync, plugins, mouse-heavy UX.
Leave clean seams (core stays UI-free), but no speculative abstractions.

## Roadmap

Released as `v0.1.0` (first milestone, vertical slice): theme, layout shell with panes,
status bar, help overlay and minimum size; method + URL editor; engine with
timing and cancellation; response viewer with highlighted JSON, headers
and a timing waterfall.

Released as `v0.2.0`: params, headers and body tabs in the request editor.

Released as `v0.3.0`: endpoint discovery (OpenAPI and `.http` files) and
the `ctrl+p` endpoint list.

Done since: Go routes (gin, echo, chi, ServeMux) in the endpoint list.

Next, in order (agreed with the owner):
1. History in `$XDG_CACHE_HOME/quest/`, per project directory (nothing is
   written into the repo); the picker shows each endpoint's last status
   and response time.
2. Environments: `{{var}}` interpolation in URL, params, headers and
   body; base URLs seeded from OpenAPI `servers`; `e` to switch.
