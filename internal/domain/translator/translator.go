package translator

import (
	"github.com/amimof/huego"
	"hue-bridge-emulator/internal/domain/model"
)

// Translator defines the interface for translating between Hue and Home Assistant states
type Translator interface {
	ToHue(haState map[string]interface{}, vd *model.VirtualDevice) *huego.State
	ToHA(hueState *huego.State, vd *model.VirtualDevice) (string, map[string]interface{})
	GetMetadata() model.HueMetadata
}
