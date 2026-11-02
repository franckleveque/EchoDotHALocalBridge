package ports

import (
	"context"
)

type AuthService interface {
	Verify(ctx context.Context, username, password string) (bool, error)
	CreateCredentials(ctx context.Context, username, password string) error
	Exists() bool
}
