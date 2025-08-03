#!/bin/bash

# MinZ Development Installation Script

echo "╔══════════════════════════════════════════════════════════════╗"
echo "║               MinZ Development Installation                  ║"
echo "╚══════════════════════════════════════════════════════════════╝"
echo

# Check if running from project root
if [ ! -f "README.md" ] || [ ! -d "minzc" ]; then
    echo "Error: Please run this script from the MinZ project root directory"
    exit 1
fi

cd minzc

# Build everything
echo "Building MinZ compiler and REPL..."
make clean >/dev/null 2>&1
make all

if [ $? -ne 0 ]; then
    echo "Build failed!"
    exit 1
fi

# Check for local bin directory
LOCAL_BIN="$HOME/.local/bin"
if [ ! -d "$LOCAL_BIN" ]; then
    echo "Creating $LOCAL_BIN directory..."
    mkdir -p "$LOCAL_BIN"
fi

# Install to local bin
echo "Installing to $LOCAL_BIN..."
cp minzc "$LOCAL_BIN/"
cp minz "$LOCAL_BIN/"

# Check if local bin is in PATH
if [[ ":$PATH:" != *":$LOCAL_BIN:"* ]]; then
    echo
    echo "⚠️  Note: $LOCAL_BIN is not in your PATH"
    echo "Add this to your shell configuration file (.bashrc, .zshrc, etc.):"
    echo
    echo "export PATH=\"\$HOME/.local/bin:\$PATH\""
    echo
fi

echo "✅ Installation complete!"
echo
echo "Available commands:"
echo "  minzc <file.minz>  - Compile MinZ source to Z80 assembly"
echo "  minz               - Start MinZ REPL (interactive mode)"
echo
echo "Try it now:"
echo "  minz"
echo
echo "Type /h for help, /q to quit"