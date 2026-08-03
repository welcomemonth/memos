package main

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pkg/errors"
)

func parseSlogLevel(s string) (slog.Level, error) {
	switch strings.ToLower(s) {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return slog.LevelInfo, errors.Errorf("unknown log level %q: must be debug, info, warn, or error", s)
	}
}

func newLogger(level slog.Level, w io.Writer) *slog.Logger {
	if logDir := os.Getenv("MEMOS_LOG_DIR"); logDir != "" {
		_ = os.MkdirAll(logDir, 0o755)
		name := fmt.Sprintf("memos-%s.log", time.Now().Format("20060102-150405"))
		f, err := os.Create(filepath.Join(logDir, name))
		if err == nil {
			w = io.MultiWriter(w, f)
			fmt.Fprintf(os.Stderr, "[log] also writing to %s\n", filepath.Join(logDir, name))
		}
	}
	return slog.New(slog.NewTextHandler(w, &slog.HandlerOptions{Level: level}))
}
