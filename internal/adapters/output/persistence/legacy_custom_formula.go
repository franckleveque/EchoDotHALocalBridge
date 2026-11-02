package persistence

type legacyCustomFormula struct {
	ToHueFormula string `json:"to_hue_formula"`
	ToHAFormula  string `json:"to_ha_formula"`
	OnService    string `json:"on_service"`
	OffService   string `json:"off_service"`
	OnEffect     string `json:"on_effect"`
	OffEffect    string `json:"off_effect"`
}
