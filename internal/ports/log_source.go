package ports

// LogSource exposes recent log lines in chronological order (a copy).
type LogSource interface {
	Lines() []string
}
