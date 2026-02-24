package homeassistant

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"hue-bridge-emulator/internal/domain/model"
	"net/http"
	"strings"
	"sync"
)

type Client struct {
	url        string
	token      string
	httpClient *http.Client
	mu         sync.RWMutex
}

func NewClient() *Client {
	return &Client{
		httpClient: &http.Client{},
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

func (c *Client) GetAllEntities(ctx context.Context) ([]*model.EntityMapping, error) {
	states, err := c.GetRawStates(ctx)
	if err != nil {
		return nil, err
	}

	var entities []*model.EntityMapping
	for _, s := range states {
		entityID, _ := s["entity_id"].(string)
		if !c.isSupported(entityID) {
			continue
		}

		attributes, _ := s["attributes"].(map[string]interface{})
		name, _ := attributes["friendly_name"].(string)
		if name == "" {
			name = entityID
		}

		entities = append(entities, &model.EntityMapping{
			EntityID: entityID,
			Name:     name,
			Type:     c.guessType(entityID),
			Exposed:  false,
		})
	}

	return entities, nil
}

func (c *Client) GetRawStates(ctx context.Context) ([]map[string]interface{}, error) {
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

	var states []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&states); err != nil {
		return nil, err
	}

	return states, nil
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

	if on, ok := params["on"].(bool); ok && !on {
		service = "turn_off"
	}

	payload := make(map[string]interface{})
	for k, v := range params {
		if k == "on" { continue }
		payload[k] = v
	}
	payload["entity_id"] = device.ExternalID

	if device.Type == model.MappingTypeCover {
		if _, ok := payload["position"]; ok {
			service = "set_cover_position"
		}
	} else if device.Type == model.MappingTypeClimate {
		if _, ok := payload["temperature"]; ok {
			service = "set_temperature"
		}
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

	// Handle custom effects if provided
	if effect, ok := params["effect"].(string); ok && effect != "" {
		c.executeEffect(ctx, effect, url_base, token)
	}

	return nil
}

func (c *Client) executeEffect(ctx context.Context, effect, urlBase, token string) {
	// effect is like "service.call" or just a service name if domain is known
	parts := strings.Split(effect, ".")
	if len(parts) < 2 { return }

	url := fmt.Sprintf("%s/api/services/%s/%s", urlBase, parts[0], parts[1])
	req, _ := http.NewRequestWithContext(ctx, "POST", url, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := c.httpClient.Do(req)
	if err == nil {
		resp.Body.Close()
	}
}

func (c *Client) isSupported(entityID string) bool {
	prefixes := []string{"light.", "switch.", "input_number.", "cover.", "climate.", "group."}
	for _, p := range prefixes {
		if strings.HasPrefix(entityID, p) {
			return true
		}
	}
	return false
}

func (c *Client) guessType(entityID string) model.MappingType {
	if strings.HasPrefix(entityID, "light.") {
		return model.MappingTypeLight
	}
	if strings.HasPrefix(entityID, "cover.") {
		return model.MappingTypeCover
	}
	if strings.HasPrefix(entityID, "climate.") {
		return model.MappingTypeClimate
	}
	return model.MappingTypeLight // Default
}
