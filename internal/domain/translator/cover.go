package translator

import (
	"hue-bridge-emulator/internal/domain/model"
)

type CoverStrategy struct{}

func (s *CoverStrategy) ToHue(haState map[string]interface{}, vd *model.VirtualDevice) *model.DeviceState {
	state := &model.DeviceState{}
	val, _ := haState["state"].(string)
	state.On = (val == "open")
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

	// Map Hue On/Off state directly to HA position
	// - Open (On) = 100
	// - Closed (Off) = 0
	if !hueState.On {
		params["position"] = 0
	} else if hueState.UpdatedByBri {
		// If explicitly adjusted via the slider (bri update)
		params["position"] = int(float64(hueState.Bri) * 100 / 254)
	} else {
		// If just turned ON
		params["position"] = 100
	}

	if vd.ActionConfig != nil {
		if hueState.On {
			if vd.ActionConfig.OnService != "" {
				service = vd.ActionConfig.OnService
				// Only clear params if we're not using the default 'set_cover_position'
				if service != "set_cover_position" {
					params = make(map[string]interface{})
				}
			}
			for k, v := range vd.ActionConfig.OnPayload {
				params[k] = v
			}
		} else {
			if vd.ActionConfig.OffService != "" {
				service = vd.ActionConfig.OffService
				if service != "set_cover_position" {
					params = make(map[string]interface{})
				}
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
		ModelID:          "LCW001",
		ManufacturerName: "Philips",
	}
}
