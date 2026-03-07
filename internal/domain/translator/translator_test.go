package translator

import (
	"hue-bridge-emulator/internal/domain/model"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLightStrategy(t *testing.T) {
	s := &LightStrategy{}
	vd := &model.VirtualDevice{
		Type: model.MappingTypeLight,
		ActionConfig: &model.ActionConfig{
			OnPayload:  map[string]interface{}{"extra": "on"},
			OffPayload: map[string]interface{}{"extra": "off"},
			OnEffect:   "rainbow",
			OffEffect:  "none",
		},
	}

	// HA to Hue (ON)
	haState := map[string]interface{}{
		"state": "on",
		"attributes": map[string]interface{}{
			"brightness": 127.0,
		},
	}
	hueState := s.ToHue(haState, vd)
	assert.True(t, hueState.On)
	assert.Equal(t, uint8(127), hueState.Bri)

	// HA to Hue (OFF/unavailable)
	assert.False(t, s.ToHue(map[string]interface{}{"state": "off"}, vd).On)
	assert.False(t, s.ToHue(map[string]interface{}{"state": "unavailable"}, vd).On)

	// Hue to HA (ON)
	hueState.Bri = 200
	hueState.On = true
	service, haParams := s.ToHA(hueState, vd)
	assert.Equal(t, "turn_on", service)
	assert.Equal(t, uint8(200), haParams["brightness"])
	assert.Equal(t, "on", haParams["extra"])
	assert.Equal(t, "rainbow", haParams["effect"])

	// Hue to HA (OFF)
	hueState.On = false
	service, haParams = s.ToHA(hueState, vd)
	assert.Equal(t, "turn_off", service)
	assert.Equal(t, "off", haParams["extra"])
	assert.Equal(t, "none", haParams["effect"])

	// Test custom service
	vd.ActionConfig.OnService = "light.custom_on"
	vd.ActionConfig.OffService = "light.custom_off"
	hueState.On = true
	service, _ = s.ToHA(hueState, vd)
	assert.Equal(t, "light.custom_on", service)
	hueState.On = false
	service, _ = s.ToHA(hueState, vd)
	assert.Equal(t, "light.custom_off", service)

	// Test non-light domain
	vd_switch := &model.VirtualDevice{EntityID: "switch.test"}
	_, haParams = s.ToHA(&model.DeviceState{On: true, Bri: 127}, vd_switch)
	assert.NotContains(t, haParams, "brightness")
}

func TestCoverStrategy(t *testing.T) {
	s := &CoverStrategy{}
	vd := &model.VirtualDevice{
		Type: model.MappingTypeCover,
		ActionConfig: &model.ActionConfig{
			OnPayload:  map[string]interface{}{"extra": "on"},
			OffPayload: map[string]interface{}{"extra": "off"},
			OnEffect:   "open_effect",
			OffEffect:  "close_effect",
		},
	}
	// Case 1: Open
	haState := map[string]interface{}{
		"state":      "open",
		"attributes": map[string]interface{}{"current_position": 50.0},
	}
	hueState := s.ToHue(haState, vd)
	assert.Equal(t, uint8(127), hueState.Bri)
	assert.True(t, hueState.On)

	// Case 2: Closed
	haState["state"] = "closed"
	assert.False(t, s.ToHue(haState, vd).On)

	// Case 3: Position 100 (explicitly via bri)
	hueState.Bri = 254
	hueState.On = true
	hueState.UpdatedByBri = true
	service, haParams := s.ToHA(hueState, vd)
	assert.Equal(t, "set_cover_position", service)
	assert.Equal(t, 100, haParams["position"])

	// Case 4: Intermediate position
	hueState.Bri = 127
	service, haParams = s.ToHA(hueState, vd)
	assert.Equal(t, 50, haParams["position"])

	// Case 5: Open (via On command)
	hueState.UpdatedByBri = false
	hueState.On = true
	service, haParams = s.ToHA(hueState, vd)
	assert.Equal(t, "set_cover_position", service)
	assert.Equal(t, 100, haParams["position"])
	assert.Equal(t, "on", haParams["extra"])
	assert.Equal(t, "open_effect", haParams["effect"])

	// Case 6: Closed (via Off command)
	hueState.On = false
	service, haParams = s.ToHA(hueState, vd)
	assert.Equal(t, "set_cover_position", service)
	assert.Equal(t, 0, haParams["position"])
	assert.Equal(t, "off", haParams["extra"])
	assert.Equal(t, "close_effect", haParams["effect"])

	// Case 7: Custom services
	vd.ActionConfig.OnService = "cover.custom_on"
	vd.ActionConfig.OffService = "cover.custom_off"
	hueState.On = true
	service, _ = s.ToHA(hueState, vd)
	assert.Equal(t, "cover.custom_on", service)
	hueState.On = false
	service, _ = s.ToHA(hueState, vd)
	assert.Equal(t, "cover.custom_off", service)
}

func TestClimateStrategy(t *testing.T) {
	s := &ClimateStrategy{}
	vd := &model.VirtualDevice{
		Type: model.MappingTypeClimate,
		ActionConfig: &model.ActionConfig{
			OnPayload:  map[string]interface{}{"hvac_mode": "heat"},
			OffPayload: map[string]interface{}{"hvac_mode": "off"},
			OnEffect:   "on_eff",
			OffEffect:  "off_eff",
		},
	}
	// Case 1: HA to Hue
	haState := map[string]interface{}{
		"state":      "heat",
		"attributes": map[string]interface{}{"temperature": 21.0},
	}
	hueState := s.ToHue(haState, vd)
	assert.Equal(t, uint8(169), hueState.Bri)
	assert.True(t, hueState.On)

	// Case 2: Temperature clipping high
	haState["attributes"].(map[string]interface{})["temperature"] = 30.0
	assert.Equal(t, uint8(254), s.ToHue(haState, vd).Bri)

	// Case 3: Temperature clipping low
	haState["attributes"].(map[string]interface{})["temperature"] = 5.0
	assert.Equal(t, uint8(0), s.ToHue(haState, vd).Bri)

	// Case 4: No attributes
	assert.Equal(t, uint8(0), s.ToHue(map[string]interface{}{"state": "heat"}, vd).Bri)

	// Case 5: Hue to HA (On)
	hueState.Bri = 254
	hueState.On = true
	service, haParams := s.ToHA(hueState, vd)
	assert.Equal(t, "set_temperature", service)
	assert.Equal(t, 28.0, haParams["temperature"])
	assert.Equal(t, "heat", haParams["hvac_mode"])
	assert.Equal(t, "on_eff", haParams["effect"])

	// Case 6: Hue to HA (Off)
	hueState.On = false
	service, haParams = s.ToHA(hueState, vd)
	assert.Equal(t, "set_temperature", service)
	assert.Equal(t, "off", haParams["hvac_mode"])
	assert.Equal(t, "off_eff", haParams["effect"])

	// Case 7: Custom services
	vd.ActionConfig.OnService = "climate.custom_on"
	vd.ActionConfig.OffService = "climate.custom_off"
	hueState.On = true
	service, _ = s.ToHA(hueState, vd)
	assert.Equal(t, "climate.custom_on", service)
	hueState.On = false
	service, _ = s.ToHA(hueState, vd)
	assert.Equal(t, "climate.custom_off", service)
}

func TestCustomStrategy(t *testing.T) {
	s := &CustomStrategy{}
	vd := &model.VirtualDevice{
		EntityID: "camera.salon",
		Type:     model.MappingTypeCustom,
		ActionConfig: &model.ActionConfig{
			ToHueFormula: "x * 2.54",
			ToHAFormula:  "x / 2.54",
			OnPayload:    map[string]interface{}{"extra": "on"},
			OffPayload:   map[string]interface{}{"extra": "off"},
			OnService:    "custom.on",
			OffService:   "custom.off",
			OnEffect:     "effect.on",
			OffEffect:    "effect.off",
		},
	}

	haState := map[string]interface{}{
		"state": "on",
		"attributes": map[string]interface{}{
			"brightness": 50.0,
		},
	}
	hueState := s.ToHue(haState, vd)
	assert.Equal(t, uint8(127), hueState.Bri)

	testState := &model.DeviceState{On: true, Bri: 254}
	service, haParams := s.ToHA(testState, vd)
	assert.Equal(t, "custom.on", service)
	assert.Equal(t, "effect.on", haParams["effect"])
	assert.InDelta(t, 100.0, haParams["value"].(float64), 0.1)

	testState.On = false
	service, haParams = s.ToHA(testState, vd)
	assert.Equal(t, "custom.off", service)
	assert.Equal(t, "effect.off", haParams["effect"])

	// Test with NO action config
	vd_no_config := &model.VirtualDevice{EntityID: "light.test"}
	service, haParams = s.ToHA(&model.DeviceState{On: true, Bri: 127}, vd_no_config)
	assert.Equal(t, "turn_on", service)
	assert.Equal(t, 127.0, haParams["brightness"])

	// Test other domains for ToHA default logic
	vd_cover := &model.VirtualDevice{EntityID: "cover.test"}
	service, haParams = s.ToHA(&model.DeviceState{On: true, Bri: 100, UpdatedByBri: true}, vd_cover)
	assert.Equal(t, "set_cover_position", service)
	assert.Equal(t, 100, haParams["position"])

	vd_climate := &model.VirtualDevice{EntityID: "climate.test"}
	service, _ = s.ToHA(&model.DeviceState{On: true, Bri: 254}, vd_climate)
	assert.Equal(t, "set_temperature", service)

	vd_input := &model.VirtualDevice{EntityID: "input_number.test"}
	service, _ = s.ToHA(&model.DeviceState{On: true, Bri: 254}, vd_input)
	assert.Equal(t, "set_value", service)
}

func TestCustomStrategy_Evaluate(t *testing.T) {
	s := &CustomStrategy{}
	assert.Equal(t, 10.0, s.evaluate("x * 2", 5))
	assert.Equal(t, 5.0, s.evaluate("x / 2", 10))
	assert.Equal(t, 5.0, s.evaluate("invalid syntax (", 5)) // Parser error
	assert.Equal(t, 5.0, s.evaluate("x + y", 5))           // Eval error (y missing)
	assert.Equal(t, 5.0, s.evaluate("1 == 1", 5))         // Bool return
	assert.Equal(t, 5.0, s.evaluate("'string'", 5))       // String return
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
	assert.IsType(t, &LightStrategy{}, f.GetTranslator("unknown"))
}

func TestCustomStrategy_ToHue_Fallback(t *testing.T) {
	s := &CustomStrategy{}
	vd := &model.VirtualDevice{EntityID: "other.entity"}

	// Test position fallback
	haState := map[string]interface{}{
		"attributes": map[string]interface{}{"current_position": 127.0},
	}
	hueState := s.ToHue(haState, vd)
	assert.Equal(t, uint8(127), hueState.Bri)

	// Test temperature fallback
	haState = map[string]interface{}{
		"attributes": map[string]interface{}{"temperature": 169.0},
	}
	hueState = s.ToHue(haState, vd)
	assert.Equal(t, uint8(169), hueState.Bri)

	// Test value fallback
	haState = map[string]interface{}{
		"attributes": map[string]interface{}{"value": 50.0},
	}
	hueState = s.ToHue(haState, vd)
	assert.Equal(t, uint8(50), hueState.Bri)
}
