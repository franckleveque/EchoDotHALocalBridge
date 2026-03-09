package ports

import (
	"context"
	"hue-bridge-emulator/internal/domain/model"
	"hue-bridge-emulator/internal/domain/translator"
)

type HomeAssistantEntity struct {
	EntityID     string `json:"entity_id"`
	FriendlyName string `json:"friendly_name"`
}

type Translator = translator.Translator

type TranslatorFactory = translator.TranslatorFactory


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


type HomeAssistantPort interface {
	GetRawStates(ctx context.Context) ([]model.HAEntityState, error)
	SetState(ctx context.Context, device *model.Device, cmd model.HomeAssistantCommand) error
}

// ReconfigurableHomeAssistantPort defines an interface for HomeAssistant ports that can be reconfigured at runtime
type ReconfigurableHomeAssistantPort interface {
	HomeAssistantPort
	Configure(url, token string)
}

// Reconfigurable defines an interface for ports that can be reconfigured at runtime
type Reconfigurable interface {
	Configure(url, token string)
}
