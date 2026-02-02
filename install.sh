#!/bin/bash

# ideasbglobot installer
# Usage: curl -sSL https://raw.githubusercontent.com/nathfavour/ideasbglobot/main/install.sh | bash

set -e

APP_NAME="ideasbglobot"
INSTALL_DIR="$HOME/.local/bin"
REPO_URL="https://github.com/nathfavour/ideasbglobot"

# Ensure we are not installing to a restricted system path
if [[ "$INSTALL_DIR" != "$HOME"* ]]; then
    echo "❌ Error: Installation is restricted to the user's home directory ($HOME)."
    exit 1
fi

echo "🚀 Installing $APP_NAME to $INSTALL_DIR..."

# Create install directory if it doesn't exist
mkdir -p "$INSTALL_DIR"

# Check if go is installed
if ! command -v go &> /dev/null; then
    echo "❌ Error: 'go' is not installed. Please install Go (https://golang.org/dl/) and try again."
    exit 1
fi

# Create a persistent source directory for future updates
SRC_DIR="$HOME/.ideasbglobot/src"
mkdir -p "$SRC_DIR"

if [ -d "$SRC_DIR/.git" ]; then
    echo "🔄 Updating existing source..."
    cd "$SRC_DIR"
    git pull
else
    echo "📦 Fetching latest source..."
    rm -rf "$SRC_DIR"
    git clone --depth 1 "$REPO_URL" "$SRC_DIR"
    cd "$SRC_DIR"
fi

echo "🛠 Building..."
go build -o "$APP_NAME" main.go

# Install
echo "📥 Installing to $INSTALL_DIR..."
cp "$APP_NAME" "$INSTALL_DIR/"

echo "✅ $APP_NAME installed successfully!"
echo "Make sure $INSTALL_DIR is in your PATH."
echo "Run '$APP_NAME --help' to get started."
