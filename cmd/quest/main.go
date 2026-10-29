// Command quest is a fast, keyboard-first HTTP client for the terminal.
package main

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/skinleak/gloq"

	"github.com/skinleak/quest/internal/core/engine"
	"github.com/skinleak/quest/internal/tui/app"
)

func main() {
	os.Exit(run())
}

func run() int {
	logFile, log := openLog()
	if logFile != nil {
		defer logFile.Close()
	}

	// Detect the terminal background before Bubble Tea takes over stdin;
	// otherwise the terminal's reply can be swallowed by the input reader
	// and the first render stalls until the query times out.
	lipgloss.HasDarkBackground()

	m := app.New(engine.New(engine.Options{}), log)
	if _, err := tea.NewProgram(m, tea.WithAltScreen()).Run(); err != nil {
		log.Error("program failed", "error", err)
		fmt.Fprintln(os.Stderr, "quest:", err)
		return 1
	}
	return 0
}

// openLog logs to quest.log in the user's cache directory, never to the
// terminal the UI draws on. QUEST_LOG sets the level (default info). When
// the file can't be opened, logs are discarded rather than failing start.
func openLog() (*os.File, *slog.Logger) {
	opts := []gloq.Option{
		gloq.WithFormat(gloq.FormatPretty),
		gloq.WithColor(gloq.ColorNever),
		gloq.WithLevelFromEnv("QUEST_LOG"),
	}
	dir, err := os.UserCacheDir()
	if err == nil {
		dir = filepath.Join(dir, "quest")
		err = os.MkdirAll(dir, 0o700)
	}
	if err != nil {
		return nil, slog.New(gloq.NewHandler(io.Discard, opts...))
	}
	f, err := os.OpenFile(filepath.Join(dir, "quest.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return nil, slog.New(gloq.NewHandler(io.Discard, opts...))
	}
	return f, slog.New(gloq.NewHandler(f, opts...))
}
