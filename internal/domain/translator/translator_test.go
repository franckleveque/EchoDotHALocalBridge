package translator

import (
	"testing"
	"github.com/stretchr/testify/assert"
	"hue-bridge-emulator/internal/domain/model"
)

func TestLightStrategy(t *testing.T) {
	s := &LightStrategy{}
	vd := &model.VirtualDevice{Type: model.MappingTypeLight}

	// HA to Hue
	haState := map[string]interface{}{
		"state": "on",
		"attributes": map[string]interface{}{
			"brightness": 127.0,
		},
	}
	hueState := s.ToHue(haState, vd)
	assert.True(t, hueState.On)
	assert.Equal(t, uint8(127), hueState.Bri)

	// Hue to HA
	hueState.Bri = 200
	service, haParams := s.ToHA(hueState, vd)
	assert.Equal(t, "turn_on", service)
	assert.Equal(t, uint8(200), haParams["brightness"])

	// Test custom service
	vd.ActionConfig = &model.ActionConfig{OnService: "light.special_on"}
	hueState.On = true
	service, _ = s.ToHA(hueState, vd)
	assert.Equal(t, "light.special_on", service)
}

func TestCoverStrategy(t *testing.T) {
	s := &CoverStrategy{}
	vd := &model.VirtualDevice{Type: model.MappingTypeCover}
	haState := map[string]interface{}{
		"state": "open",
		"attributes": map[string]interface{}{"current_position": 50.0},
	}
	hueState := s.ToHue(haState, vd)
	assert.Equal(t, uint8(127), hueState.Bri)

	hueState.Bri = 254
	service, haParams := s.ToHA(hueState, vd)
	assert.Equal(t, "set_cover_position", service)
	assert.Equal(t, 100, haParams["position"])

	// Test custom service
	vd.ActionConfig = &model.ActionConfig{OnService: "cover.special_open"}
	hueState.On = true
	service, _ = s.ToHA(hueState, vd)
	assert.Equal(t, "cover.special_open", service)
}

func TestClimateStrategy(t *testing.T) {
	s := &ClimateStrategy{}
	vd := &model.VirtualDevice{Type: model.MappingTypeClimate}
	haState := map[string]interface{}{
		"attributes": map[string]interface{}{"temperature": 21.0},
	}
	hueState := s.ToHue(haState, vd)
	assert.Equal(t, uint8(169), hueState.Bri)

	hueState.Bri = 254
	service, haParams := s.ToHA(hueState, vd)
	assert.Equal(t, "set_temperature", service)
	assert.Equal(t, 28.0, haParams["temperature"])

	// Test custom service
	vd.ActionConfig = &model.ActionConfig{OffService: "climate.special_off"}
	hueState.On = false
	service, _ = s.ToHA(hueState, vd)
	assert.Equal(t, "climate.special_off", service)
}

func TestCustomStrategy(t *testing.T) {
	s := &CustomStrategy{}
	vd := &model.VirtualDevice{
		EntityID: "input_number.test",
		Type: model.MappingTypeCustom,
		ActionConfig: &model.ActionConfig{
			ToHueFormula: "x * 2.54",
			ToHAFormula: "x / 2.54",
		},
	}

	haState := map[string]interface{}{
		"state": "50",
		"attributes": map[string]interface{}{"brightness": 50.0},
	}
	hueState := s.ToHue(haState, vd)
	assert.Equal(t, uint8(127), hueState.Bri)

	// Test other attributes
	haState["attributes"] = map[string]interface{}{"current_position": 10.0}
	hueState = s.ToHue(haState, vd)
	assert.Equal(t, uint8(25), hueState.Bri)

	haState["attributes"] = map[string]interface{}{"temperature": 15.0}
	hueState = s.ToHue(haState, vd)
	assert.Equal(t, uint8(38), hueState.Bri)

	haState["attributes"] = map[string]interface{}{"value": 100.0}
	hueState = s.ToHue(haState, vd)
	assert.Equal(t, uint8(254), hueState.Bri)

	hueState.Bri = 254
	service, haParams := s.ToHA(hueState, vd)
	assert.Equal(t, "set_value", service)
	assert.InDelta(t, 100.0, haParams["value"].(float64), 0.1)

	// Test dynamic service
	vd.ActionConfig.OnService = "camera.enable_motion"
	hueState.On = true
	service, haParams = s.ToHA(hueState, vd)
	assert.Equal(t, "camera.enable_motion", service)

	// Test Off Service and Effects
	vd.ActionConfig.OffService = "camera.disable_motion"
	vd.ActionConfig.OffEffect = "homeassistant.update_entity"
	hueState.On = false
	service, haParams = s.ToHA(hueState, vd)
	assert.Equal(t, "camera.disable_motion", service)
	assert.Equal(t, "homeassistant.update_entity", haParams["effect"])
}

func TestCustomStrategy_Evaluate(t *testing.T) {
	s := &CustomStrategy{}
	assert.Equal(t, 10.0, s.evaluate("x * 2", 5))
	assert.Equal(t, 5.0, s.evaluate("x / 2", 10))
	assert.Equal(t, 15.0, s.evaluate("x + 5", 10))
	assert.Equal(t, 5.0, s.evaluate("x - 5", 10))
	assert.Equal(t, 20.0, s.evaluate("x * 2 + 10", 5))

	// Error path
	assert.Equal(t, 5.0, s.evaluate("invalid", 5.0))
}

func TestMetadata(t *testing.T) {
	ls := &LightStrategy{}
	assert.Equal(t, "Extended color light", ls.GetMetadata().Type)

	cs := &ClimateStrategy{}
	assert.Equal(t, "Dimmable light", cs.GetMetadata().Type)
	assert.Equal(t, "LWB004", cs.GetMetadata().ModelID)

	covs := &CoverStrategy{}
	assert.Equal(t, "Window covering device", covs.GetMetadata().Type)

	custs := &CustomStrategy{}
	assert.Equal(t, "Extended color light", custs.GetMetadata().Type)
}

func TestFactory(t *testing.T) {
	f := NewFactory()
	assert.IsType(t, &LightStrategy{}, f.GetTranslator(model.MappingTypeLight))
	assert.IsType(t, &CoverStrategy{}, f.GetTranslator(model.MappingTypeCover))
	assert.IsType(t, &ClimateStrategy{}, f.GetTranslator(model.MappingTypeClimate))
	assert.IsType(t, &CustomStrategy{}, f.GetTranslator(model.MappingTypeCustom))
}
