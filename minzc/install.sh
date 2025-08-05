#!/bin/bash

# MinZ Installation Script
# Installs mz (compiler) and mzr (REPL) to ~/.local/bin

set -e

echo "🚀 Installing MinZ..."

# Create ~/.local/bin directory if it doesn't exist
mkdir -p ~/.local/bin

# Build everything
echo "📦 Building MinZ compiler and REPL..."
make clean

# Try to build both, but don't fail if REPL has issues
echo "Building compiler..."
make build

echo "Building REPL (optional)..."
make repl || echo "⚠️  REPL build failed, but compiler is ready!"

# Copy executables to ~/.local/bin
echo "📂 Installing to ~/.local/bin..."
if [ -f mz ]; then
    cp mz ~/.local/bin/
    chmod +x ~/.local/bin/mz
    echo "✅ Installed mz (compiler)"
fi

if [ -f mzr ]; then
    cp mzr ~/.local/bin/
    chmod +x ~/.local/bin/mzr
    echo "✅ Installed mzr (REPL)"
else
    echo "⚠️  REPL not available (build failed)"
fi

# Check if ~/.local/bin is in PATH
if [[ ":$PATH:" != *":$HOME/.local/bin:"* ]]; then
    echo ""
    echo "⚠️  ~/.local/bin is not in your PATH!"
    echo ""
    echo "Add this line to your ~/.bashrc or ~/.zshrc:"
    echo '  export PATH="$HOME/.local/bin:$PATH"'
    echo ""
    echo "Then run:"
    echo "  source ~/.bashrc  # or source ~/.zshrc"
else
    echo "✅ ~/.local/bin is already in PATH"
fi

echo ""
echo "✨ MinZ installation complete!"
echo ""
echo "Available commands:"
echo "  mz  - MinZ compiler"
if [ -f ~/.local/bin/mzr ]; then
    echo "  mzr - MinZ REPL"
fi
echo ""
echo "Try it out:"
echo "  mz --list-backends"
echo "  mz ../examples/fibonacci.minz -o fibonacci.a80"
if [ -f ~/.local/bin/mzr ]; then
    echo "  mzr  # Start REPL"
fi