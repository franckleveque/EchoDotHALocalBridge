package model

import "github.com/amimof/huego"

type DeviceType string

const (
	DeviceTypeLight   DeviceType = "light"
	DeviceTypeCover   DeviceType = "cover"
	DeviceTypeClimate DeviceType = "climate"
)

type Device struct {
	ID         string
	Name       string
	Type       DeviceType
	ExternalID string // Home Assistant Entity ID
	State      *huego.State
}
