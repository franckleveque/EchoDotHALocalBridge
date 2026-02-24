package translator

import (
	"testing"
	"github.com/stretchr/testify/assert"
)

func TestLightStrategy(t *testing.T) {
	s := &LightStrategy{}

	// HA to Hue
	haState := map[string]interface{}{
		"state": "on",
		"attributes": map[string]interface{}{
			"brightness": 127.0,
		},
	}
	hueState := s.ToHue(haState)
	assert.True(t, hueState.On)
	assert.Equal(t, uint8(127), hueState.Bri)

	// Hue to HA
	hueState.Bri = 200
	haParams := s.ToHA(hueState)
	assert.Equal(t, uint8(200), haParams["brightness"])
}

func TestCoverStrategy(t *testing.T) {
	s := &CoverStrategy{}

	// HA to Hue
	haState := map[string]interface{}{
		"state": "open",
		"attributes": map[string]interface{}{
			"current_position": 50.0,
		},
	}
	hueState := s.ToHue(haState)
	assert.True(t, hueState.On)
	assert.Equal(t, uint8(127), hueState.Bri) // 50 * 2.54

	// Hue to HA
	hueState.Bri = 254
	haParams := s.ToHA(hueState)
	assert.Equal(t, 100, haParams["position"])
}

func TestClimateStrategy(t *testing.T) {
	s := &ClimateStrategy{}

	// HA to Hue
	haState := map[string]interface{}{
		"attributes": map[string]interface{}{
			"temperature": 21.0,
		},
	}
	hueState := s.ToHue(haState)
	assert.Equal(t, uint8(169), hueState.Bri) // (21-7)*254/21 = 14*254/21 = 2/3 * 254 = 169.33

	// Hue to HA
	hueState.Bri = 254
	haParams := s.ToHA(hueState)
	assert.Equal(t, 28.0, haParams["temperature"])
}

func TestFactory(t *testing.T) {
	f := NewFactory()
	assert.IsType(t, &LightStrategy{}, f.GetTranslator("light"))
	assert.IsType(t, &CoverStrategy{}, f.GetTranslator("cover"))
	assert.IsType(t, &ClimateStrategy{}, f.GetTranslator("climate"))
}
