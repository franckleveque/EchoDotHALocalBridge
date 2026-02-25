package translator

import (
	"fmt"
	"github.com/amimof/huego"
	"hue-bridge-emulator/internal/domain/model"
	"strconv"
	"strings"
)

type CustomStrategy struct{}

func (s *CustomStrategy) ToHue(haState map[string]interface{}, mapping *model.EntityMapping) *huego.State {
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
		}
	}

	if mapping.CustomFormula != nil && mapping.CustomFormula.ToHueFormula != "" {
		state.Bri = uint8(s.evaluate(mapping.CustomFormula.ToHueFormula, input))
	} else {
		state.Bri = uint8(input)
	}

	state.Reachable = true
	return state
}

func (s *CustomStrategy) ToHA(hueState *huego.State, mapping *model.EntityMapping) map[string]interface{} {
	params := make(map[string]interface{})
	params["on"] = hueState.On

	input := float64(hueState.Bri)
	var output float64
	if mapping.CustomFormula != nil && mapping.CustomFormula.ToHAFormula != "" {
		output = s.evaluate(mapping.CustomFormula.ToHAFormula, input)
	} else {
		output = input
	}

	// Guessing attribute name based on entity domain if possible
	domain := strings.Split(mapping.EntityID, ".")[0]
	switch domain {
	case "light":
		params["brightness"] = output
	case "cover":
		params["position"] = int(output)
	case "climate":
		params["temperature"] = output
	case "input_number":
		params["value"] = output
	default:
		params["value"] = output
	}

	if mapping.CustomFormula != nil {
		if hueState.On && mapping.CustomFormula.OnEffect != "" {
			params["effect"] = mapping.CustomFormula.OnEffect
		} else if !hueState.On && mapping.CustomFormula.OffEffect != "" {
			params["effect"] = mapping.CustomFormula.OffEffect
		}
	}

	return params
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
	formula = strings.ReplaceAll(formula, "x", fmt.Sprintf(" %f ", x))
	for _, op := range []string{"*", "/", "+", "-"} {
		formula = strings.ReplaceAll(formula, op, " "+op+" ")
	}
	// This is a VERY hacky "evaluator". For a real production app, use a proper expression parser.
	// For this task, I'll support basic multiplication and addition.
	parts := strings.Fields(formula)
	if len(parts) == 0 {
		return x
	}

	res, err := strconv.ParseFloat(parts[0], 64)
	if err != nil {
		return x
	}

	for i := 1; i < len(parts)-1; i += 2 {
		op := parts[i]
		nextVal, err := strconv.ParseFloat(parts[i+1], 64)
		if err != nil {
			continue
		}
		switch op {
		case "*":
			res *= nextVal
		case "/":
			if nextVal != 0 {
				res /= nextVal
			}
		case "+":
			res += nextVal
		case "-":
			res -= nextVal
		}
	}
	return res
}
