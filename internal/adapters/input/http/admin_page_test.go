package http

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAdminPage_Markers(t *testing.T) {
	s := newLogsTestServer()
	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	req.SetBasicAuth("a", "b")
	rec := httptest.NewRecorder()
	s.Mux().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{"function testRow(", "function sendTest(", "function readDeviceForm(", "isUnsaved(", "Not sent: ", "Sending to Home Assistant...", "clearTimeout(statusTimer)"} {
		if !strings.Contains(body, want) {
			t.Errorf("admin page must contain %q", want)
		}
	}
	for _, banned := range []string{"Save config first", "/api/admin/lights/' + hueId"} {
		if strings.Contains(body, banned) {
			t.Errorf("admin page must not contain %q", banned)
		}
	}
}
