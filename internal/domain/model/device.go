package model

type Device struct {
	ID            string
	Name          string
	Type          MappingType
	ExternalID    string // Home Assistant Entity ID
	State         *DeviceState
	VirtualDevice *VirtualDevice
}
