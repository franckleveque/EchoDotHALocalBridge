package logging

import (
	"strings"
	"sync"
)

// LineRing keeps the last maxLines log lines in memory. Each Write call is one line.
type LineRing struct {
	mu       sync.Mutex
	lines    []string
	start    int
	count    int
	maxBytes int
}

func NewLineRing(maxLines, maxLineBytes int) *LineRing {
	if maxLines < 1 {
		maxLines = 1
	}
	return &LineRing{lines: make([]string, maxLines), maxBytes: maxLineBytes}
}

func (r *LineRing) Write(p []byte) (int, error) {
	line := strings.TrimRight(string(p), "\r\n")
	if r.maxBytes > 0 && len(line) > r.maxBytes {
		line = line[:r.maxBytes]
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.count < len(r.lines) {
		r.lines[(r.start+r.count)%len(r.lines)] = line
		r.count++
	} else {
		r.lines[r.start] = line
		r.start = (r.start + 1) % len(r.lines)
	}
	return len(p), nil
}

// Lines returns a copy of the buffered lines in chronological order.
func (r *LineRing) Lines() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]string, r.count)
	for i := range out {
		out[i] = r.lines[(r.start+i)%len(r.lines)]
	}
	return out
}
