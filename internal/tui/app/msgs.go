package app

import (
	"context"
	"log/slog"

	tea "github.com/charmbracelet/bubbletea"

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
func send(ctx context.Context, eng *engine.Engine, log *slog.Logger, st theme.Styles, seq int, r request.Request) tea.Cmd {
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
