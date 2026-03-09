package translator

import (
	"github.com/Knetic/govaluate"
	"hue-bridge-emulator/internal/domain/model"
	"strings"
)

type CustomStrategy struct{}

func (s *CustomStrategy) ToHue(haState model.HAEntityState, vd *model.VirtualDevice) *model.DeviceState {
	state := &model.DeviceState{}
	state.On = (haState.State != "off" && haState.State != "closed" && haState.State != "unavailable")

	// Default to brightness/level if available
	var input float64
	if v, ok := haState.Attributes["brightness"].(float64); ok {
		input = v
	} else if v, ok := haState.Attributes["current_position"].(float64); ok {
		input = v
	} else if v, ok := haState.Attributes["temperature"].(float64); ok {
		input = v
	} else if v, ok := haState.Attributes["value"].(float64); ok {
		input = v
	}

	if vd.ActionConfig != nil && vd.ActionConfig.ToHueFormula != "" {
		state.Bri = uint8(s.evaluate(vd.ActionConfig.ToHueFormula, input))
	} else {
		state.Bri = uint8(input)
	}

	state.Reachable = true
	return state
}

func (s *CustomStrategy) ToHA(hueState *model.DeviceState, vd *model.VirtualDevice) model.HomeAssistantCommand {
	service := "turn_on"
	if !hueState.On {
		service = "turn_off"
	}

	params := make(model.HAFields)
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

	var effect string
	if vd.ActionConfig != nil {
		if hueState.On {
			if vd.ActionConfig.OnService != "" {
				service = vd.ActionConfig.OnService
			}
			effect = vd.ActionConfig.OnEffect
			// Merge custom ON payload
			for k, v := range vd.ActionConfig.OnPayload {
				params[k] = v
			}
		} else {
			if vd.ActionConfig.OffService != "" {
				service = vd.ActionConfig.OffService
			}
			effect = vd.ActionConfig.OffEffect
			// Merge custom OFF payload
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
