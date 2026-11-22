package util

import (
	"io"
	"strings"
	"sync"
)

// LogBuffer keeps the tail of the log for the admin panel while still
// writing everything to stdout (captured by journalctl).
type LogBuffer struct {
	mu    sync.Mutex
	lines []string
	limit int
}

func NewLogBuffer(limit int) *LogBuffer {
	if limit < 10 {
		limit = 200
	}
	return &LogBuffer{limit: limit}
}

func (b *LogBuffer) Write(p []byte) (int, error) {
	parts := strings.Split(strings.TrimRight(string(p), "\n"), "\n")
	b.mu.Lock()
	for _, part := range parts {
		if part == "" {
			continue
		}
		b.lines = append(b.lines, part)
	}
	if len(b.lines) > b.limit {
		b.lines = append([]string(nil), b.lines[len(b.lines)-b.limit:]...)
	}
	b.mu.Unlock()
	return len(p), nil
}

func (b *LogBuffer) Lines(n int) []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	if n <= 0 || n > len(b.lines) {
		n = len(b.lines)
	}
	out := make([]string, n)
	copy(out, b.lines[len(b.lines)-n:])
	return out
}

func MultiWriter(dst io.Writer, b *LogBuffer) io.Writer {
	return io.MultiWriter(dst, b)
}
