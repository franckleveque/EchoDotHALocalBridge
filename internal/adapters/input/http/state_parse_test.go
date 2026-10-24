package http

import (
	"context"
	"errors"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"hue-bridge-emulator/internal/domain/model"
	"hue-bridge-emulator/internal/ports"
)

func TestParseStateUpdate(t *testing.T) {
	if s := parseStateUpdate(map[string]any{"on": true}); !s.On || s.UpdatedByBri {
		t.Errorf("on: got %+v", s)
	}
	if s := parseStateUpdate(map[string]any{"bri": float64(127)}); s.Bri != 127 || !s.UpdatedByBri {
		t.Errorf("bri: got %+v", s)
	}
	if s := parseStateUpdate(map[string]any{}); !reflect.DeepEqual(*s, model.DeviceState{}) {
		t.Errorf("empty: got %+v", s)
	}
}

type stubAdmin struct {
	ports.AdminPort
	err error
}

func (s stubAdmin) TestDeviceAction(ctx context.Context, vd *model.VirtualDevice, st *model.DeviceState) error {
	return s.err
}

func TestHandleAdminTestAction_StatusMapping(t *testing.T) {
	body := `{"virtual_device":{"entity_id":"light.x"},"state_update":{"on":true}}`
	cases := []struct {
		name string
		err  error
		body string
		want int
	}{
		{"ok", nil, body, 200},
		{"ha error", &model.HAAPIError{Status: 400, Body: "nope"}, body, 502},
		{"network error", errors.New("dial tcp: refused"), body, 502},
		{"too many", model.ErrTooManyRequests, body, 429},
		{"invalid", model.ErrInvalidTestRequest, body, 400},
		{"missing device", nil, `{"state_update":{"on":true}}`, 400},
		{"missing state", nil, `{"virtual_device":{}}`, 400},
		{"bad json", nil, `{`, 400},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := NewServer(nil, stubAdmin{err: c.err}, stubAuth{}, "127.0.0.1")
			rec := httptest.NewRecorder()
			s.handleAdminTestAction(rec, httptest.NewRequest("POST", "/admin/test-action", strings.NewReader(c.body)))
			if rec.Code != c.want {
				t.Errorf("got %d want %d (%s)", rec.Code, c.want, rec.Body.String())
			}
			if c.want == 502 && !strings.Contains(rec.Body.String(), c.err.Error()) {
				t.Errorf("body %q lacks error text", rec.Body.String())
			}
		})
	}
}
