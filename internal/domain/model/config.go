package model

type Config struct {
	HassURL   string `json:"hass_url"`
	HassToken string `json:"hass_token"`
	LocalIP   string `json:"local_ip"`
}
