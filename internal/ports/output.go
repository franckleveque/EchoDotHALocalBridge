package ports

import (
	"context"
	"hue-bridge-emulator/internal/domain/model"
)

type HomeAssistantPort interface {
	GetDevices(ctx context.Context) ([]*model.Device, error)
	SetState(ctx context.Context, device *model.Device, params map[string]interface{}) error
	Configure(url, token string)
	IsConfigured() bool
}
