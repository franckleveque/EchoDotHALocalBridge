package logging

import (
	"bytes"
	"fmt"
	"log/slog"
	"reflect"
	"strings"
	"sync"
	"testing"
)

var testLogger = slog.New(slog.DiscardHandler)

func TestLineRing_KeepsLastN(t *testing.T) {
	r := NewLineRing(3, 4096, testLogger)
	for i := 1; i <= 5; i++ {
		fmt.Fprintf(r, "l%d", i)
	}
	if got, want := r.Lines(), []string{"l3", "l4", "l5"}; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v want %v", got, want)
	}
}

func TestLineRing_TruncatesLongLine(t *testing.T) {
	r := NewLineRing(3, 4096, testLogger)
	r.Write([]byte(strings.Repeat("a", 10000)))
	if l := r.Lines(); len(l) != 1 || len(l[0]) > 4096 {
		t.Errorf("unexpected lines, len=%d", len(l))
	}
}

func TestLineRing_StripsTrailingNewline(t *testing.T) {
	r := NewLineRing(3, 4096, testLogger)
	r.Write([]byte("abc\n"))
	if got := r.Lines(); !reflect.DeepEqual(got, []string{"abc"}) {
		t.Errorf("got %v", got)
	}
}

func TestLineRing_LinesIsCopy(t *testing.T) {
	r := NewLineRing(3, 4096, testLogger)
	r.Write([]byte("abc"))
	r.Lines()[0] = "changed"
	if got := r.Lines(); got[0] != "abc" {
		t.Errorf("buffer modified: %v", got)
	}
}

func TestLineRing_Concurrent(t *testing.T) {
	r := NewLineRing(50, 4096, testLogger)
	var wg sync.WaitGroup
	for g := 0; g < 20; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 100; i++ {
				r.Write([]byte("x"))
				r.Lines()
			}
		}()
	}
	wg.Wait()
	if n := len(r.Lines()); n != 50 {
		t.Errorf("len=%d want 50", n)
	}
}

func TestLineRing_LogsTruncationOnItsOwnLogger(t *testing.T) {
	var out bytes.Buffer
	r := NewLineRing(3, 10, slog.New(slog.NewTextHandler(&out, &slog.HandlerOptions{Level: slog.LevelDebug})))
	r.Write([]byte(strings.Repeat("a", 50)))
	if !strings.Contains(out.String(), "Log line truncated") {
		t.Errorf("expected truncation log, got %q", out.String())
	}
	if lines := r.Lines(); len(lines) != 1 || len(lines[0]) != 10 {
		t.Errorf("ring must hold only the truncated line, got %v", lines)
	}
}
