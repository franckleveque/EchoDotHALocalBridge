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

## 📦 Deployment & Installation (Raspberry Pi 3)

The bridge is designed to run on a dedicated **Raspberry Pi 3** to avoid port 80 conflicts (common in Kubernetes/Talos clusters) and to support SSDP discovery via host networking.

### 🛠 Phase 1: OS Installation

1.  **Download [Raspberry Pi Imager](https://www.raspberrypi.com/software/)**.
2.  **Insert your SD Card** into your computer.
3.  **Choose OS**: Select `Raspberry Pi OS Lite (64-bit)` for a headless setup.
4.  **Configuration**:
    - Click the Cog icon (Advanced options).
    - Set hostname (e.g., `hue-bridge.local`).
    - Enable SSH with password or authorized keys.
    - Configure your Wi-Fi (if not using Ethernet).
5.  **Write**: Flash the SD card and insert it into your Raspberry Pi 3.

### 🐋 Phase 2: Docker Setup

Once logged into your RPi via SSH:

```bash
# Update system
sudo apt update && sudo apt upgrade -y

# Install Docker
curl -sSL https://get.docker.com | sh

# Add your user to the docker group
sudo usermod -aG docker $USER
# (Log out and back in for this to take effect)

# Install Docker Compose
sudo apt install -y docker-compose-plugin
```

### 🚀 Phase 3: Bridge Deployment

1.  **Clone the repository**:
    ```bash
    git clone https://github.com/your-repo/hue-bridge-emulator.git
    cd hue-bridge-emulator
    ```
2.  **Configure environment**:
    Create a `.env` file:
    ```bash
    HASS_URL=http://<YOUR_HA_IP>:8123
    HASS_TOKEN=your_long_lived_access_token
    ```
3.  **Deploy**:
    ```bash
    docker compose up -d
    ```

### ⚠️ Important Notes
- **Port 80**: The bridge **must** use port 80 for Alexa discovery. Ensure no other service (Nginx, Apache, etc.) is running on your RPi.
- **Network**: The container uses `network_mode: host` for SSDP. This is mandatory for discovery to work.

### 💻 Testing on Mac/Windows (Docker Desktop)

On Mac and Windows, Docker runs in a virtual machine. This means `network_mode: host` **does not map to localhost** and Alexa discovery (SSDP) will not work.

To test the Web Admin and API on your computer:

```bash
# Use the windows-specific compose file
docker compose -f docker-compose.windows.yml up -d
```

You can then access the admin panel at: `http://localhost/admin`.

> **Note**: For actual production deployment on a Raspberry Pi, use the standard `docker compose up -d` which uses `network_mode: host`.

### 🔍 Troubleshooting

- **ERR_CONNECTION_REFUSED**:
    - The server listens on `0.0.0.0:80` (all interfaces). If you get this error, it's likely a firewall (UFW) or port conflict.
    - Ensure no other service is using port 80: `sudo lsof -i :80`
    - Check the bridge logs: `docker compose logs -f`
    - **Crucial**: Verify the "Automatically discovered local IP" in logs. If it says `192.168.65.6` but your network is `192.168.1.x`, the bridge is advertising the wrong address via SSDP.
    - **Fix**: Manually set your real RPi IP in `.env`: `LOCAL_IP=192.168.1.XX` (then update `docker-compose.yml` environment if necessary) OR use `PREFERRED_NETWORK=192.168.1.0/24`.
- **Alexa cannot find the bridge**:
    - Ensure your Echo device and RPi are on the same subnet/VLAN.
    - Host networking is mandatory (`network_mode: host` in `docker-compose.yml`).
    - Verify SSDP is not blocked by a firewall (UFW): `sudo ufw allow 1900/udp`
