package model

type AuthConfig struct {
	Username string `json:"username"`
	Password string `json:"password"` // Hashed
}
