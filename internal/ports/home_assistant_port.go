package ports

import (
	"context"
	"hue-bridge-emulator/internal/domain/model"
)

type HomeAssistantPort interface {
	GetRawStates(ctx context.Context) ([]model.HAEntityState, error)
	SetState(ctx context.Context, device *model.Device, cmd model.HomeAssistantCommand) error
}
