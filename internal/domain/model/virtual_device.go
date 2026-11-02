package model

type VirtualDevice struct {
	HueID        string        `json:"hue_id"`    // Stable Hue identifier, e.g., "1"
	Name         string        `json:"name"`      // Displayed in Alexa
	EntityID     string        `json:"entity_id"` // HA entity reference
	Type         MappingType   `json:"type"`
	ActionConfig *ActionConfig `json:"action_config,omitempty"`
}
