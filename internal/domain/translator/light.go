package translator

import (
	"hue-bridge-emulator/internal/domain/model"
	"strings"
)

type LightStrategy struct{}

func (s *LightStrategy) ToHue(haState model.HAEntityState, vd *model.VirtualDevice) *model.DeviceState {
	state := &model.DeviceState{}
	state.On = (haState.State == "on")
	if bri, ok := haState.Attributes["brightness"].(float64); ok {
		state.Bri = uint8(bri)
	}
	state.Reachable = true
	return state
}

func (s *LightStrategy) ToHA(hueState *model.DeviceState, vd *model.VirtualDevice) model.HomeAssistantCommand {
	service := "turn_on"
	params := make(model.HAFields)

	// Map domain to service if possible
	domain := "light"
	if vd.EntityID != "" {
		parts := strings.Split(vd.EntityID, ".")
		domain = parts[0]
	}

	if !hueState.On {
		service = "turn_off"
	} else {
		// Only include brightness for light domain or if explicitly on
		if domain == "light" && hueState.Bri > 0 {
			params["brightness"] = hueState.Bri
		}
	}

	var effect string
	if vd.ActionConfig != nil {
		if hueState.On {
			if vd.ActionConfig.OnService != "" {
				service = vd.ActionConfig.OnService
			}
			effect = vd.ActionConfig.OnEffect
			for k, v := range vd.ActionConfig.OnPayload {
				params[k] = v
			}
		} else {
			if vd.ActionConfig.OffService != "" {
				service = vd.ActionConfig.OffService
			}
			effect = vd.ActionConfig.OffEffect
			for k, v := range vd.ActionConfig.OffPayload {
				params[k] = v
			}
		}
	}

	return model.HomeAssistantCommand{
		Service: service,
		Data:    params,
		Effect:  effect,
	}
}

func (s *LightStrategy) GetMetadata() model.HueMetadata {
	return model.HueMetadata{
		Type:             "Extended color light",
		ModelID:          "LCT001",
		ManufacturerName: "Philips",
	}
}
