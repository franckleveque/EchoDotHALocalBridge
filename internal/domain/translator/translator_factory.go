package translator

import (
	"hue-bridge-emulator/internal/domain/model"
)

// TranslatorFactory translates between HA and Hue for a mapping type. Callers
// tell the factory what to translate instead of asking it for a strategy.
type TranslatorFactory interface {
	ToHue(mappingType model.MappingType, haState model.HAEntityState, vd *model.VirtualDevice) *model.DeviceState
	ToHA(mappingType model.MappingType, hueState *model.DeviceState, vd *model.VirtualDevice) model.HomeAssistantCommand
	Metadata(mappingType model.MappingType) model.HueMetadata
}
