package translator

import (
	"github.com/amimof/huego"
	"hue-bridge-emulator/internal/domain/model"
)

type CoverStrategy struct{}

func (s *CoverStrategy) ToHue(haState map[string]interface{}, mapping *model.EntityMapping) *huego.State {
	state := &huego.State{}
	if val, ok := haState["state"].(string); ok {
		state.On = (val != "closed")
	}
	if attr, ok := haState["attributes"].(map[string]interface{}); ok {
		if pos, ok := attr["current_position"].(float64); ok {
			state.Bri = uint8(pos * 2.54)
		}
	}
	state.Reachable = true
	return state
}

func (s *CoverStrategy) ToHA(hueState *huego.State, mapping *model.EntityMapping) (string, map[string]interface{}) {
	service := "set_cover_position"
	params := make(map[string]interface{})
	params["position"] = int(float64(hueState.Bri) / 2.54)
	return service, params
}

func (s *CoverStrategy) GetMetadata() model.HueMetadata {
	return model.HueMetadata{
		Type:             "Window covering device",
		ModelID:          "LCT001",
		ManufacturerName: "Philips",
	}
}
