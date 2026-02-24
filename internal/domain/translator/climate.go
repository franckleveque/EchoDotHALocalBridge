package translator

import (
	"github.com/amimof/huego"
	"hue-bridge-emulator/internal/domain/model"
)

type ClimateStrategy struct{}

func (s *ClimateStrategy) ToHue(haState map[string]interface{}, mapping *model.EntityMapping) *huego.State {
	state := &huego.State{}
	if attr, ok := haState["attributes"].(map[string]interface{}); ok {
		if temp, ok := attr["temperature"].(float64); ok {
			if temp < 7 {
				temp = 7
			}
			if temp > 28 {
				temp = 28
			}
			state.Bri = uint8((temp - 7) * 254 / 21)
		}
	}
	state.On = true
	state.Reachable = true
	return state
}

func (s *ClimateStrategy) ToHA(hueState *huego.State, mapping *model.EntityMapping) map[string]interface{} {
	params := make(map[string]interface{})
	temp := float64(hueState.Bri)*21/254 + 7
	params["temperature"] = temp
	return params
}
