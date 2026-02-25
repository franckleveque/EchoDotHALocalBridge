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
	"time"
)

type Client struct {
	url        string
	token      string
	httpClient *http.Client
	mu         sync.RWMutex

	cacheStates []map[string]interface{}
	cacheTime   time.Time
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
	if time.Since(c.cacheTime) < 2*time.Second {
		res := c.cacheStates
		c.mu.RUnlock()
		return res, nil
	}
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

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HA API error: %d", resp.StatusCode)
	}

	// Still using map[string]interface{} here because BridgeService needs various attributes
	// but we could filter it more if needed.
	var states []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&states); err != nil {
		return nil, err
	}

	c.mu.Lock()
	c.cacheStates = states
	c.cacheTime = time.Now()
	c.mu.Unlock()

	// Optional: strip very large attributes to save RAM
	for _, s := range states {
		if attr, ok := s["attributes"].(map[string]interface{}); ok {
			delete(attr, "entity_picture")
			delete(attr, "entity_picture_local")
			delete(attr, "source_list")
			delete(attr, "sound_mode_list")
		}
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
	service := params["service"].(string)

	serviceParts := strings.Split(service, ".")
	if len(serviceParts) == 2 {
		domain = serviceParts[0]
		service = serviceParts[1]
	}

	payload := make(map[string]interface{})
	for k, v := range params {
		if k == "effect" || k == "service" {
			continue
		}
		payload[k] = v
	}
	payload["entity_id"] = device.ExternalID

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
	if len(parts) < 2 {
		return
	}

	url := fmt.Sprintf("%s/api/services/%s/%s", urlBase, parts[0], parts[1])
	req, _ := http.NewRequestWithContext(ctx, "POST", url, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		fmt.Printf("Error executing effect %s: %v\n", effect, err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		fmt.Printf("Effect %s returned status %d\n", effect, resp.StatusCode)
	}
}

func (c *Client) isSupported(entityID string) bool {
	// Accept almost all domains that could be useful for Alexa interaction
	// including cameras, media players, and all input helpers.
	ignoredDomains := []string{"automation.", "zone.", "person.", "sun.", "weather.", "sensor.", "binary_sensor."}
	for _, domain := range ignoredDomains {
		if strings.HasPrefix(entityID, domain) {
			return false
		}
	}
	return strings.Contains(entityID, ".")
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
	// For other types, especially inputs and cameras, default to Custom to encourage formula/service config
	if strings.HasPrefix(entityID, "input_") || strings.HasPrefix(entityID, "camera.") || strings.HasPrefix(entityID, "media_player.") {
		return model.MappingTypeCustom
	}
	return model.MappingTypeLight // Default
}
