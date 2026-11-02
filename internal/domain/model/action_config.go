package model

type ActionConfig struct {
	// Conversion values for DIM (brightness Hue <-> value HA)
	ToHueFormula string `json:"to_hue_formula,omitempty"`
	ToHAFormula  string `json:"to_ha_formula,omitempty"`

	// ON Actions
	OnService string                 `json:"on_service,omitempty"`
	OnPayload map[string]interface{} `json:"on_payload,omitempty"` // Static params
	OnEffect  string                 `json:"on_effect,omitempty"`
	NoOpOn    bool                   `json:"no_op_on,omitempty"`

	// OFF Actions
	OffService string                 `json:"off_service,omitempty"`
	OffPayload map[string]interface{} `json:"off_payload,omitempty"` // Static params
	OffEffect  string                 `json:"off_effect,omitempty"`
	NoOpOff    bool                   `json:"no_op_off,omitempty"`

	// Options
	OmitEntityID bool `json:"omit_entity_id,omitempty"` // For scripts, notify.*
}
