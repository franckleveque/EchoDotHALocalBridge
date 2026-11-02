package ports

import (
	"context"
	"hue-bridge-emulator/internal/domain/model"
)

// HueEmulationPort defines the interface for Hue protocol emulation
type HueEmulationPort interface {
	GetDevices(ctx context.Context) ([]*model.Device, error)
	GetDevice(ctx context.Context, id string) (*model.Device, error)
	GetDeviceMetadata(deviceType model.MappingType) model.HueMetadata
	UpdateDeviceState(ctx context.Context, id string, state *model.DeviceState) error
}
