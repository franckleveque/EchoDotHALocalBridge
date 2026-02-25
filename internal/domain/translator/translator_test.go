package translator

import (
	"testing"
	"github.com/stretchr/testify/assert"
	"hue-bridge-emulator/internal/domain/model"
)

func TestLightStrategy(t *testing.T) {
	s := &LightStrategy{}
	vd := &model.VirtualDevice{
		Type: model.MappingTypeLight,
		ActionConfig: &model.ActionConfig{
			OnPayload: map[string]interface{}{"extra": "on"},
			OffPayload: map[string]interface{}{"extra": "off"},
		},
	}

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

	// Hue to HA (ON)
	hueState.Bri = 200
	service, haParams := s.ToHA(hueState, vd)
	assert.Equal(t, "turn_on", service)
	assert.Equal(t, uint8(200), haParams["brightness"])
	assert.Equal(t, "on", haParams["extra"])

	// Hue to HA (OFF)
	hueState.On = false
	service, haParams = s.ToHA(hueState, vd)
	assert.Equal(t, "turn_off", service)
	assert.Equal(t, "off", haParams["extra"])
}

func TestCoverStrategy(t *testing.T) {
	s := &CoverStrategy{}
	vd := &model.VirtualDevice{
		Type: model.MappingTypeCover,
		ActionConfig: &model.ActionConfig{
			OffPayload: map[string]interface{}{"extra": "off"},
		},
	}
	haState := map[string]interface{}{
		"state": "open",
		"attributes": map[string]interface{}{"current_position": 50.0},
	}
	hueState := s.ToHue(haState, vd)
	assert.Equal(t, uint8(127), hueState.Bri)

	hueState.Bri = 254
	hueState.On = true
	service, haParams := s.ToHA(hueState, vd)
	assert.Equal(t, "set_cover_position", service)
	assert.Equal(t, 100, haParams["position"])

	hueState.On = false
	service, haParams = s.ToHA(hueState, vd)
	assert.Equal(t, "set_cover_position", service)
	assert.Equal(t, "off", haParams["extra"])
}

func TestClimateStrategy(t *testing.T) {
	s := &ClimateStrategy{}
	vd := &model.VirtualDevice{
		Type: model.MappingTypeClimate,
		ActionConfig: &model.ActionConfig{
			OnPayload: map[string]interface{}{"hvac_mode": "heat"},
			OffPayload: map[string]interface{}{"hvac_mode": "off"},
		},
	}
	haState := map[string]interface{}{
		"attributes": map[string]interface{}{"temperature": 21.0},
	}
	hueState := s.ToHue(haState, vd)
	assert.Equal(t, uint8(169), hueState.Bri)

	hueState.Bri = 254
	hueState.On = true
	service, haParams := s.ToHA(hueState, vd)
	assert.Equal(t, "set_temperature", service)
	assert.Equal(t, 28.0, haParams["temperature"])
	assert.Equal(t, "heat", haParams["hvac_mode"])

	hueState.On = false
	service, haParams = s.ToHA(hueState, vd)
	assert.Equal(t, "set_temperature", service)
	assert.Equal(t, "off", haParams["hvac_mode"])
}

func TestCustomStrategy(t *testing.T) {
	s := &CustomStrategy{}
	vd := &model.VirtualDevice{
		EntityID: "camera.salon",
		Type: model.MappingTypeCustom,
		ActionConfig: &model.ActionConfig{
			ToHueFormula: "x * 2.54",
			ToHAFormula: "x / 2.54",
			OnPayload: map[string]interface{}{"extra": "on"},
			OffPayload: map[string]interface{}{"extra": "off"},
		},
	}

	haState := map[string]interface{}{
		"state": "idle",
		"attributes": map[string]interface{}{
			"brightness": 50.0,
			"current_position": 10.0,
			"temperature": 20.0,
			"value": 30.0,
		},
	}
	// ToHue should pick brightness first
	hueState := s.ToHue(haState, vd)
	assert.Equal(t, uint8(127), hueState.Bri)

	hueState.Bri = 254
	hueState.On = true
	service, haParams := s.ToHA(hueState, vd)
	assert.Equal(t, "turn_on", service)
	assert.InDelta(t, 100.0, haParams["value"].(float64), 0.1)
	assert.Equal(t, "on", haParams["extra"])

	hueState.On = false
	service, haParams = s.ToHA(hueState, vd)
	assert.Equal(t, "turn_off", service)
	assert.Equal(t, "off", haParams["extra"])

	// Test other domains for ToHA
	vd.EntityID = "light.test"
	service, haParams = s.ToHA(hueState, vd)
	assert.Contains(t, haParams, "brightness")

	vd.EntityID = "cover.test"
	service, haParams = s.ToHA(hueState, vd)
	assert.Equal(t, "set_cover_position", service)

	vd.EntityID = "climate.test"
	service, haParams = s.ToHA(hueState, vd)
	assert.Equal(t, "set_temperature", service)

	vd.EntityID = "input_number.test"
	service, haParams = s.ToHA(hueState, vd)
	assert.Equal(t, "set_value", service)
}

func TestCustomStrategy_Evaluate(t *testing.T) {
	s := &CustomStrategy{}
	assert.Equal(t, 10.0, s.evaluate("x * 2", 5))
	assert.Equal(t, 5.0, s.evaluate("x / 2", 10))
	assert.Equal(t, 5.0, s.evaluate("invalid", 5))
}

func TestMetadata(t *testing.T) {
	ls := &LightStrategy{}
	assert.Equal(t, "Extended color light", ls.GetMetadata().Type)

	cs := &ClimateStrategy{}
	assert.Equal(t, "Dimmable light", cs.GetMetadata().Type)

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
