package homeassistant

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"hue-bridge-emulator/internal/domain/model"
	"log/slog"
	"net/http"
	"os"
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


func (c *Client) GetRawStates(ctx context.Context) ([]model.HAEntityState, error) {
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

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HA API error: %d", resp.StatusCode)
	}

	type haState struct {
		EntityID   string         `json:"entity_id"`
		State      string         `json:"state"`
		Attributes model.HAFields `json:"attributes"`
	}

	var states []haState
	if err := json.NewDecoder(resp.Body).Decode(&states); err != nil {
		return nil, err
	}

	res := make([]model.HAEntityState, len(states))
	for i, v := range states {
		res[i] = model.HAEntityState{
			EntityID:   v.EntityID,
			State:      v.State,
			Attributes: v.Attributes,
		}
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

	payload := make(map[string]any)
	for k, v := range cmd.Data {
		payload[k] = v
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
	if cmd.Effect != "" {
		c.executeEffect(ctx, cmd.Effect, urlBase, token)
	}

	return nil
}

func (c *Client) executeEffect(ctx context.Context, effect, urlBase, token string) {
	parts := strings.Split(effect, ".")
	if len(parts) < 2 {
		return
	}

	url := fmt.Sprintf("%s/api/services/%s/%s", urlBase, parts[0], parts[1])
	req, err := http.NewRequestWithContext(ctx, "POST", url, nil)
	if err != nil {
		slog.Error("Failed to create request for effect", "error", err, "url", url)
		return
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		slog.Error("Failed to execute effect", "error", err, "url", url)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		slog.Warn("Effect call returned error", "status", resp.StatusCode, "url", url)
	}
}

