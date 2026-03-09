package homeassistant

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"hue-bridge-emulator/internal/domain/model"
	"hue-bridge-emulator/internal/ports"
	"log/slog"
	"net/http"
	"os"
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

func (c *Client) GetAllEntities(ctx context.Context) ([]ports.HomeAssistantEntity, error) {
	states, err := c.GetRawStates(ctx)
	if err != nil {
		return nil, err
	}

	var entities []ports.HomeAssistantEntity
	for _, item := range states {
		s, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		entityID, _ := s["entity_id"].(string)
		if !c.isSupported(entityID) {
			continue
		}

		name, _ := s["name"].(string)
		attributes, _ := s["attributes"].(map[string]interface{})
		if name == "" && attributes != nil {
			name, _ = attributes["friendly_name"].(string)
			if name == "" {
				name, _ = attributes["name"].(string)
			}
		}
		if name == "" {
			name = entityID
		}

		entities = append(entities, ports.HomeAssistantEntity{
			EntityID:     entityID,
			FriendlyName: name,
		})
	}

	return entities, nil
}

func (c *Client) GetRawStates(ctx context.Context) ([]interface{}, error) {
	c.mu.RLock()
	if time.Since(c.cacheTime) < 2*time.Second {
		res := make([]interface{}, len(c.cacheStates))
		for i, v := range c.cacheStates {
			res[i] = v
		}
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

	var states []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&states); err != nil {
		return nil, err
	}

	// Optimization: strip large attributes to save RAM
	for _, s := range states {
		if attr, ok := s["attributes"].(map[string]interface{}); ok {
			delete(attr, "entity_picture")
			delete(attr, "entity_picture_local")
			delete(attr, "source_list")
			delete(attr, "sound_mode_list")
		}
	}

	c.mu.Lock()
	c.cacheStates = states
	c.cacheTime = time.Now()
	c.mu.Unlock()

	res := make([]interface{}, len(states))
	for i, v := range states {
		res[i] = v
	}
	return res, nil
}


func (c *Client) SetState(ctx context.Context, device *model.Device, cmd model.HomeAssistantCommand) error {
	c.mu.RLock()
	urlBase := c.url
	token := c.token
	c.mu.RUnlock()

	if urlBase == "" || token == "" {
		return fmt.Errorf("Home Assistant not configured")
	}

	domain := strings.Split(device.ExternalID, ".")[0]
	serviceParts := strings.Split(cmd.Service, ".")
	service := cmd.Service
	if len(serviceParts) == 2 {
		domain = serviceParts[0]
		service = serviceParts[1]
	}

	payload := make(map[string]interface{})
	if cmd.Data != nil {
		if dm, ok := cmd.Data.(map[string]interface{}); ok {
			for k, v := range dm {
				if k == "effect" {
					continue
				}
				payload[k] = v
			}
		}
	}

	// Handle OmitEntityID
	omit := false
	if device.VirtualDevice != nil && device.VirtualDevice.ActionConfig != nil {
		omit = device.VirtualDevice.ActionConfig.OmitEntityID
	}
	if !omit {
		payload["entity_id"] = device.ExternalID
	}

	url := fmt.Sprintf("%s/api/services/%s/%s", urlBase, domain, service)
	body, _ := json.Marshal(payload)

	slog.Info("HA Service Call", "pid", os.Getpid(), "url", url, "payload", string(body))

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
	if cmd.Data != nil {
		if dm, ok := cmd.Data.(map[string]interface{}); ok {
			if effect, ok := dm["effect"].(string); ok && effect != "" {
				c.executeEffect(ctx, effect, urlBase, token)
			}
		}
	}

	return nil
}

func (c *Client) executeEffect(ctx context.Context, effect, urlBase, token string) {
	parts := strings.Split(effect, ".")
	if len(parts) < 2 {
		return
	}

	url := fmt.Sprintf("%s/api/services/%s/%s", urlBase, parts[0], parts[1])
	req, _ := http.NewRequestWithContext(ctx, "POST", url, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
}

func (c *Client) isSupported(entityID string) bool {
	ignoredDomains := []string{"zone.", "sun.", "weather."}
	for _, domain := range ignoredDomains {
		if strings.HasPrefix(entityID, domain) {
			return false
		}
	}
	return strings.Contains(entityID, ".")
}
