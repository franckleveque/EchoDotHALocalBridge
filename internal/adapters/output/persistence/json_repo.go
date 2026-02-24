package persistence

import (
	"context"
	"encoding/json"
	"hue-bridge-emulator/internal/domain/model"
	"os"
	"sync"
)

type JSONConfigRepository struct {
	path string
	mu   sync.RWMutex
}

func NewJSONConfigRepository(path string) *JSONConfigRepository {
	return &JSONConfigRepository{path: path}
}

func (r *JSONConfigRepository) Get(ctx context.Context) (*model.Config, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	data, err := os.ReadFile(r.path)
	if err != nil {
		if os.IsNotExist(err) {
			return &model.Config{}, nil
		}
		return nil, err
	}

	var cfg model.Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (r *JSONConfigRepository) Save(ctx context.Context, cfg *model.Config) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(r.path, data, 0644)
}
