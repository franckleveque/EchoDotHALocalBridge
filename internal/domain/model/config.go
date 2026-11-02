package model

type Config struct {
	HassURL             string           `json:"hass_url"`
	HassToken           string           `json:"hass_token,omitempty"`
	HassTokenConfigured bool             `json:"-"`
	LocalIP             string           `json:"local_ip"`
	VirtualDevices      []*VirtualDevice `json:"virtual_devices"` // Ordered slice
}
