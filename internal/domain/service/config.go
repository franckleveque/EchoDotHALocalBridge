package service

import (
	"context"
	"hue-bridge-emulator/internal/domain/model"
	"hue-bridge-emulator/internal/ports"
)

type ConfigService struct {
	repo   ports.ConfigRepository
	haPort ports.HomeAssistantPort
}

func NewConfigService(repo ports.ConfigRepository, haPort ports.HomeAssistantPort) *ConfigService {
	return &ConfigService{
		repo:   repo,
		haPort: haPort,
	}
}

func (s *ConfigService) GetConfig(ctx context.Context) (*model.Config, error) {
	return s.repo.Get(ctx)
}

func (s *ConfigService) UpdateConfig(ctx context.Context, cfg *model.Config) error {
	err := s.repo.Save(ctx, cfg)
	if err != nil {
		return err
	}
	s.haPort.Configure(cfg.HassURL, cfg.HassToken)
	return nil
}
