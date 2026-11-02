package model

type DeviceState struct {
	On        bool      `json:"on"`
	Bri       uint8     `json:"bri"`
	Hue       uint16    `json:"hue"`
	Sat       uint8     `json:"sat"`
	Xy        []float32 `json:"xy"`
	Ct        uint16    `json:"ct"`
	Reachable bool      `json:"reachable"`

	// Helper field to distinguish between direct On/Off vs Brightness update
	UpdatedByBri bool `json:"-"`
}
