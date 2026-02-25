package translator

import (
	"testing"
	"github.com/stretchr/testify/assert"
	"hue-bridge-emulator/internal/domain/model"
)

func TestLightStrategy(t *testing.T) {
	s := &LightStrategy{}
	mapping := &model.EntityMapping{Type: model.MappingTypeLight}

	// HA to Hue
	haState := map[string]interface{}{
		"state": "on",
		"attributes": map[string]interface{}{
			"brightness": 127.0,
		},
	}
	hueState := s.ToHue(haState, mapping)
	assert.True(t, hueState.On)
	assert.Equal(t, uint8(127), hueState.Bri)

	// Hue to HA
	hueState.Bri = 200
	service, haParams := s.ToHA(hueState, mapping)
	assert.Equal(t, "turn_on", service)
	assert.Equal(t, uint8(200), haParams["brightness"])
}

func TestCoverStrategy(t *testing.T) {
	s := &CoverStrategy{}
	mapping := &model.EntityMapping{Type: model.MappingTypeCover}
	haState := map[string]interface{}{
		"state": "open",
		"attributes": map[string]interface{}{"current_position": 50.0},
	}
	hueState := s.ToHue(haState, mapping)
	assert.Equal(t, uint8(127), hueState.Bri)

	hueState.Bri = 254
	service, haParams := s.ToHA(hueState, mapping)
	assert.Equal(t, "set_cover_position", service)
	assert.Equal(t, 100, haParams["position"])
}

func TestClimateStrategy(t *testing.T) {
	s := &ClimateStrategy{}
	mapping := &model.EntityMapping{Type: model.MappingTypeClimate}
	haState := map[string]interface{}{
		"attributes": map[string]interface{}{"temperature": 21.0},
	}
	hueState := s.ToHue(haState, mapping)
	assert.Equal(t, uint8(169), hueState.Bri)

	hueState.Bri = 254
	service, haParams := s.ToHA(hueState, mapping)
	assert.Equal(t, "set_temperature", service)
	assert.Equal(t, 28.0, haParams["temperature"])
}

func TestCustomStrategy(t *testing.T) {
	s := &CustomStrategy{}
	mapping := &model.EntityMapping{
		EntityID: "input_number.test",
		Type: model.MappingTypeCustom,
		CustomFormula: &model.CustomFormula{
			ToHueFormula: "x * 2.54",
			ToHAFormula: "x / 2.54",
		},
	}

	haState := map[string]interface{}{
		"state": "50",
		"attributes": map[string]interface{}{"brightness": 50.0},
	}
	hueState := s.ToHue(haState, mapping)
	assert.Equal(t, uint8(127), hueState.Bri)

	hueState.Bri = 254
	service, haParams := s.ToHA(hueState, mapping)
	assert.Equal(t, "set_value", service)
	assert.InDelta(t, 100.0, haParams["value"].(float64), 0.1)

	// Test dynamic service
	mapping.CustomFormula.OnService = "camera.enable_motion"
	hueState.On = true
	service, haParams = s.ToHA(hueState, mapping)
	assert.Equal(t, "camera.enable_motion", service)

	// Test Off Service and Effects
	mapping.CustomFormula.OffService = "camera.disable_motion"
	mapping.CustomFormula.OffEffect = "homeassistant.update_entity"
	hueState.On = false
	service, haParams = s.ToHA(hueState, mapping)
	assert.Equal(t, "camera.disable_motion", service)
	assert.Equal(t, "homeassistant.update_entity", haParams["effect"])
}

func TestCustomStrategy_Evaluate(t *testing.T) {
	s := &CustomStrategy{}
	assert.Equal(t, 10.0, s.evaluate("x * 2", 5))
	assert.Equal(t, 5.0, s.evaluate("x / 2", 10))
	assert.Equal(t, 15.0, s.evaluate("x + 5", 10))
	assert.Equal(t, 5.0, s.evaluate("x - 5", 10))
	// govaluate supports precedence, so x * 2 + 10 = 5 * 2 + 10 = 20
	assert.Equal(t, 20.0, s.evaluate("x * 2 + 10", 5))
}

func TestMetadata(t *testing.T) {
	ls := &LightStrategy{}
	assert.Equal(t, "Extended color light", ls.GetMetadata().Type)

	cs := &ClimateStrategy{}
	assert.Equal(t, "Dimmable light", cs.GetMetadata().Type)
	assert.Equal(t, "LWB004", cs.GetMetadata().ModelID)

	covs := &CoverStrategy{}
	assert.Equal(t, "Window covering device", covs.GetMetadata().Type)
}

func TestFactory(t *testing.T) {
	f := NewFactory()
	assert.IsType(t, &LightStrategy{}, f.GetTranslator(model.MappingTypeLight))
	assert.IsType(t, &CoverStrategy{}, f.GetTranslator(model.MappingTypeCover))
	assert.IsType(t, &ClimateStrategy{}, f.GetTranslator(model.MappingTypeClimate))
	assert.IsType(t, &CustomStrategy{}, f.GetTranslator(model.MappingTypeCustom))
}
