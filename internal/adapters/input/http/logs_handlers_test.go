package http

import (
	"context"
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
	if strings.Contains(body, "innerHTML") {
		t.Error("page must never use innerHTML")
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
