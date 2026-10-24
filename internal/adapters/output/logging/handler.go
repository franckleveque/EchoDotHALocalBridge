package logging

import (
	"io"
	"log/slog"
)

// NewHandler fans records out to two text handlers (stdout and the line ring) using the same format.
func NewHandler(out io.Writer, ring io.Writer, level slog.Leveler) slog.Handler {
	opts := &slog.HandlerOptions{Level: level}
	return slog.NewMultiHandler(slog.NewTextHandler(out, opts), slog.NewTextHandler(ring, opts))
}
