package translator

import (
	"github.com/amimof/huego"
	"hue-bridge-emulator/internal/domain/model"
)

// Translator defines the interface for translating between Hue and Home Assistant states
type Translator interface {
	ToHue(haState map[string]interface{}, mapping *model.EntityMapping) *huego.State
	ToHA(hueState *huego.State, mapping *model.EntityMapping) map[string]interface{}
	GetMetadata() model.HueMetadata
}
