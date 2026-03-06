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

	// Default position based on brightness
	params["position"] = int(float64(hueState.Bri) * 100 / 254)

	// In Home Assistant:
	// - Open = on
	// - Closed = off
	if !hueState.On {
		service = "close_cover"
		delete(params, "position")
	} else if hueState.Bri == 254 && !hueState.UpdatedByBri {
		// If fully open (from ON command, not from explicit BRI setting)
		service = "open_cover"
		delete(params, "position")
	}

	if vd.ActionConfig != nil {
		if hueState.On {
			if vd.ActionConfig.OnService != "" {
				service = vd.ActionConfig.OnService
				// Reset params if custom service is used, to avoid sending position to non-position services
				params = make(map[string]interface{})
			}
			for k, v := range vd.ActionConfig.OnPayload {
				params[k] = v
			}
		} else {
			if vd.ActionConfig.OffService != "" {
				service = vd.ActionConfig.OffService
				params = make(map[string]interface{})
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
