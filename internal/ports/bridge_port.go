package ports

import (
	"context"
	"hue-bridge-emulator/internal/domain/model"
)

type BridgePort interface {
	GetDevices(ctx context.Context) ([]*model.Device, error)
	GetDevice(ctx context.Context, id string) (*model.Device, error)
	UpdateDeviceState(ctx context.Context, id string, state map[string]interface{}) error
	TestDeviceAction(ctx context.Context, vd *model.VirtualDevice, stateUpdate map[string]interface{}) error

	// Config management
	GetConfig(ctx context.Context) (*model.Config, error)
	UpdateConfig(ctx context.Context, cfg *model.Config) error
	GetAllEntities(ctx context.Context) ([]HomeAssistantEntity, error)
	GetRawStates(ctx context.Context) ([]map[string]interface{}, error)
}
