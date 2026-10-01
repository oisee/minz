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
cp mz "$LOCAL_BIN/"
cp mzr "$LOCAL_BIN/"
# Create compatibility symlinks
ln -sf "$LOCAL_BIN/mz" "$LOCAL_BIN/minzc"
ln -sf "$LOCAL_BIN/mzr" "$LOCAL_BIN/minz"

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
echo "  mz  <file.minz>  - Compile MinZ source to Z80 assembly"
echo "  mzr              - Start MinZ REPL (interactive mode)"
echo
echo "Also available for compatibility:"
echo "  minzc  - Same as mz"
echo "  minz   - Same as mzr"
echo
echo "Try it now:"
echo "  mzr"
echo
echo "Type /h for help, /q to quit"