#!/bin/bash

# ideasbglobot installer
# Usage: curl -sSL https://raw.githubusercontent.com/nathfavour/ideasbglobot/main/install.sh | bash

set -e

APP_NAME="ideasbglobot"
INSTALL_DIR="$HOME/.local/bin"
REPO_URL="https://github.com/nathfavour/ideasbglobot"

echo "🚀 Installing $APP_NAME..."

# Create install directory if it doesn't exist
mkdir -p "$INSTALL_DIR"

# Check if go is installed
if ! command -v go &> /dev/null; then
    echo "❌ Error: 'go' is not installed. Please install Go (https://golang.org/dl/) and try again."
    exit 1
fi

# Create a temporary directory for building
TMP_DIR=$(mktemp -d)
cd "$TMP_DIR"

# Clone and build
echo "📦 Fetching latest source..."
git clone --depth 1 "$REPO_URL" .
echo "🛠 Building..."
go build -o "$APP_NAME" main.go

# Install
echo "📥 Installing to $INSTALL_DIR..."
mv "$APP_NAME" "$INSTALL_DIR/"

# Cleanup
cd "$HOME"
rm -rf "$TMP_DIR"

echo "✅ $APP_NAME installed successfully!"
echo "Make sure $INSTALL_DIR is in your PATH."
echo "Run '$APP_NAME --help' to get started."
