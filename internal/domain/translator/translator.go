package translator

import (
	"github.com/amimof/huego"
)

// Translator defines the interface for translating between Hue and Home Assistant states
type Translator interface {
	ToHue(haState map[string]interface{}) *huego.State
	ToHA(hueState *huego.State) map[string]interface{}
}
