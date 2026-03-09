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

func TestAdminSetup(t *testing.T) {
	ts := newTestStack(t, nil, nil)

	// Check if setup is needed
	resp, err := http.Get(ts.URL + "/")
	assert.NoError(t, err)
	// should redirect to /admin/setup
	assert.Contains(t, resp.Request.URL.Path, "/admin/setup")

	// Post credentials to setup
	// We use a client that doesn't follow redirects to check the 303 status
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	resp, err = client.Post(ts.URL+"/admin/setup", "application/x-www-form-urlencoded",
		strings.NewReader("username=admin&password=password123"))
	assert.NoError(t, err)
	assert.Equal(t, http.StatusSeeOther, resp.StatusCode)

	// After setup, / should redirect to /admin
	resp, err = http.Get(ts.URL + "/")
	assert.NoError(t, err)
	assert.Contains(t, resp.Request.URL.Path, "/admin")
	// And we should get 401 Unauthorized because it needs Basic Auth now
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestAdminConfig(t *testing.T) {
	ts := newTestStack(t, nil, nil)

	// Setup first
	http.Post(ts.URL+"/admin/setup", "application/x-www-form-urlencoded",
		strings.NewReader("username=admin&password=password123"))

	// Get config
	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/admin/config", nil)
	req.SetBasicAuth("admin", "password123")
	resp, err := http.DefaultClient.Do(req)
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	var cfg model.Config
	err = json.NewDecoder(resp.Body).Decode(&cfg)
	assert.NoError(t, err)

	// Update config
	newCfg := &model.Config{
		HassURL:   "http://localhost:8123",
		HassToken: "some-token",
		VirtualDevices: []*model.VirtualDevice{
			{HueID: "1", Name: "Test", EntityID: "light.test", Type: model.MappingTypeLight},
		},
	}
	body, _ := json.Marshal(newCfg)
	req, _ = http.NewRequest(http.MethodPost, ts.URL+"/admin/config", strings.NewReader(string(body)))
	req.SetBasicAuth("admin", "password123")
	req.Header.Set("Content-Type", "application/json")
	resp, err = http.DefaultClient.Do(req)
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	// Verify update
	time.Sleep(100 * time.Millisecond) // Wait for Save goroutine if any? (Persistence Save is sync, but Bridge UpdateConfig is somewhat async)
	req, _ = http.NewRequest(http.MethodGet, ts.URL+"/admin/config", nil)
	// We need to use a client that provides basic auth
	req.SetBasicAuth("admin", "password123")
	resp, err = http.DefaultClient.Do(req)
	assert.NoError(t, err)
	json.NewDecoder(resp.Body).Decode(&cfg)
	assert.Equal(t, "http://localhost:8123", cfg.HassURL)
	assert.Equal(t, 1, len(cfg.VirtualDevices))
}

func TestAdminHAEntities(t *testing.T) {
	ha := newFakeHA(t, []map[string]interface{}{
		{"entity_id": "light.living_room", "state": "off", "attributes": map[string]interface{}{"friendly_name": "Living Room"}},
		{"entity_id": "switch.kitchen", "state": "on", "attributes": map[string]interface{}{"friendly_name": "Kitchen"}},
	})
	cfg := &model.Config{
		HassURL:   ha.server.URL,
		HassToken: "test-token",
	}
	ts := newTestStack(t, ha, cfg)

	// Setup
	http.Post(ts.URL+"/admin/setup", "application/x-www-form-urlencoded",
		strings.NewReader("username=admin&password=password123"))

	// GET /admin/ha-entities
	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/admin/ha-entities", nil)
	req.SetBasicAuth("admin", "password123")
	resp, err := http.DefaultClient.Do(req)
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	var entities []map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&entities)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(entities))
}
