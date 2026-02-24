package model

type MappingType string

const (
	MappingTypeLight   MappingType = "light"
	MappingTypeCover   MappingType = "cover"
	MappingTypeClimate MappingType = "climate"
	MappingTypeCustom  MappingType = "custom"
)

type CustomFormula struct {
	ToHueFormula string `json:"to_hue_formula"` // e.g. "x * 2.54"
	ToHAFormula  string `json:"to_ha_formula"`  // e.g. "x / 2.54"
	OnEffect     string `json:"on_effect"`     // optional extra command
	OffEffect    string `json:"off_effect"`    // optional extra command
}

type EntityMapping struct {
	EntityID      string         `json:"entity_id"`
	HueID         string         `json:"hue_id"`
	Name          string         `json:"name"`
	Type          MappingType    `json:"type"`
	Exposed       bool           `json:"exposed"`
	CustomFormula *CustomFormula `json:"custom_formula,omitempty"`
}

type Config struct {
	HassURL        string                    `json:"hass_url"`
	HassToken      string                    `json:"hass_token"`
	LocalIP        string                    `json:"local_ip"`
	EntityMappings map[string]*EntityMapping `json:"entity_mappings"` // Keyed by EntityID
}
