package translator

import (
	"hue-bridge-emulator/internal/domain/model"
)

// Translator defines the interface for translating between Hue and Home Assistant states
type Translator interface {
	ToHue(haState map[string]interface{}, vd *model.VirtualDevice) *model.DeviceState
	ToHA(hueState *model.DeviceState, vd *model.VirtualDevice) (string, map[string]interface{})
	GetMetadata() model.HueMetadata
}
