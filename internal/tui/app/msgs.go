package app

import (
	"context"
	"log/slog"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/skinleak/quest/internal/core/discover"
	"github.com/skinleak/quest/internal/core/engine"
	"github.com/skinleak/quest/internal/core/request"
	"github.com/skinleak/quest/internal/tui/format"
	"github.com/skinleak/quest/internal/tui/theme"
)

// responseMsg is a finished request. seq tells a stale result (from a
// request that was replaced) from the current one.
type responseMsg struct {
	seq  int
	resp *request.Response
	doc  *format.Doc
	err  error
}

// send runs r under ctx and formats the body, all off the UI goroutine.
// The first chunk is highlighted here too, so the first frame is colored.
// Only the method, host and outcome are logged: URLs and headers can carry
// secrets.
func send(ctx context.Context, eng *engine.Engine, log *slog.Logger, st *theme.Styles, seq int, r request.Request) tea.Cmd {
	return func() tea.Msg {
		resp, err := eng.Do(ctx, r)
		host := ""
		if u, uerr := r.FullURL(); uerr == nil {
			host = u.Host
		}
		if err != nil {
			log.Info("request failed", "method", r.Method, "host", host, "error", err)
			return responseMsg{seq: seq, err: err}
		}
		log.Debug("request done", "method", r.Method, "host", host,
			"status", resp.StatusCode, "bytes", resp.Size, "total", resp.Timing.Total)
		doc := format.New(resp.Body, resp.ContentType, st)
		doc.Highlight(0, format.ChunkLines)
		return responseMsg{seq: seq, resp: resp, doc: doc}
	}
}

// searchTimeout bounds a search for endpoints, for very large trees.
const searchTimeout = 10 * time.Second

// discoveredMsg is a finished search for the project's endpoints.
type discoveredMsg struct {
	res discover.Result
}

// search looks for endpoints under root off the UI goroutine. A search
// that times out returns what it found so far, marked partial. Only counts
// are logged.
func search(root string, log *slog.Logger) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), searchTimeout)
		defer cancel()
		start := time.Now()
		res, err := discover.Find(ctx, root)
		if err != nil {
			res.Partial = true
		}
		log.Debug("endpoints found", "count", len(res.Endpoints), "files", res.Files,
			"unreadable", len(res.Failed), "partial", res.Partial, "took", time.Since(start))
		return discoveredMsg{res: res}
	}
}
