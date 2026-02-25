# Hue Bridge Emulator for Home Assistant (SOLID Edition)

This bridge allows emulating a Philips Hue V1 Bridge to expose Home Assistant entities to Alexa (Echo Dot 3).

## 🚀 Key Features

- **Hexagonal Architecture**: Strict isolation between business logic and infrastructure.
- **SOLID Compliance**: High modularity, dependency inversion, and single responsibility.
- **Dynamic Entity Discovery**: Fetch all HA entities (lights, covers, climate, switches, input_numbers, groups).
- **Flexible Mapping**: Choose which entities to expose and how.
- **Custom Translation Engine**: Define your own conversion formulas (linear mapping) for non-standard devices.
- **Multi-arch Support**: Docker images for amd64 and arm64.
- **High Performance**: Asynchronous calls to Home Assistant, minimal footprint (< 20MB RAM).

## 🛠 Admin Interface

Access the admin UI at `http://<IP>/admin`.
- **General Config**: Set Home Assistant URL and Token.
- **Virtual Devices**:
  - Define "Virtual Intentions" for any Home Assistant entity.
  - **Custom Actions**: Manually specify HA services (e.g., `script.my_script`) and JSON payloads for ON/OFF commands.
  - **Formula Engine**: Use `x` as a variable to define linear mapping between Hue (0-254) and HA values.
  - **Metadata**: Select device type (Light, Cover, Climate, Custom) to ensure correct Alexa icons and behavior.

## 📐 Architecture & SOLID

- **Single Responsibility**: Each strategy handles one type of conversion. The bridge service only coordinates.
- **Open/Closed**: New device types can be added by implementing the `Translator` interface and registering them in the `Factory` without modifying existing logic.
- **Liskov Substitution**: All strategies are interchangeable via the `Translator` interface.
- **Interface Segregation**: Ports define minimal required interfaces for the domain.
- **Dependency Inversion**: Domain services depend on interfaces (ports), not concrete adapter implementations.

## 🧪 Testing

```bash
make test
```
ArchUnit is used to enforce architectural boundaries. Domain coverage is strictly monitored (> 80%).

## 📦 Deployment

Optimized for **Talos Cluster**:
- `hostNetwork: true` for SSDP.
- `CAP_NET_BIND_SERVICE` for port 80.
- `scratch` base image for security.
