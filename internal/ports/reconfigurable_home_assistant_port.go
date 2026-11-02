package ports

// ReconfigurableHomeAssistantPort defines an interface for HomeAssistant ports that can be reconfigured at runtime
type ReconfigurableHomeAssistantPort interface {
	HomeAssistantPort
	Configure(url, token string)
}
