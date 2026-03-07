# Hue Bridge Emulator RPi Installation Script (PowerShell 5.1+)
# This script builds the Docker image locally and transfers it to the RPi.

$ErrorActionPreference = "Stop"

Write-Host "--- Hue Bridge Emulator RPi Installer ---" -ForegroundColor Cyan

# 1. SSH Credentials
$RPi_IP = Read-Host "Enter Raspberry Pi IP address"
$RPi_User = Read-Host "Enter Raspberry Pi username"
if ([string]::IsNullOrWhiteSpace($RPi_User)) { $RPi_User = "pi" }

# 2. Local Build
Write-Host "`n[1/5] Building Docker image locally (linux/arm64)..." -ForegroundColor Yellow
# Ensure we are in the root directory
$RepoRoot = Resolve-Path "$PSScriptRoot\.."
Push-Location $RepoRoot

# For RPi 3/4/5 with 64-bit OS, linux/arm64 is recommended.
# For older RPi or 32-bit OS, use --platform linux/arm/v7
docker buildx build --platform linux/arm64 -t hue-bridge-emulator:latest --load .

Write-Host "[2/5] Exporting image to tar file..." -ForegroundColor Yellow
if (Test-Path "hue-bridge-emulator.tar") { Remove-Item "hue-bridge-emulator.tar" }
docker save hue-bridge-emulator:latest -o hue-bridge-emulator.tar

# 3. Transfer to RPi
Write-Host "[3/5] Transferring image and docker-compose to RPi..." -ForegroundColor Yellow
# Use scp (standard in Windows 10/11)
scp hue-bridge-emulator.tar docker-compose.yml "$($RPi_User)@$($RPi_IP):/home/$($RPi_User)/"

# 4. SSH Commands to Setup RPi
Write-Host "[4/5] Running setup commands on RPi..." -ForegroundColor Yellow

$SetupCommands = @"
# Check for Docker
if ! command -v docker &> /dev/null; then
    echo 'Installing Docker...'
    curl -sSL https://get.docker.com | sh
    sudo usermod -aG docker \$USER
fi

# Check for Docker Compose plugin
if ! docker compose version &> /dev/null; then
    echo 'Installing Docker Compose plugin...'
    sudo apt update && sudo apt install -y docker-compose-plugin
fi

# Load the image (use sudo to avoid group permission issues on first run)
echo 'Loading Docker image...'
sudo docker load -i /home/$($RPi_User)/hue-bridge-emulator.tar
rm /home/$($RPi_User)/hue-bridge-emulator.tar

# Prepare directory
mkdir -p /home/$($RPi_User)/hue-bridge
if [ -f /home/$($RPi_User)/docker-compose.yml ]; then
    mv /home/$($RPi_User)/docker-compose.yml /home/$($RPi_User)/hue-bridge/
fi
mkdir -p /home/$($RPi_User)/hue-bridge/data

# Run (use sudo for the same reason)
cd /home/$($RPi_User)/hue-bridge
sudo docker compose up -d
"@

# Note: This will prompt for password unless SSH keys are set up
ssh "$($RPi_User)@$($RPi_IP)" "bash -c \"$SetupCommands\""

# 5. Cleanup
Pop-Location
if (Test-Path "$RepoRoot\hue-bridge-emulator.tar") {
    Remove-Item "$RepoRoot\hue-bridge-emulator.tar"
}

Write-Host "`n[5/5] Installation complete!" -ForegroundColor Green
Write-Host "Access the Admin UI at http://$($RPi_IP)/admin" -ForegroundColor Cyan
Write-Host "Note: Home Assistant configuration is handled in the Settings section of the Admin UI." -ForegroundColor Gray
