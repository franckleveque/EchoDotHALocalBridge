package ports

import (
	"context"
	"hue-bridge-emulator/internal/domain/model"
)

type HomeAssistantEntity struct {
	EntityID     string `json:"entity_id"`
	FriendlyName string `json:"friendly_name"`
}

type Translator interface {
	ToHue(haState any, vd *model.VirtualDevice) *model.DeviceState
	ToHA(hueState *model.DeviceState, vd *model.VirtualDevice) model.HomeAssistantCommand
	GetMetadata() model.HueMetadata
}

type TranslatorFactory interface {
	GetTranslator(mappingType model.MappingType) Translator
}

// HueEmulationPort defines the interface for Hue protocol emulation
type HueEmulationPort interface {
	GetDevices(ctx context.Context) ([]*model.Device, error)
	GetDevice(ctx context.Context, id string) (*model.Device, error)
	GetDeviceMetadata(deviceType model.MappingType) model.HueMetadata
	UpdateDeviceState(ctx context.Context, id string, state *model.DeviceState) error
}

// AdminPort defines the interface for administrative tasks
type AdminPort interface {
	GetConfig(ctx context.Context) (*model.Config, error)
	UpdateConfig(ctx context.Context, cfg *model.Config) error
	GetAllEntities(ctx context.Context) ([]HomeAssistantEntity, error)
	TestDeviceAction(ctx context.Context, vd *model.VirtualDevice, state *model.DeviceState) error
}

// BridgePort combines emulation and administration (deprecated, use segregated interfaces)
type BridgePort interface {
	HueEmulationPort
	AdminPort
}

type HomeAssistantPort interface {
	GetRawStates(ctx context.Context) ([]any, error)
	GetAllEntities(ctx context.Context) ([]HomeAssistantEntity, error)
	SetState(ctx context.Context, device *model.Device, cmd model.HomeAssistantCommand) error
	Configure(url, token string)
	IsConfigured() bool
}
