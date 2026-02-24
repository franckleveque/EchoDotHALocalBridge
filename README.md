# Hue Bridge Emulator for Home Assistant

This project is a high-performance, lightweight Hue V1 bridge emulator designed to interface an Echo Dot 3 (or other Alexa devices) with Home Assistant.

## Features

- **Hexagonal Architecture**: Strict isolation between domain logic, ports, and adapters.
- **Translation Engine**: Factory + Strategy pattern for Light, Cover, and Climate (7-28°C range).
- **SSDP Discovery**: Automatic discovery by Alexa devices.
- **Lightweight**: Multi-arch Docker `scratch` image with < 20MB RAM usage.
- **Async HA Calls**: Non-blocking calls to Home Assistant REST API.
- **High Coverage**: > 90% domain logic coverage.
- **Security**: Runs as non-root with minimal capabilities (`CAP_NET_BIND_SERVICE`).

## Architecture

The project follows the Hexagonal Architecture pattern:
- `internal/domain`: Core business logic and entities.
- `internal/ports`: Interface definitions for input and output.
- `internal/adapters`: Implementations of Hue API, SSDP, and Home Assistant client.

## Deployment on Talos / Kubernetes

The emulator requires `hostNetwork: true` for SSDP multicast discovery to work correctly.

### Prerequisites

- A Home Assistant instance with a Long-Lived Access Token.
- A Kubernetes cluster (optimized for Talos).

### Configuration

Set the following environment variables:
- `HASS_URL`: Your Home Assistant URL (e.g., `http://192.168.1.10:8123`).
- `HASS_TOKEN`: Your Home Assistant Long-Lived Access Token.
- `LOCAL_IP`: (Optional) The IP address of the node. If not set, it will be automatically detected.

## CI/CD

The project includes a GitHub Actions pipeline that:
- Runs Unit and ArchUnit tests.
- Enforces > 80% domain code coverage.
- Builds a multi-arch static binary and Docker image.
- Performs Trivy vulnerability scanning.

## Development

```bash
# Run tests
make test

# Build binary
make build

# Check coverage
make coverage
```
