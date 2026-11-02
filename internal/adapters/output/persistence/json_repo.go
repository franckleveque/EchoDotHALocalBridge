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
	"log/slog"
	"os"
	"sync"
)

type JSONConfigRepository struct {
	filepath   string
	mu         sync.RWMutex
	cache      *model.Config
	key        []byte
	defaultKey []byte
	logger     *slog.Logger
}

// aesKeySizes lists the valid AES key sizes, largest first.
var aesKeySizes = []int{32, 24, 16}

// selectKeySize returns the largest AES key size that fits in n bytes, or 0.
func selectKeySize(n int) int {
	for _, size := range aesKeySizes {
		if size <= n {
			return size
		}
	}
	return 0
}

func NewJSONConfigRepository(filepath string, logger *slog.Logger) *JSONConfigRepository {
	// Static key for token encryption. Can be overridden via HUE_ENCRYPTION_KEY env var.
	defaultKey := []byte("a-very-secret-key-32-chars-long!")
	key := defaultKey
	if envKey := os.Getenv("HUE_ENCRYPTION_KEY"); envKey != "" {
		size := selectKeySize(len(envKey))
		if size == 0 {
			logger.Warn("HUE_ENCRYPTION_KEY is too short (min 16 chars). Using default key.")
		}
		if size > 0 {
			key = []byte(envKey[:size])
		}
		if size > 0 && len(envKey) > size {
			logger.Warn("HUE_ENCRYPTION_KEY is longer than the AES key size and will be truncated.", "key_size", size)
		}
	}
	return &JSONConfigRepository{filepath: filepath, key: key, defaultKey: defaultKey, logger: logger}
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
			r.logger.Info("Config: migrated legacy format", "path", r.filepath, "devices", len(migrated.VirtualDevices))
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
				r.logger.Warn("Config: HA token is not encrypted, using it as plaintext", "path", r.filepath)
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
		r.logger.Error("Config: write failed", "path", r.filepath, "error", err)
		return err
	}
	r.logger.Debug("Config: saved", "path", r.filepath)

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

	plaintext, err := r.decryptWithKey(ciphertext, r.key)
	if err == nil {
		return string(plaintext), nil
	}

	// Try default key as fallback
	if len(r.key) != len(r.defaultKey) || string(r.key) != string(r.defaultKey) {
		plaintext, err = r.decryptWithKey(ciphertext, r.defaultKey)
		if err == nil {
			return string(plaintext), nil
		}
	}

	return "", fmt.Errorf("decryption failed")
}

func (r *JSONConfigRepository) decryptWithKey(ciphertext []byte, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}

	nonce, actualCiphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	return gcm.Open(nil, nonce, actualCiphertext, nil)
}
