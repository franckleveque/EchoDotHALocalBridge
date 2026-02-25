package translator

import (
	"github.com/Knetic/govaluate"
	"github.com/amimof/huego"
	"hue-bridge-emulator/internal/domain/model"
	"strings"
)

type CustomStrategy struct{}

func (s *CustomStrategy) ToHue(haState map[string]interface{}, vd *model.VirtualDevice) *huego.State {
	state := &huego.State{}
	valStr, _ := haState["state"].(string)
	state.On = (valStr != "off" && valStr != "closed" && valStr != "unavailable")

	// Default to brightness/level if available
	var input float64
	if attr, ok := haState["attributes"].(map[string]interface{}); ok {
		if v, ok := attr["brightness"].(float64); ok {
			input = v
		} else if v, ok := attr["current_position"].(float64); ok {
			input = v
		} else if v, ok := attr["temperature"].(float64); ok {
			input = v
		} else if v, ok := attr["value"].(float64); ok {
			input = v
		}
	}

	if vd.ActionConfig != nil && vd.ActionConfig.ToHueFormula != "" {
		state.Bri = uint8(s.evaluate(vd.ActionConfig.ToHueFormula, input))
	} else {
		state.Bri = uint8(input)
	}

	state.Reachable = true
	return state
}

func (s *CustomStrategy) ToHA(hueState *huego.State, vd *model.VirtualDevice) (string, map[string]interface{}) {
	service := "turn_on"
	if !hueState.On {
		service = "turn_off"
	}

	params := make(map[string]interface{})
	input := float64(hueState.Bri)
	var output float64
	if vd.ActionConfig != nil && vd.ActionConfig.ToHAFormula != "" {
		output = s.evaluate(vd.ActionConfig.ToHAFormula, input)
	} else {
		output = input
	}

	// Guessing attribute name based on entity domain if possible
	domain := strings.Split(vd.EntityID, ".")[0]
	switch domain {
	case "light":
		params["brightness"] = output
	case "cover":
		service = "set_cover_position"
		params["position"] = int(output)
	case "climate":
		service = "set_temperature"
		params["temperature"] = output
	case "input_number":
		service = "set_value"
		params["value"] = output
	default:
		params["value"] = output
	}

	if vd.ActionConfig != nil {
		if hueState.On {
			if vd.ActionConfig.OnService != "" {
				service = vd.ActionConfig.OnService
			}
			if vd.ActionConfig.OnEffect != "" {
				params["effect"] = vd.ActionConfig.OnEffect
			}
		} else {
			if vd.ActionConfig.OffService != "" {
				service = vd.ActionConfig.OffService
			}
			if vd.ActionConfig.OffEffect != "" {
				params["effect"] = vd.ActionConfig.OffEffect
			}
		}
	}

	return service, params
}

func (s *CustomStrategy) GetMetadata() model.HueMetadata {
	return model.HueMetadata{
		Type:             "Extended color light",
		ModelID:          "LCT001",
		ManufacturerName: "Philips",
	}
}

// evaluate handles simple formulas like "x * 2.54" or "x / 2.54 + 7"
func (s *CustomStrategy) evaluate(formula string, x float64) float64 {
	expression, err := govaluate.NewEvaluableExpression(formula)
	if err != nil {
		return x
	}
	parameters := make(map[string]interface{}, 1)
	parameters["x"] = x

	result, err := expression.Evaluate(parameters)
	if err != nil {
		return x
	}

	if val, ok := result.(float64); ok {
		return val
	}
	return x
}
