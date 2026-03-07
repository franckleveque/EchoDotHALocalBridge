package ports

import (
	"context"
	"hue-bridge-emulator/internal/domain/model"
)

type AuthPort interface {
	Get(ctx context.Context) (*model.AuthConfig, error)
	Save(ctx context.Context, auth *model.AuthConfig) error
	Exists() bool
}
