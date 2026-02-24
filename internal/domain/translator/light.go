package translator

import (
	"github.com/amimof/huego"
	"hue-bridge-emulator/internal/domain/model"
)

type LightStrategy struct{}

func (s *LightStrategy) ToHue(haState map[string]interface{}, mapping *model.EntityMapping) *huego.State {
	state := &huego.State{}
	if val, ok := haState["state"].(string); ok {
		state.On = (val == "on")
	}
	if attr, ok := haState["attributes"].(map[string]interface{}); ok {
		if bri, ok := attr["brightness"].(float64); ok {
			state.Bri = uint8(bri)
		}
	}
	state.Reachable = true
	return state
}

func (s *LightStrategy) ToHA(hueState *huego.State, mapping *model.EntityMapping) map[string]interface{} {
	params := make(map[string]interface{})
	params["on"] = hueState.On
	if hueState.Bri > 0 {
		params["brightness"] = hueState.Bri
	}
	return params
}
