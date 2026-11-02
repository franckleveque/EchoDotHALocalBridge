package logging

import "log/slog"

var levels = map[string]slog.Level{
	"DEBUG": slog.LevelDebug,
	"WARN":  slog.LevelWarn,
	"ERROR": slog.LevelError,
}

// ParseLevel maps a LOG_LEVEL value to a slog level. Unknown values mean INFO.
func ParseLevel(name string) slog.Level {
	level, ok := levels[name]
	if !ok {
		return slog.LevelInfo
	}
	return level
}
