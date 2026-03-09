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

func TestIntegrationScenario(t *testing.T) {
	ha := newFakeHA(t, []map[string]interface{}{
		{"entity_id": "light.bedroom", "state": "off", "attributes": map[string]interface{}{"friendly_name": "Bedroom"}},
	})
	ts := newTestStack(t, ha, nil)

	// Step 1: Initial setup
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	resp, err := client.Post(ts.URL+"/admin/setup", "application/x-www-form-urlencoded",
		strings.NewReader("username=admin&password=secret123"))
	assert.NoError(t, err)
	assert.Equal(t, http.StatusSeeOther, resp.StatusCode)

	// Step 2: Configure HA and Virtual Devices via Admin API
	cfg := &model.Config{
		HassURL:   ha.server.URL,
		HassToken: "test-token",
		VirtualDevices: []*model.VirtualDevice{
			{HueID: "1", Name: "Bedroom Lamp", EntityID: "light.bedroom", Type: model.MappingTypeLight},
		},
	}
	body, _ := json.Marshal(cfg)
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/admin/config", strings.NewReader(string(body)))
	// The helpers_test.go's random IP bypass might not be enough if we use the same client
	// but here we just need to make sure we use correct credentials
	req.SetBasicAuth("admin", "secret123")
	req.Header.Set("Content-Type", "application/json")
	resp, err = http.DefaultClient.Do(req)
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	// Step 3: Wait for background refresh from HA
	// (BridgeService.UpdateConfig starts a background refresh)
	// Actually BridgeService.UpdateConfig calls s.refreshInternal(ctx) which starts a goroutine.
	// Wait a bit to ensure the device list is populated.
	time.Sleep(100 * time.Millisecond)

	// Step 4: Verify device exists in Hue API
	resp, err = http.Get(ts.URL + "/api/admin/lights")
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
	var lights map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&lights)
	assert.Contains(t, lights, "1")

	// Step 5: Send Alexa command via Hue API
	req, _ = http.NewRequest(http.MethodPut, ts.URL+"/api/admin/lights/1/state",
		strings.NewReader(`{"on":true,"bri":128}`))
	resp, err = http.DefaultClient.Do(req)
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	// Step 6: Verify HA received the service call
	assert.Eventually(t, func() bool {
		return ha.callCount() > 0
	}, 1*time.Second, 50*time.Millisecond)

	call := ha.lastCall()
	assert.Equal(t, "light", call.Domain)
	assert.Equal(t, "turn_on", call.Service)
	assert.Equal(t, "light.bedroom", call.Payload["entity_id"])
	assert.Equal(t, float64(128), call.Payload["brightness"])
}
