package persistence

import (
	"context"
	"encoding/json"
	"hue-bridge-emulator/internal/domain/model"
	"os"
	"sync"
)

type JSONAuthRepository struct {
	filepath string
	mu       sync.RWMutex
	cache    *model.AuthConfig
}

func NewJSONAuthRepository(filepath string) *JSONAuthRepository {
	return &JSONAuthRepository{filepath: filepath}
}

func (r *JSONAuthRepository) Get(ctx context.Context) (*model.AuthConfig, error) {
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
		return nil, err
	}

	var auth model.AuthConfig
	if err := json.Unmarshal(data, &auth); err != nil {
		return nil, err
	}

	r.cache = &auth
	return &auth, nil
}

func (r *JSONAuthRepository) Save(ctx context.Context, auth *model.AuthConfig) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	data, err := json.MarshalIndent(auth, "", "  ")
	if err != nil {
		return err
	}

	if err := os.WriteFile(r.filepath, data, 0600); err != nil {
		return err
	}

	r.cache = auth
	return nil
}

func (r *JSONAuthRepository) Exists() bool {
	_, err := os.Stat(r.filepath)
	return err == nil
}
