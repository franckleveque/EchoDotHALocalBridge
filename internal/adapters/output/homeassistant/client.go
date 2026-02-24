package homeassistant

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"hue-bridge-emulator/internal/domain/model"
	"hue-bridge-emulator/internal/domain/translator"
	"net/http"
	"strings"
	"sync"
)

type Client struct {
	url        string
	token      string
	httpClient *http.Client
	factory    *translator.Factory
	mu         sync.RWMutex
}

type hassState struct {
	EntityID   string                 `json:"entity_id"`
	State      string                 `json:"state"`
	Attributes map[string]interface{} `json:"attributes"`
}

func NewClient() *Client {
	return &Client{
		httpClient: &http.Client{},
		factory:    translator.NewFactory(),
	}
}

func (c *Client) Configure(url, token string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.url = strings.TrimSuffix(url, "/")
	c.token = token
}

func (c *Client) IsConfigured() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.url != "" && c.token != ""
}

func (c *Client) GetDevices(ctx context.Context) ([]*model.Device, error) {
	c.mu.RLock()
	url := c.url
	token := c.token
	c.mu.RUnlock()

	if url == "" || token == "" {
		return nil, fmt.Errorf("Home Assistant not configured")
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url+"/api/states", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HA API error: %d", resp.StatusCode)
	}

	var states []hassState
	if err := json.NewDecoder(resp.Body).Decode(&states); err != nil {
		return nil, err
	}

	var devices []*model.Device
	for _, s := range states {
		devType, supported := c.mapDomainToType(s.EntityID)
		if !supported {
			continue
		}

		t := c.factory.GetTranslator(devType)
		hueState := t.ToHue(map[string]interface{}{
			"state":      s.State,
			"attributes": s.Attributes,
		})

		name := s.Attributes["friendly_name"]
		if name == nil {
			name = s.EntityID
		}

		devices = append(devices, &model.Device{
			ID:         c.entityIDToHueID(s.EntityID),
			Name:       name.(string),
			Type:       devType,
			ExternalID: s.EntityID,
			State:      hueState,
		})
	}

	return devices, nil
}

func (c *Client) SetState(ctx context.Context, device *model.Device, params map[string]interface{}) error {
	c.mu.RLock()
	url_base := c.url
	token := c.token
	c.mu.RUnlock()

	if url_base == "" || token == "" {
		return fmt.Errorf("Home Assistant not configured")
	}

	domain := strings.Split(device.ExternalID, ".")[0]
	service := "turn_on"

	payload := make(map[string]interface{})
	for k, v := range params {
		if k == "on" {
			if !v.(bool) {
				service = "turn_off"
			}
			continue
		}
		payload[k] = v
	}
	payload["entity_id"] = device.ExternalID

	if device.Type == model.DeviceTypeCover {
		service = "set_cover_position"
	} else if device.Type == model.DeviceTypeClimate {
		service = "set_temperature"
	}

	url := fmt.Sprintf("%s/api/services/%s/%s", url_base, domain, service)
	body, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("HA API error: %d", resp.StatusCode)
	}

	return nil
}

func (c *Client) mapDomainToType(entityID string) (model.DeviceType, bool) {
	if strings.HasPrefix(entityID, "light.") {
		return model.DeviceTypeLight, true
	}
	if strings.HasPrefix(entityID, "cover.") {
		return model.DeviceTypeCover, true
	}
	if strings.HasPrefix(entityID, "climate.") {
		return model.DeviceTypeClimate, true
	}
	return "", false
}

func (c *Client) entityIDToHueID(entityID string) string {
	h := 0
	for _, b := range entityID {
		h = 31*h + int(b)
	}
	if h < 0 {
		h = -h
	}
	return fmt.Sprintf("%d", (h%1000)+1)
}
