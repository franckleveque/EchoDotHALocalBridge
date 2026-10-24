package homeassistant

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"hue-bridge-emulator/internal/domain/model"
)

type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func captureLogs(t *testing.T) *syncBuffer {
	t.Helper()
	return captureLogsAt(t, slog.LevelDebug)
}

func captureLogsAt(t *testing.T, level slog.Level) *syncBuffer {
	t.Helper()
	buf := &syncBuffer{}
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{Level: level})))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return buf
}

func newTestClient(url string) *Client {
	c := NewClient()
	c.Configure(url, "test-token")
	return c
}

func testDevice() *model.Device {
	return &model.Device{ID: "1", ExternalID: "light.kitchen"}
}

func TestSetState_400ReturnsHAAPIErrorWithBody(t *testing.T) {
	captureLogs(t)
	const body = `{"message":"Service not found."}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	err := newTestClient(srv.URL).SetState(context.Background(), testDevice(), model.HomeAssistantCommand{Service: "light.turn_on"})
	var apiErr *model.HAAPIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected HAAPIError, got %v", err)
	}
	if apiErr.Status != 400 || apiErr.Body != body {
		t.Fatalf("unexpected error: %+v", apiErr)
	}
}

func TestSetState_LogsErrorBody(t *testing.T) {
	logs := captureLogs(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
		_, _ = w.Write([]byte(`{"message":"Service not found."}`))
	}))
	defer srv.Close()

	_ = newTestClient(srv.URL).SetState(context.Background(), testDevice(), model.HomeAssistantCommand{Service: "light.turn_on"})
	out := logs.String()
	if !strings.Contains(out, "Service not found.") || !strings.Contains(out, "status=400") {
		t.Fatalf("log missing body or status: %s", out)
	}
}

func TestSetState_BodyTruncated(t *testing.T) {
	captureLogs(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		_, _ = w.Write([]byte(strings.Repeat("x", 5000)))
	}))
	defer srv.Close()

	err := newTestClient(srv.URL).SetState(context.Background(), testDevice(), model.HomeAssistantCommand{Service: "light.turn_on"})
	var apiErr *model.HAAPIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected HAAPIError, got %v", err)
	}
	if len(apiErr.Body) > 2048 || len(apiErr.Body) == 0 {
		t.Fatalf("unexpected body length %d", len(apiErr.Body))
	}
}

func TestSetState_LogsSuccess(t *testing.T) {
	logs := captureLogs(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	if err := newTestClient(srv.URL).SetState(context.Background(), testDevice(), model.HomeAssistantCommand{Service: "light.turn_on"}); err != nil {
		t.Fatal(err)
	}
	out := logs.String()
	if !strings.Contains(out, "HA API Response") || !strings.Contains(out, "status=200") || !strings.Contains(out, "op=service_call") {
		t.Fatalf("log missing response line: %s", out)
	}
}

func TestSetState_TransportError(t *testing.T) {
	logs := captureLogs(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	url := srv.URL
	srv.Close()

	err := newTestClient(url).SetState(context.Background(), testDevice(), model.HomeAssistantCommand{Service: "light.turn_on"})
	if err == nil {
		t.Fatal("expected error")
	}
	out := logs.String()
	if !strings.Contains(out, "level=ERROR") || !strings.Contains(out, url+"/api/services/light/turn_on") {
		t.Fatalf("log missing transport error with URL: %s", out)
	}
}

func TestSetState_TokenNeverLogged(t *testing.T) {
	logs := captureLogs(t)
	var status atomic.Int32
	status.Store(200)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(int(status.Load()))
		_, _ = w.Write([]byte(`{"message":"nope"}`))
	}))
	c := newTestClient(srv.URL)
	cmd := model.HomeAssistantCommand{Service: "light.turn_on", Effect: "script.fx"}
	_ = c.SetState(context.Background(), testDevice(), cmd)
	status.Store(400)
	_ = c.SetState(context.Background(), testDevice(), cmd)
	_, _ = c.GetRawStates(context.Background())
	url := srv.URL
	srv.Close()
	_ = newTestClient(url).SetState(context.Background(), testDevice(), cmd)

	out := logs.String()
	if out == "" {
		t.Fatal("expected logs")
	}
	if strings.Contains(out, "test-token") || strings.Contains(strings.ToLower(out), "bearer") {
		t.Fatalf("token leaked in logs: %s", out)
	}
}

func TestSetState_LogsActualPayload(t *testing.T) {
	logs := captureLogs(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()

	cmd := model.HomeAssistantCommand{Service: "light.turn_on", Data: model.HAFields{"value": 0}}
	if err := newTestClient(srv.URL).SetState(context.Background(), testDevice(), cmd); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(logs.String(), `\"value\":0`) {
		t.Fatalf("payload not logged: %s", logs.String())
	}
}

