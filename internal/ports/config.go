package ports

import (
	"context"
	"hue-bridge-emulator/internal/domain/model"
)

type ConfigRepository interface {
	Get(ctx context.Context) (*model.Config, error)
	Save(ctx context.Context, config *model.Config) error
}
