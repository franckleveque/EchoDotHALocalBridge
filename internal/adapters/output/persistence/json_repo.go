package persistence

import (
	"context"
	"encoding/json"
	"hue-bridge-emulator/internal/domain/model"
	"os"
	"sync"
)

type JSONConfigRepository struct {
	filepath string
	mu       sync.RWMutex
}

// Internal structure for migration
type legacyConfig struct {
	HassURL        string                           `json:"hass_url"`
	HassToken      string                           `json:"hass_token"`
	EntityMappings map[string]*legacyEntityMapping `json:"entity_mappings"`
}

type legacyEntityMapping struct {
	EntityID      string               `json:"entity_id"`
	HueID         string               `json:"hue_id"`
	Name          string               `json:"name"`
	Type          model.MappingType    `json:"type"`
	Exposed       bool                 `json:"exposed"`
	CustomFormula *legacyCustomFormula `json:"custom_formula,omitempty"`
}

type legacyCustomFormula struct {
	ToHueFormula string `json:"to_hue_formula"`
	ToHAFormula  string `json:"to_ha_formula"`
	OnService    string `json:"on_service"`
	OffService   string `json:"off_service"`
	OnEffect     string `json:"on_effect"`
	OffEffect    string `json:"off_effect"`
}

func NewJSONConfigRepository(filepath string) *JSONConfigRepository {
	return &JSONConfigRepository{filepath: filepath}
}

func (r *JSONConfigRepository) Get(ctx context.Context) (*model.Config, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	data, err := os.ReadFile(r.filepath)
	if err != nil {
		if os.IsNotExist(err) {
			return &model.Config{VirtualDevices: []*model.VirtualDevice{}}, nil
		}
		return nil, err
	}

	// Try to decode into new structure
	var cfg model.Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	// Migration check: if virtual_devices is empty but there's a file, check for old format
	if len(cfg.VirtualDevices) == 0 {
		return r.migrate(data)
	}

	return &cfg, nil
}

func (r *JSONConfigRepository) migrate(data []byte) (*model.Config, error) {
	var legacy legacyConfig
	if err := json.Unmarshal(data, &legacy); err != nil {
		return &model.Config{VirtualDevices: []*model.VirtualDevice{}}, nil
	}

	if len(legacy.EntityMappings) == 0 {
		return &model.Config{
			HassURL:        legacy.HassURL,
			HassToken:      legacy.HassToken,
			VirtualDevices: []*model.VirtualDevice{},
		}, nil
	}

	cfg := &model.Config{
		HassURL:        legacy.HassURL,
		HassToken:      legacy.HassToken,
		VirtualDevices: make([]*model.VirtualDevice, 0),
	}

	for _, m := range legacy.EntityMappings {
		if !m.Exposed {
			continue
		}
		vd := &model.VirtualDevice{
			HueID:    m.HueID,
			Name:     m.Name,
			EntityID: m.EntityID,
			Type:     m.Type,
		}
		if m.CustomFormula != nil {
			vd.ActionConfig = &model.ActionConfig{
				ToHueFormula: m.CustomFormula.ToHueFormula,
				ToHAFormula:  m.CustomFormula.ToHAFormula,
				OnService:    m.CustomFormula.OnService,
				OffService:   m.CustomFormula.OffService,
				OnEffect:     m.CustomFormula.OnEffect,
				OffEffect:    m.CustomFormula.OffEffect,
			}
		}
		cfg.VirtualDevices = append(cfg.VirtualDevices, vd)
	}

	return cfg, nil
}

func (r *JSONConfigRepository) Save(ctx context.Context, config *model.Config) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(r.filepath, data, 0644)
}
