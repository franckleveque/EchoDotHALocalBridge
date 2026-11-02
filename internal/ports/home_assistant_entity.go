package ports

type HomeAssistantEntity struct {
	EntityID     string `json:"entity_id"`
	FriendlyName string `json:"friendly_name"`
}
