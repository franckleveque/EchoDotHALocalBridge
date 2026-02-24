package ports

import (
	"context"
	"hue-bridge-emulator/internal/domain/model"
)

type BridgePort interface {
	GetDevices(ctx context.Context) ([]*model.Device, error)
	GetDevice(ctx context.Context, id string) (*model.Device, error)
	UpdateDeviceState(ctx context.Context, id string, state map[string]interface{}) error
}
