package translator

import (
	"hue-bridge-emulator/internal/domain/model"
)

type ClimateStrategy struct{}

func (s *ClimateStrategy) ToHue(haState any, vd *model.VirtualDevice) *model.DeviceState {
	haMap, _ := haState.(map[string]interface{})
	state := &model.DeviceState{}
	state.On = true
	if attr, ok := haMap["attributes"].(map[string]interface{}); ok {
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

func (s *ClimateStrategy) ToHA(hueState *model.DeviceState, vd *model.VirtualDevice) model.HomeAssistantCommand {
	service := "set_temperature"
	params := make(map[string]interface{})
	temp := 7.0 + (float64(hueState.Bri) * (28.0 - 7.0) / 254.0)
	params["temperature"] = temp

	if vd.ActionConfig != nil {
		if hueState.On {
			if vd.ActionConfig.OnService != "" {
				service = vd.ActionConfig.OnService
			}
			if vd.ActionConfig.OnEffect != "" {
				params["effect"] = vd.ActionConfig.OnEffect
			}
			for k, v := range vd.ActionConfig.OnPayload {
				params[k] = v
			}
		} else {
			if vd.ActionConfig.OffService != "" {
				service = vd.ActionConfig.OffService
			}
			if vd.ActionConfig.OffEffect != "" {
				params["effect"] = vd.ActionConfig.OffEffect
			}
			for k, v := range vd.ActionConfig.OffPayload {
				params[k] = v
			}
		}
	}

	return model.HomeAssistantCommand{
		Service: service,
		Data:    params,
	}
}

func (s *ClimateStrategy) GetMetadata() model.HueMetadata {
	return model.HueMetadata{
		Type:             "Dimmable light",
		ModelID:          "LWB004",
		ManufacturerName: "Philips",
	}
}
