package persistence

// Internal structure for migration
type legacyConfig struct {
	HassURL        string                          `json:"hass_url"`
	HassToken      string                          `json:"hass_token"`
	EntityMappings map[string]*legacyEntityMapping `json:"entity_mappings"`
}
