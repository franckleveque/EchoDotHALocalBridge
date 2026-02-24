# Agent Instructions

## Coding Standards

- **Hexagonal Architecture**: Do not allow infrastructure imports in the domain layer. Use `internal/architecture_test.go` to verify.
- **Library Choice**: Always use `github.com/amimof/huego` for Hue models.
- **Asynchrony**: All calls to Home Assistant must be asynchronous to ensure low latency for the Hue API.
- **Performance**: Keep the binary size and memory usage minimal. Use `scratch` as the base image.

## Testing

- Maintain > 80% coverage in the `internal/domain` package.
- Use mocks for `HomeAssistantPort` when testing domain services.

## Deployment

- Ensure the Kubernetes manifests include `securityContext` with `NET_BIND_SERVICE` capability.
- Use `hostNetwork: true` for SSDP functionality.
