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

type AuthService interface {
	Verify(ctx context.Context, username, password string) (bool, error)
	CreateCredentials(ctx context.Context, username, password string) error
	Exists() bool
}
