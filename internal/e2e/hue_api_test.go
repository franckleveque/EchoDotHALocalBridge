//go:build e2e

package e2e_test

import (
	"encoding/json"
	"hue-bridge-emulator/internal/domain/model"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestHueRegister(t *testing.T) {
	ha := newFakeHA(t, nil)
	ts := newTestStack(t, ha, nil)

	resp, err := http.Post(ts.URL+"/api", "application/json",
		strings.NewReader(`{"devicetype":"test"}`))
	assert.NoError(t, err)

	assert.Equal(t, 200, resp.StatusCode)
	var result []map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&result)
	assert.NoError(t, err)
	assert.Equal(t, "admin", result[0]["success"].(map[string]interface{})["username"])
}

func TestHueGetLightsAndSetState(t *testing.T) {
	ha := newFakeHA(t, []map[string]interface{}{
		{
			"entity_id": "light.living_room",
			"state":     "off",
			"attributes": map[string]interface{}{
				"brightness":    0.0,
				"friendly_name": "Living Room",
			},
		},
	})
	cfg := &model.Config{
		HassURL:   ha.server.URL,
		HassToken: "test-token",
		VirtualDevices: []*model.VirtualDevice{
			{
				HueID:    "1",
				Name:     "Living Room",
				EntityID: "light.living_room",
				Type:     model.MappingTypeLight,
			},
		},
	}
	ts := newTestStack(t, ha, cfg)

	// GET /api/admin/lights
	resp, err := http.Get(ts.URL + "/api/admin/lights")
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
	var lights map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&lights)
	assert.NoError(t, err)
	assert.Contains(t, lights, "1")

	// PUT /api/admin/lights/1/state  → should call HA turn_on
	req, err := http.NewRequest(http.MethodPut, ts.URL+"/api/admin/lights/1/state",
		strings.NewReader(`{"on":true,"bri":200}`))
	assert.NoError(t, err)
	_, err = http.DefaultClient.Do(req)
	assert.NoError(t, err)

	// UpdateDeviceState in BridgeService is async (uses worker pool)
	assert.Eventually(t, func() bool {
		return ha.callCount() > 0
	}, 1*time.Second, 50*time.Millisecond)

	call := ha.lastCall()
	assert.Equal(t, "light", call.Domain)
	assert.Equal(t, "turn_on", call.Service)
	assert.Equal(t, "light.living_room", call.Payload["entity_id"])
	assert.Equal(t, float64(200), call.Payload["brightness"])
}
