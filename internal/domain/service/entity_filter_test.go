package service

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEntityFilter_Supports(t *testing.T) {
	ignored := []string{"zone.", "sun.", "weather."}

	tests := []struct {
		name     string
		entityID string
		expected bool
	}{
		{"Light", "light.test", true},
		{"Switch", "switch.test", true},
		{"Zone (ignored)", "zone.home", false},
		{"Sun (ignored)", "sun.sun", false},
		{"Weather (ignored)", "weather.london", false},
		{"No dot", "invalid_entity", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := NewEntityFilter(ignored, slog.New(slog.DiscardHandler))
			assert.Equal(t, tt.expected, f.Supports(tt.entityID))
		})
	}
}

func TestEntityFilter_LogsIgnoredEntities(t *testing.T) {
	var buf bytes.Buffer
	f := NewEntityFilter([]string{"zone."}, slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	f.Supports("zone.home")
	f.Supports("bad")
	assert.True(t, strings.Contains(buf.String(), "zone.home") && strings.Contains(buf.String(), "bad"))
}
