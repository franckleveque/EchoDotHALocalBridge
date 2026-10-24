package logging

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

func TestNewHandler_WritesToBoth(t *testing.T) {
	var buf bytes.Buffer
	ring := NewLineRing(10, 4096)
	log := slog.New(NewHandler(&buf, ring, slog.LevelInfo))
	log.Info("hello")
	log.Debug("hidden")

	if !strings.Contains(buf.String(), "msg=hello") || strings.Contains(buf.String(), "hidden") {
		t.Errorf("unexpected out: %q", buf.String())
	}
	lines := ring.Lines()
	if len(lines) != 1 || !strings.Contains(lines[0], "msg=hello") {
		t.Errorf("unexpected ring: %v", lines)
	}
}
