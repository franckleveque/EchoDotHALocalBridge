package http

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"hue-bridge-emulator/internal/domain/model"
	"hue-bridge-emulator/internal/ports"
)

type fakeHue struct{ updated []string }

func (f *fakeHue) GetDevices(ctx context.Context) ([]*model.Device, error) {
	return []*model.Device{{ID: "1", Name: "L", Type: model.MappingTypeLight, State: &model.DeviceState{On: true}}}, nil
}
func (f *fakeHue) GetDevice(ctx context.Context, id string) (*model.Device, error) {
	if id != "1" {
		return nil, fmt.Errorf("device %s not found", id)
	}
	return &model.Device{ID: "1", Name: "L", Type: model.MappingTypeLight, State: &model.DeviceState{}}, nil
}
func (f *fakeHue) GetDeviceMetadata(t model.MappingType) model.HueMetadata {
	return model.HueMetadata{Type: "T", ModelID: "M", ManufacturerName: "P"}
}
func (f *fakeHue) UpdateDeviceState(ctx context.Context, id string, s *model.DeviceState) error {
	f.updated = append(f.updated, id)
	return nil
}

// noCredsAuth simulates a bridge whose initial setup is not done yet.
type noCredsAuth struct{ stubAuth }

func (noCredsAuth) Exists() bool { return false }

type fakeAdmin struct{}

func (fakeAdmin) GetConfig(ctx context.Context) (*model.Config, error) {
	return &model.Config{HassURL: "u", HassToken: "t"}, nil
}
func (fakeAdmin) UpdateConfig(ctx context.Context, c *model.Config) error { return nil }
func (fakeAdmin) GetAllEntities(ctx context.Context) ([]ports.HomeAssistantEntity, error) {
	return nil, nil
}
func (fakeAdmin) TestDeviceAction(ctx context.Context, vd *model.VirtualDevice, s *model.DeviceState) error {
	return nil
}

// Characterization of the routing behaviour (statuses and body shape) that
// Alexa and the admin page depend on. It must stay green across routing refactors.
func TestRouting_Characterization(t *testing.T) {
	cases := []struct {
		method, path, body string
		status             int
		contains           string // "" means the body must be empty
	}{
		{"GET", "/api", "", 200, `"lights"`},
		{"GET", "/api/", "", 200, `"lights"`},
		{"POST", "/api", "{}", 200, `"username":"admin"`},
		{"POST", "/api/", "{}", 200, `"username":"admin"`},
		{"GET", "/api/u", "", 200, `"lights"`},
		{"GET", "/api/u/", "", 200, `"lights"`},
		{"GET", "/api/u/lights", "", 200, `"name":"L"`},
		{"GET", "/api/u/lights/", "", 200, `"name":"L"`},
		{"GET", "/api/u/lights/1", "", 200, `"name":"L"`},
		{"GET", "/api/u/lights/2", "", 404, "not found"},
		{"PUT", "/api/u/lights/1/state", `{"on":true}`, 200, "/lights/1/state/on"},
		{"GET", "/api/u/lights/1/state", "", 405, "Method not allowed"},
		{"GET", "/api/u/lights/1/other", "", 200, ""},
		{"GET", "/api/u/groups", "", 200, ""},
		{"GET", "/api/u/lights/1/state/x", "", 200, ""},
		{"GET", "/admin/setup", "", 200, "<form"},
		{"PUT", "/admin/setup", "", 405, "Method not allowed"},
		{"GET", "/admin/config", "", 200, `"hass_url"`},
		{"POST", "/admin/config", `{"hass_url":"x"}`, 200, ""},
		{"PUT", "/admin/config", "", 405, "Method not allowed"},
	}
	for _, c := range cases {
		var auth ports.AuthService = stubAuth{}
		if strings.HasPrefix(c.path, "/admin/setup") {
			auth = noCredsAuth{}
		}
		s := NewServer(&fakeHue{}, fakeAdmin{}, auth, "127.0.0.1")
		req := httptest.NewRequest(c.method, c.path, strings.NewReader(c.body))
		req.SetBasicAuth("a", "b")
		rec := httptest.NewRecorder()
		s.Mux().ServeHTTP(rec, req)
		body := rec.Body.String()
		if rec.Code != c.status || (c.contains == "" && body != "") || !strings.Contains(body, c.contains) {
			t.Errorf("%s %s: got %d %q, want %d containing %q", c.method, c.path, rec.Code, body, c.status, c.contains)
		}
	}
}

var _ = http.StatusOK
