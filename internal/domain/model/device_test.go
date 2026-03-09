package model

import (
	"testing"
	"github.com/stretchr/testify/assert"
)

func TestHAEntityState_IsSupported(t *testing.T) {
	ignored := []string{"zone.", "sun.", "weather."}

	tests := []struct {
		name     string
		entityID string
		expected bool
	}{
		{"Light", "light.test", true},
		{"Switch", "switch.test", true},
		{"Zone (ignored)", "zone.home", false},
		{"Sun (ignored)", "sun.sun", false},
		{"Weather (ignored)", "weather.london", false},
		{"No dot", "invalid_entity", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := HAEntityState{EntityID: tt.entityID}
			assert.Equal(t, tt.expected, s.IsSupported(ignored))
		})
	}
}
