package ports

// Reconfigurable defines an interface for ports that can be reconfigured at runtime
type Reconfigurable interface {
	Configure(url, token string)
}
