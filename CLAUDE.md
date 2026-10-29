# CLAUDE.md

quest is a terminal HTTP client in Go: what Postman does for everyday work
(build requests, send them, inspect responses, collections, environments),
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
```

Logs go to `$XDG_CACHE_HOME/quest/quest.log` (`~/.cache/quest/quest.log`);
`QUEST_LOG=debug` raises the level.

## Stack

Go 1.26+, Bubble Tea v1, Bubbles v1 (`key`, `help`, `textinput`), Lip Gloss
v1, `charmbracelet/x/ansi`, `skinleak/gloq` for logging, `net/http` for the
engine. YAML (`gopkg.in/yaml.v3`) arrives with the store. Every new
dependency needs a reason written here.

## Architecture

```
cmd/quest/main.go           open the log, query the terminal background, run tea.Program
internal/core/              no imports from internal/tui, ever; it will also power a headless CLI
  request/                  Request (method, URL, params, headers, body, timeout, redirects),
                            Validate, FullURL; Response and Timing
  engine/                   Engine.Do: net/http with a shared pooled transport, per-request
                            client for the redirect policy, capped body read, cancellation
    trace.go                httptrace hooks → DNS/connect/TLS/TTFB/transfer/total
internal/tui/
  app/                      root Model: key routing, focus, layout, frame assembly
    msgs.go                 tea.Msg types and the tea.Cmd that sends and formats a request
  components/               panes and widgets; each renders lines already cut to its width
    editor.go               method + URL input
    response.go             status line, body/headers/timing tabs, windowed scrolling
    sidebar.go              collections and active env (empty states until the store lands)
    help.go                 status-bar hints and the ? overlay, generated from keys
    text.go, units.go       Fit/Lines/Spread; duration and size formatting
  format/                   body → display lines: JSON indent + per-line highlighting, sanitize
  theme/                    the ONLY place colors, styles and icons are defined
  keys/                     the ONLY place keybindings are defined
```

### Data flow of a request

1. `enter` in the URL or `ctrl+s` anywhere → `Model.send()` validates the
   edited request (errors go to the footer), cancels any request in
   flight, bumps `seq`, and returns `send(...)` as a `tea.Cmd`.
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
  active tab are bold, so focus stays visible with `NO_COLOR`.
- **Microcopy is short and lowercase**: "nothing sent yet", "request
  cancelled", "window too small".
- **Plain Unicode icons only** (`▸ ▾ · ━ …`, in `theme`); no Nerd Font.
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
- **Method is cycled with `ctrl+t`**: the URL input takes printable keys,
  so the method needs a modifier. `f1` is help and `ctrl+c` quits while
  typing, because `?` and `q` are URL characters.
- **A URL without a scheme gets `http://`**, like curl.
- **A body without a Content-Type** gets `application/json` when it parses
  as JSON, else `text/plain; charset=utf-8`. The User-Agent defaults to
  `quest`.
- **Limits:** 30 s default timeout, 10 redirects, bodies read up to
  10 MiB (the status line says when a body was cut), display lines cut at
  4 KiB, bodies with a NUL in the first 512 bytes are shown as binary.
- **Timing with redirects** shows the phases of the last hop; total covers
  all hops. The waterfall anchors wait and transfer at the end.
- **Narrow windows** (inner width < 80) never put the sidebar beside the
  panes: it is shown alone while it has focus. `ctrl+b` turns it off and on
  (and focuses it when narrow). Below 40×10 a "window too small" note
  replaces the frame.

## Gotchas

- **`lipgloss.HasDarkBackground()` must run in `main` before
  `tea.NewProgram`**, or Bubble Tea's input reader can swallow the reply
  and startup stalls.
- **Don't use `bubbles/viewport` for bodies.** It wraps long lines and
  measures every line; `Response` renders only visible lines, truncated.
- **`View` assembles the frame line by line** from parts already cut to
  width. Lip Gloss's `Width`, `Height`, `Join*` re-measure every cell and
  made qitlog's frames 3× slower. Lines are not padded on the right.
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
| Keypress + full 220×60 frame, 2 MiB JSON response | ~60 µs | UI goroutine |
| `Engine.Do`, 64 KiB body, pooled local connection | ~0.2 ms | background |

Anything proportional to body size belongs in the `tea.Cmd`, not in
`Update` or `View`.

## Out of scope (do not build unless the owner asks)

Scripting and tests, auth helpers (OAuth etc.), WebSocket, gRPC, GraphQL,
Postman import/export, cloud sync, plugins, mouse-heavy UX. Leave clean
seams (core stays UI-free), but no speculative abstractions.

## Roadmap

Done (first milestone, vertical slice): theme, layout shell with panes,
status bar, help overlay and minimum size; method + URL editor; engine with
timing and cancellation; response viewer with highlighted JSON, headers
and a timing waterfall.

Next, in order:
1. Params, headers and body tabs in the request editor.
2. Environments: `internal/core/env`, `{{var}}` interpolation in URL,
   params, headers and body; `e` to switch.
3. Store: collections and requests as YAML in `~/.config/quest/`
   (lazy-loaded), sidebar tree with `/` filter, `ctrl+n` new request.
4. History: last N requests in a local file.
