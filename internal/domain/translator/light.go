package translator

import (
	"github.com/amimof/huego"
	"hue-bridge-emulator/internal/domain/model"
)

type LightStrategy struct{}

func (s *LightStrategy) ToHue(haState map[string]interface{}, vd *model.VirtualDevice) *huego.State {
	state := &huego.State{}
	val, _ := haState["state"].(string)
	state.On = (val == "on")
	if attr, ok := haState["attributes"].(map[string]interface{}); ok {
		if bri, ok := attr["brightness"].(float64); ok {
			state.Bri = uint8(bri)
		}
	}
	state.Reachable = true
	return state
}

func (s *LightStrategy) ToHA(hueState *huego.State, vd *model.VirtualDevice) (string, map[string]interface{}) {
	service := "turn_on"
	if !hueState.On {
		service = "turn_off"
	}
	params := make(map[string]interface{})
	if hueState.Bri > 0 {
		params["brightness"] = hueState.Bri
	}

	if vd.ActionConfig != nil {
		if hueState.On && vd.ActionConfig.OnService != "" {
			service = vd.ActionConfig.OnService
		} else if !hueState.On && vd.ActionConfig.OffService != "" {
			service = vd.ActionConfig.OffService
		}
	}

	return service, params
}

func (s *LightStrategy) GetMetadata() model.HueMetadata {
	return model.HueMetadata{
		Type:             "Extended color light",
		ModelID:          "LCT001",
		ManufacturerName: "Philips",
	}
}
