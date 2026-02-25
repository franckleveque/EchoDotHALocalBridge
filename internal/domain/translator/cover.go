package translator

import (
	"github.com/amimof/huego"
	"hue-bridge-emulator/internal/domain/model"
)

type CoverStrategy struct{}

func (s *CoverStrategy) ToHue(haState map[string]interface{}, vd *model.VirtualDevice) *huego.State {
	state := &huego.State{}
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

func (s *CoverStrategy) ToHA(hueState *huego.State, vd *model.VirtualDevice) (string, map[string]interface{}) {
	service := "set_cover_position"
	params := make(map[string]interface{})
	params["position"] = int(float64(hueState.Bri) * 100 / 254)

	if vd.ActionConfig != nil {
		if hueState.On {
			if vd.ActionConfig.OnService != "" {
				service = vd.ActionConfig.OnService
			}
			for k, v := range vd.ActionConfig.OnPayload {
				params[k] = v
			}
		} else {
			if vd.ActionConfig.OffService != "" {
				service = vd.ActionConfig.OffService
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
