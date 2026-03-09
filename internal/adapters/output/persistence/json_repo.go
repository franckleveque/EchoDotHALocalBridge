package persistence

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"hue-bridge-emulator/internal/domain/model"
	"io"
	"os"
	"sync"
)

type JSONConfigRepository struct {
	filepath string
	mu       sync.RWMutex
	cache    *model.Config
	key      []byte
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
	// Static key for token encryption. Can be overridden via HUE_ENCRYPTION_KEY env var.
	key := []byte("a-very-secret-key-32-chars-long!")
	if envKey := os.Getenv("HUE_ENCRYPTION_KEY"); len(envKey) >= 16 {
		// Use the first 16, 24, or 32 chars for the key
		if len(envKey) >= 32 {
			key = []byte(envKey[:32])
		} else if len(envKey) >= 24 {
			key = []byte(envKey[:24])
		} else {
			key = []byte(envKey[:16])
		}
	}
	return &JSONConfigRepository{filepath: filepath, key: key}
}

func (r *JSONConfigRepository) Get(ctx context.Context) (*model.Config, error) {
	r.mu.RLock()
	if r.cache != nil {
		defer r.mu.RUnlock()
		return r.cache, nil
	}
	r.mu.RUnlock()

	r.mu.Lock()
	defer r.mu.Unlock()

	// Double check cache after acquiring write lock
	if r.cache != nil {
		return r.cache, nil
	}

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
		migrated, err := r.migrate(data)
		if err == nil {
			if migrated.HassToken != "" {
				decrypted, err := r.decrypt(migrated.HassToken)
				if err == nil {
					migrated.HassToken = decrypted
				}
			}
			r.cache = migrated
		}
		return migrated, err
	}

	if cfg.HassToken != "" {
		decrypted, err := r.decrypt(cfg.HassToken)
		if err == nil {
			cfg.HassToken = decrypted
		} else {
			// If decryption fails, it could be plaintext (e.g. first run after update)
			// Check if it's base64 encoded, if not, it's definitely plaintext or corrupted
			if _, decodeErr := base64.StdEncoding.DecodeString(cfg.HassToken); decodeErr != nil {
				// Not base64, likely plaintext
			} else {
				// It was base64 but decryption failed - possibly wrong key
				return nil, fmt.Errorf("failed to decrypt HA token: %w", err)
			}
		}
	}

	r.cache = &cfg
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

	// Clone config to encrypt token for storage without affecting memory state
	storageConfig := *config
	if config.VirtualDevices != nil {
		storageConfig.VirtualDevices = make([]*model.VirtualDevice, len(config.VirtualDevices))
		for i, vd := range config.VirtualDevices {
			vdCopy := *vd
			storageConfig.VirtualDevices[i] = &vdCopy
		}
	}

	if storageConfig.HassToken != "" {
		encrypted, err := r.encrypt(storageConfig.HassToken)
		if err != nil {
			return err
		}
		storageConfig.HassToken = encrypted
	}

	data, err := json.MarshalIndent(storageConfig, "", "  ")
	if err != nil {
		return err
	}

	if err := os.WriteFile(r.filepath, data, 0600); err != nil {
		return err
	}

	r.cache = config
	return nil
}

func (r *JSONConfigRepository) encrypt(plaintext string) (string, error) {
	block, err := aes.NewCipher(r.key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

func (r *JSONConfigRepository) decrypt(cryptoText string) (string, error) {
	ciphertext, err := base64.StdEncoding.DecodeString(cryptoText)
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(r.key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}
