package translator

import (
	"github.com/amimof/huego"
	"hue-bridge-emulator/internal/domain/model"
)

type ClimateStrategy struct{}

func (s *ClimateStrategy) ToHue(haState map[string]interface{}, vd *model.VirtualDevice) *huego.State {
	state := &huego.State{}
	state.On = true
	if attr, ok := haState["attributes"].(map[string]interface{}); ok {
		if temp, ok := attr["temperature"].(float64); ok {
			// Map 7-28°C to 0-254
			if temp < 7 {
				temp = 7
			}
			if temp > 28 {
				temp = 28
			}
			state.Bri = uint8((temp - 7) * 254 / (28 - 7))
		}
	}
	state.Reachable = true
	return state
}

func (s *ClimateStrategy) ToHA(hueState *huego.State, vd *model.VirtualDevice) (string, map[string]interface{}) {
	service := "set_temperature"
	params := make(map[string]interface{})
	temp := 7.0 + (float64(hueState.Bri) * (28.0 - 7.0) / 254.0)
	params["temperature"] = temp

	if vd.ActionConfig != nil {
		if hueState.On && vd.ActionConfig.OnService != "" {
			service = vd.ActionConfig.OnService
		} else if !hueState.On && vd.ActionConfig.OffService != "" {
			service = vd.ActionConfig.OffService
		}
	}

	return service, params
}

func (s *ClimateStrategy) GetMetadata() model.HueMetadata {
	return model.HueMetadata{
		Type:             "Dimmable light",
		ModelID:          "LWB004",
		ManufacturerName: "Philips",
	}
}
