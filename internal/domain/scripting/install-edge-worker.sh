#!/bin/bash
set -euo pipefail

# Define the repository and binary name
REPO="stroppy-io/hatchet-workflow"
BINARY_NAME="edge-worker"
INSTALL_DIR="/usr/local/bin"

ensure_command() {
  local cmd="$1"
  if ! command -v "$cmd" >/dev/null 2>&1; then
    return 1
  fi
  return 0
}

install_packages_apt() {
  local pkgs=("$@")
  if [ -w "/var/lib/apt/lists" ]; then
    apt-get update -y >/dev/null
    apt-get install -y "${pkgs[@]}" >/dev/null
  else
    sudo apt-get update -y >/dev/null
    sudo apt-get install -y "${pkgs[@]}" >/dev/null
  fi
}

# Ensure required dependencies are installed (Ubuntu/Debian via apt-get)
if ! ensure_command curl || ! ensure_command systemctl || ! ensure_command bash; then
  if ensure_command apt-get; then
    install_packages_apt bash curl ca-certificates systemd
  else
    echo "Error: missing required tools (bash, curl, systemctl) and apt-get not found."
    exit 1
  fi
fi

if ! ensure_command curl || ! ensure_command systemctl; then
  echo "Error: required tools are not available after install."
  exit 1
fi

# Get the latest release tag (including prerelease) from /releases
LATEST_TAG=$(curl -s "https://api.github.com/repos/$REPO/releases" | grep -m1 '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')

if [ -z "$LATEST_TAG" ]; then
  echo "Error: Could not fetch the latest release tag."
  exit 1
fi

echo "Latest release: $LATEST_TAG"

# Construct the download URL
DOWNLOAD_URL="https://github.com/$REPO/releases/download/$LATEST_TAG/${BINARY_NAME}"

echo "Downloading $BINARY_NAME from $DOWNLOAD_URL..."

# Download the binary
curl -L -o "$BINARY_NAME" "$DOWNLOAD_URL"

if [ $? -ne 0 ]; then
  echo "Error: Failed to download the binary."
  exit 1
fi

# Make it executable
chmod +x "$BINARY_NAME"

# Move to install directory (requires sudo if not root)
if [ -w "$INSTALL_DIR" ]; then
    mv "$BINARY_NAME" "$INSTALL_DIR/$BINARY_NAME"
    echo "Successfully installed $BINARY_NAME to $INSTALL_DIR"
else
    echo "Installing to $INSTALL_DIR requires sudo..."
    sudo mv "$BINARY_NAME" "$INSTALL_DIR/$BINARY_NAME"
    if [ $? -eq 0 ]; then
        echo "Successfully installed $BINARY_NAME to $INSTALL_DIR"
    else
        echo "Error: Failed to move binary to $INSTALL_DIR"
        exit 1
    fi
fi

# --- Systemd Service Setup ---

CONFIG_DIR="/etc/hatchet"
ENV_FILE="$CONFIG_DIR/edge-worker.env"
SERVICE_FILE="/etc/systemd/system/hatchet-edge-worker.service"

echo "Setting up systemd service..."

# Create config directory
if [ ! -d "$CONFIG_DIR" ]; then
    if [ -w "/etc" ]; then
        mkdir -p "$CONFIG_DIR"
    else
        sudo mkdir -p "$CONFIG_DIR"
    fi
fi

# Ensure env file exists (it might have been created by the wrapper script)
if [ ! -f "$ENV_FILE" ]; then
    if [ -w "$CONFIG_DIR" ]; then
        touch "$ENV_FILE"
    else
        sudo touch "$ENV_FILE"
    fi
fi

# Append arguments to env file
if [ $# -gt 0 ]; then
    echo "Adding environment variables to $ENV_FILE..."
    for ARG in "$@"; do
        # Check if argument is a variable assignment
        if [[ "$ARG" =~ ^[A-Za-z0-9_]+=.*$ ]]; then
            if [ -w "$ENV_FILE" ]; then
                echo "$ARG" >> "$ENV_FILE"
            else
                echo "$ARG" | sudo tee -a "$ENV_FILE" > /dev/null
            fi
        fi
    done
fi

# Define Service Content
# Note: ExecStart assumes the binary takes a 'start' command. Adjust if necessary.
SERVICE_CONTENT="[Unit]
Description=Hatchet Edge Worker
After=network.target

[Service]
Type=simple
ExecStart=$INSTALL_DIR/$BINARY_NAME
Restart=always
RestartSec=5
EnvironmentFile=$ENV_FILE

[Install]
WantedBy=multi-user.target"

# Write Service File
if [ -w "/etc/systemd/system" ]; then
    echo "$SERVICE_CONTENT" > "$SERVICE_FILE"
    systemctl daemon-reload
    systemctl enable hatchet-edge-worker
    systemctl start hatchet-edge-worker
else
    echo "$SERVICE_CONTENT" | sudo tee "$SERVICE_FILE" > /dev/null
    sudo systemctl daemon-reload
    sudo systemctl enable hatchet-edge-worker
    sudo systemctl start hatchet-edge-worker
fi

echo "Hatchet Edge Worker service installed and started."
