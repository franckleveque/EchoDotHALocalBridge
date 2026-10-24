package http

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"hue-bridge-emulator/internal/ports"
)

type stubAuth struct{}

func (stubAuth) Verify(ctx context.Context, u, p string) (bool, error) { return true, nil }
func (stubAuth) CreateCredentials(ctx context.Context, u, p string) error {
	return nil
}
func (stubAuth) Exists() bool { return true }

func newLogsTestServer() *Server {
	return NewServer(nil, nil, stubAuth{}, "127.0.0.1")
}

var _ ports.AuthService = stubAuth{}

func TestLogsPage_UsesTextContent(t *testing.T) {
	s := newLogsTestServer()
	req := httptest.NewRequest(http.MethodGet, "/admin/logs", nil)
	req.SetBasicAuth("a", "b")
	rec := httptest.NewRecorder()
	s.Mux().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "textContent") {
		t.Error("page must render values with textContent")
	}
	for _, banned := range []string{"innerHTML", "insertAdjacentHTML", "outerHTML"} {
		if strings.Contains(body, banned) {
			t.Errorf("page must never use %s", banned)
		}
	}
}

func TestLogsData_MethodNotAllowed(t *testing.T) {
	s := newLogsTestServer()
	for _, path := range []string{"/admin/logs", "/admin/logs/data"} {
		req := httptest.NewRequest(http.MethodPost, path, nil)
		req.SetBasicAuth("a", "b")
		rec := httptest.NewRecorder()
		s.Mux().ServeHTTP(rec, req)
		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("POST %s = %d, want 405", path, rec.Code)
		}
	}
}

func TestLogsData_NilSourceIsEmptyArray(t *testing.T) {
	s := newLogsTestServer()
	req := httptest.NewRequest(http.MethodGet, "/admin/logs/data", nil)
	req.SetBasicAuth("a", "b")
	rec := httptest.NewRecorder()
	s.Mux().ServeHTTP(rec, req)
	if got := strings.TrimSpace(rec.Body.String()); got != "[]" {
		t.Errorf("body = %q, want []", got)
	}
}

func TestLogsPage_HasEmptyStateMessages(t *testing.T) {
	s := newLogsTestServer()
	req := httptest.NewRequest(http.MethodGet, "/admin/logs", nil)
	req.SetBasicAuth("a", "b")
	rec := httptest.NewRecorder()
	s.Mux().ServeHTTP(rec, req)
	body := rec.Body.String()
	for _, msg := range []string{"No log lines yet.", "No lines match the filter."} {
		if !strings.Contains(body, msg) {
			t.Errorf("page missing empty-state message %q", msg)
		}
	}
}

func captureSlog(t *testing.T) *bytes.Buffer {
	t.Helper()
	buf := &bytes.Buffer{}
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return buf
}

func TestLogsDataPolling_DoesNotAddLogLines(t *testing.T) {
	buf := captureSlog(t)
	h := newLogsTestServer().Handler()
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest(http.MethodGet, "/admin/logs/data", nil)
		req.SetBasicAuth("a", "b")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
	}
	if buf.Len() != 0 {
		t.Errorf("polling /admin/logs/data must not log, got: %s", buf.String())
	}
}

func TestLoggingMiddleware_StillLogsOtherRoutes(t *testing.T) {
	buf := captureSlog(t)
	h := newLogsTestServer().Handler()
	req := httptest.NewRequest(http.MethodGet, "/admin/logs", nil)
	req.SetBasicAuth("a", "b")
	h.ServeHTTP(httptest.NewRecorder(), req)
	if !strings.Contains(buf.String(), "HTTP request") || !strings.Contains(buf.String(), "/admin/logs") {
		t.Errorf("expected request log line, got: %s", buf.String())
	}
}
