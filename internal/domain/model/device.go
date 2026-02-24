package model

import "github.com/amimof/huego"

type Device struct {
	ID         string
	Name       string
	Type       MappingType
	ExternalID string // Home Assistant Entity ID
	State      *huego.State
	Mapping    *EntityMapping
}
