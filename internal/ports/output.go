package ports

import (
	"context"
	"hue-bridge-emulator/internal/domain/model"
)

type HomeAssistantPort interface {
	GetRawStates(ctx context.Context) ([]map[string]interface{}, error)
	GetAllEntities(ctx context.Context) ([]*model.EntityMapping, error)
	SetState(ctx context.Context, device *model.Device, params map[string]interface{}) error
	Configure(url, token string)
	IsConfigured() bool
}
