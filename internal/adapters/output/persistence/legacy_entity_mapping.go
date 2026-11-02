package persistence

import "hue-bridge-emulator/internal/domain/model"

type legacyEntityMapping struct {
	EntityID      string               `json:"entity_id"`
	HueID         string               `json:"hue_id"`
	Name          string               `json:"name"`
	Type          model.MappingType    `json:"type"`
	Exposed       bool                 `json:"exposed"`
	CustomFormula *legacyCustomFormula `json:"custom_formula,omitempty"`
}
