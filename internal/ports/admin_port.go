package ports

import (
	"context"
	"hue-bridge-emulator/internal/domain/model"
)

// AdminPort defines the interface for administrative tasks
type AdminPort interface {
	GetConfig(ctx context.Context) (*model.Config, error)
	UpdateConfig(ctx context.Context, cfg *model.Config) error
	GetAllEntities(ctx context.Context) ([]HomeAssistantEntity, error)
	TestDeviceAction(ctx context.Context, vd *model.VirtualDevice, state *model.DeviceState) error
}
