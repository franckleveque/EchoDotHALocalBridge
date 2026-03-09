package translator

import (
	"hue-bridge-emulator/internal/domain/model"
)

type Translator interface {
	ToHue(haState model.HAEntityState, vd *model.VirtualDevice) *model.DeviceState
	ToHA(hueState *model.DeviceState, vd *model.VirtualDevice) model.HomeAssistantCommand
	GetMetadata() model.HueMetadata
}

type TranslatorFactory interface {
	GetTranslator(mappingType model.MappingType) Translator
}
