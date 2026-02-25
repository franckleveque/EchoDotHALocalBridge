package model

import "github.com/amimof/huego"

type HueMetadata struct {
	Type             string
	ModelID          string
	ManufacturerName string
}

type Device struct {
	ID            string
	Name          string
	Type          MappingType
	ExternalID    string // Home Assistant Entity ID
	State         *huego.State
	VirtualDevice *VirtualDevice
}
