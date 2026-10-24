//go:build e2e

package e2e_test

import (
	"encoding/json"
	"hue-bridge-emulator/internal/domain/model"
	"io"
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

func adminGet(t *testing.T, url string, auth bool) *http.Response {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, url, nil)
	if auth {
		req.SetBasicAuth("admin", "password123")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { resp.Body.Close() })
	return resp
}

func setupAdmin(t *testing.T, base string) {
	t.Helper()
	resp, err := http.Post(base+"/admin/setup", "application/x-www-form-urlencoded",
		strings.NewReader("username=admin&password=password123"))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
}

func TestAdminLogs_RequiresAuth(t *testing.T) {
	ts := newTestStack(t, nil, nil)
	for _, p := range []string{"/admin/logs", "/admin/logs/data"} {
		assert.Equal(t, http.StatusForbidden, adminGet(t, ts.URL+p, false).StatusCode, p+" before setup")
	}
	setupAdmin(t, ts.URL)
	for _, p := range []string{"/admin/logs", "/admin/logs/data"} {
		assert.Equal(t, http.StatusUnauthorized, adminGet(t, ts.URL+p, false).StatusCode, p)
		assert.Equal(t, http.StatusOK, adminGet(t, ts.URL+p, true).StatusCode, p)
	}
}

func TestAdminLogs_EmptyIsJSONArray(t *testing.T) {
	ts := newTestStack(t, nil, nil)
	setupAdmin(t, ts.URL)
	resp := adminGet(t, ts.URL+"/admin/logs/data", true)
	b, _ := io.ReadAll(resp.Body)
	assert.Equal(t, "[]", strings.TrimSpace(string(b)))
}

func TestAdminLogs_ShowsFailedHACall(t *testing.T) {
	ha := newFakeHA(t, nil)
	ha.failStatus = http.StatusBadRequest
	ha.failBody = `{"message":"Service not found."}`
	ts := newTestStack(t, ha, nil)
	setupAdmin(t, ts.URL)

	payload := `{"virtual_device":{"hue_id":"1","name":"T","entity_id":"light.test","type":"light"},"state_update":{"on":true}}`
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/admin/test-action", strings.NewReader(payload))
	req.SetBasicAuth("admin", "password123")
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	assert.NoError(t, err)
	resp.Body.Close()
	assert.Equal(t, http.StatusBadGateway, resp.StatusCode)

	assert.Eventually(t, func() bool {
		r, err := http.NewRequest(http.MethodGet, ts.URL+"/admin/logs/data", nil)
		if err != nil {
			return false
		}
		r.SetBasicAuth("admin", "password123")
		resp, err := http.DefaultClient.Do(r)
		if err != nil {
			return false
		}
		defer resp.Body.Close()
		var lines []string
		if json.NewDecoder(resp.Body).Decode(&lines) != nil {
			return false
		}
		for _, l := range lines {
			if strings.Contains(l, "Service not found.") && strings.Contains(l, "status=400") {
				return true
			}
		}
		return false
	}, 3*time.Second, 50*time.Millisecond)
}

func postTestAction(t *testing.T, base, payload string) (int, string) {
	t.Helper()
	req, _ := http.NewRequest(http.MethodPost, base+"/admin/test-action", strings.NewReader(payload))
	req.SetBasicAuth("admin", "password123")
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b)
}

const testActionDevice = `"virtual_device":{"hue_id":"1","name":"T","entity_id":"light.x","type":"light"}`

func TestAdminTestAction_HAError502(t *testing.T) {
	ha := newFakeHA(t, nil)
	ha.failStatus = http.StatusBadRequest
	ha.failBody = `{"message":"Service not found."}`
	ts := newTestStack(t, ha, nil)
	setupAdmin(t, ts.URL)

	code, body := postTestAction(t, ts.URL, `{`+testActionDevice+`,"state_update":{"on":true}}`)
	assert.Equal(t, http.StatusBadGateway, code)
	assert.Contains(t, body, "400")
	assert.Contains(t, body, "Service not found.")
}

func TestAdminTestAction_Success200AfterHAAccepts(t *testing.T) {
	ha := newFakeHA(t, nil)
	ts := newTestStack(t, ha, nil)
	setupAdmin(t, ts.URL)

	code, _ := postTestAction(t, ts.URL, `{`+testActionDevice+`,"state_update":{"on":true}}`)
	assert.Equal(t, http.StatusOK, code)
	assert.Equal(t, 1, ha.callCount())
}

func TestAdminTestAction_DimSendsTurnOn(t *testing.T) {
	ha := newFakeHA(t, nil)
	ts := newTestStack(t, ha, nil)
	setupAdmin(t, ts.URL)

	code, _ := postTestAction(t, ts.URL, `{`+testActionDevice+`,"state_update":{"bri":127}}`)
	assert.Equal(t, http.StatusOK, code)
	call := ha.lastCall()
	assert.Equal(t, "light", call.Domain)
	assert.Equal(t, "turn_on", call.Service)
	assert.EqualValues(t, 127, call.Payload["brightness"])
}

// The entity chosen in the form must be the one Home Assistant receives, whatever
// the mapping type (bug report: light.* replaced by a switch.* entity).
func TestAdminTestAction_ReachesSwitchEntityForEveryType(t *testing.T) {
	const sw = "switch.intlumieresalleamanger"
	cases := map[string]string{
		"light": `{"virtual_device":{"name":"T","entity_id":"` + sw + `","type":"light"},"state_update":{"on":true}}`,
		"custom": `{"virtual_device":{"name":"T","entity_id":"` + sw + `","type":"custom","action_config":` +
			`{"on_service":"switch.turn_on","on_payload":{"entity_id":"` + sw + `"}}},"state_update":{"on":true}}`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			ha := newFakeHA(t, nil)
			ts := newTestStack(t, ha, nil)
			setupAdmin(t, ts.URL)

			code, _ := postTestAction(t, ts.URL, body)
			assert.Equal(t, http.StatusOK, code)
			assert.Equal(t, 1, ha.callCount())
			call := ha.lastCall()
			assert.Equal(t, "switch", call.Domain)
			assert.Equal(t, "turn_on", call.Service)
			assert.Equal(t, sw, call.Payload["entity_id"])
		})
	}
}

func TestAdminTestAction_BadBody400(t *testing.T) {
	ha := newFakeHA(t, nil)
	ts := newTestStack(t, ha, nil)
	setupAdmin(t, ts.URL)

	code, _ := postTestAction(t, ts.URL, `{"state_update":{"on":true}}`)
	assert.Equal(t, http.StatusBadRequest, code)
	assert.Equal(t, 0, ha.callCount())
}

// The setup rate limiter is keyed by client IP, so the test stack must hand every
// request its own address; a random pick from a small pool collides and flakes.
func TestTestStack_RequestsNeverHitSetupRateLimiter(t *testing.T) {
	ts := newTestStack(t, nil, nil)

	for i := 0; i < 600; i++ {
		resp, err := http.Get(ts.URL + "/admin/setup")
		assert.NoError(t, err)
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("request %d got %d, want 200 (limiter hit by a repeated client IP)", i, resp.StatusCode)
		}
	}
}
