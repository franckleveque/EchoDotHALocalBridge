package http

import "hue-bridge-emulator/internal/domain/model"

// parseStateUpdate builds a DeviceState from a decoded JSON state object.
// "bri" also sets UpdatedByBri, which cannot be carried by JSON.
func parseStateUpdate(raw map[string]any) *model.DeviceState {
	state := &model.DeviceState{}
	if on, ok := raw["on"].(bool); ok {
		state.On = on
	}
	if bri, ok := raw["bri"].(float64); ok {
		state.Bri = uint8(bri)
		state.UpdatedByBri = true
	}
	return state
}
