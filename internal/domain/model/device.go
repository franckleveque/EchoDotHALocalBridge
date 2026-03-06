package model

type HueMetadata struct {
	Type             string
	ModelID          string
	ManufacturerName string
}

type DeviceState struct {
	On        bool      `json:"on"`
	Bri       uint8     `json:"bri"`
	Hue       uint16    `json:"hue"`
	Sat       uint8     `json:"sat"`
	Xy        []float32 `json:"xy"`
	Ct        uint16    `json:"ct"`
	Reachable bool      `json:"reachable"`
}

type Device struct {
	ID            string
	Name          string
	Type          MappingType
	ExternalID    string // Home Assistant Entity ID
	State         *DeviceState
	VirtualDevice *VirtualDevice
}
