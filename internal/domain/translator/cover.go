package translator

import (
	"hue-bridge-emulator/internal/domain/model"
)

type CoverStrategy struct{}

func (s *CoverStrategy) ToHue(haState map[string]interface{}, vd *model.VirtualDevice) *model.DeviceState {
	state := &model.DeviceState{}
	val, _ := haState["state"].(string)
	state.On = (val != "closed")
	if attr, ok := haState["attributes"].(map[string]interface{}); ok {
		if pos, ok := attr["current_position"].(float64); ok {
			state.Bri = uint8(pos * 254 / 100)
		}
	}
	state.Reachable = true
	return state
}

func (s *CoverStrategy) ToHA(hueState *model.DeviceState, vd *model.VirtualDevice) (string, map[string]interface{}) {
	service := "set_cover_position"
	params := make(map[string]interface{})

	// Check if this was a brightness (position) update or just toggle
	if hueState.UpdatedByBri {
		params["position"] = int(float64(hueState.Bri) * 100 / 254)
	} else {
		if hueState.On {
			params["position"] = 100
		} else {
			params["position"] = 0
		}
	}

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

	return service, params
}

func (s *CoverStrategy) GetMetadata() model.HueMetadata {
	return model.HueMetadata{
		Type:             "Window covering device",
		ModelID:          "LCT001",
		ManufacturerName: "Philips",
	}
}