func TestExecuteEffect_Logged(t *testing.T) {
	logs := captureLogs(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()

	cmd := model.HomeAssistantCommand{Service: "light.turn_on", Effect: "script.fx"}
	if err := newTestClient(srv.URL).SetState(context.Background(), testDevice(), cmd); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, line := range strings.Split(logs.String(), "\n") {
		if strings.Contains(line, "HA API Response") && strings.Contains(line, "op=effect") && strings.Contains(line, "/api/services/script/fx") {
			found = true
		}
	}
	if !found {
		t.Fatalf("effect response not logged: %s", logs.String())
	}
}

func TestGetRawStates_500ReturnsHAAPIError(t *testing.T) {
	logs := captureLogs(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		_, _ = w.Write([]byte("boom"))
	}))
	defer srv.Close()

	_, err := newTestClient(srv.URL).GetRawStates(context.Background())
	var apiErr *model.HAAPIError
	if !errors.As(err, &apiErr) || apiErr.Status != 500 || apiErr.Body != "boom" {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(logs.String(), "boom") {
		t.Fatalf("body not logged: %s", logs.String())
	}
}

func TestDo_RedactsURLPassword(t *testing.T) {
	logs := captureLogs(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"message":"nope"}`))
	}))
	defer srv.Close()
	c := newTestClient(strings.Replace(srv.URL, "http://", "http://user:s3cretpw@", 1))
	_ = c.SetState(context.Background(), testDevice(), model.HomeAssistantCommand{Service: "light.turn_on", Effect: "script.fx"})
	_, _ = c.GetRawStates(context.Background())
	url := c.url
	srv.Close()
	_ = newTestClient(url).SetState(context.Background(), testDevice(), model.HomeAssistantCommand{Service: "light.turn_on", Effect: "script.fx"})

	out := logs.String()
	if !strings.Contains(out, "HA API") {
		t.Fatalf("expected HA logs, got: %s", out)
	}
	if strings.Contains(out, "s3cretpw") {
		t.Fatalf("URL password leaked in logs: %s", out)
	}
}

func TestGetRawStates_SuccessIsNotLoggedAtInfo(t *testing.T) {
	logs := captureLogsAt(t, slog.LevelInfo)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	if _, err := newTestClient(srv.URL).GetRawStates(context.Background()); err != nil {
		t.Fatal(err)
	}
	if out := logs.String(); out != "" {
		t.Fatalf("state polling must stay out of the info log, got: %s", out)
	}
}

func TestGetRawStates_SuccessIsLoggedAtDebugAsStatesOperation(t *testing.T) {
	logs := captureLogsAt(t, slog.LevelDebug)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	if _, err := newTestClient(srv.URL).GetRawStates(context.Background()); err != nil {
		t.Fatal(err)
	}
	out := logs.String()
	if !strings.Contains(out, "HA API Response") || !strings.Contains(out, "op=states") {
		t.Fatalf("expected a debug line labelled op=states, got: %s", out)
	}
	if strings.Contains(out, "HA Service") {
		t.Fatalf("state fetch must not be labelled as a service call: %s", out)
	}
}

func TestGetRawStates_ErrorIsLabelledAsStatesOperation(t *testing.T) {
	logs := captureLogsAt(t, slog.LevelInfo)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		_, _ = w.Write([]byte("boom"))
	}))
	defer srv.Close()

	_, _ = newTestClient(srv.URL).GetRawStates(context.Background())
	out := logs.String()
	if !strings.Contains(out, "HA API Error") || !strings.Contains(out, "op=states") || !strings.Contains(out, "boom") {
		t.Fatalf("state fetch error must stay visible and labelled op=states, got: %s", out)
	}
	if strings.Contains(out, "HA Service") {
		t.Fatalf("state fetch must not be labelled as a service call: %s", out)
	}
}
