#!/bin/bash
echo "Installing MinZ v0.13.0 'Module Revolution'..."
echo ""

# Check for sudo/admin
if [ "$EUID" -ne 0 ] && ! command -v sudo &> /dev/null; then
    echo "Warning: Not running as root and sudo not available."
    echo "Installing to ~/bin instead of /usr/local/bin"
    mkdir -p ~/bin
    cp bin/mz ~/bin/
    echo "MinZ installed to ~/bin/mz"
    echo "Add ~/bin to your PATH if not already included."
else
    if [ "$EUID" -ne 0 ]; then
        sudo cp bin/mz /usr/local/bin/
        sudo chmod +x /usr/local/bin/mz
    else
        cp bin/mz /usr/local/bin/
        chmod +x /usr/local/bin/mz
    fi
    echo "MinZ installed to /usr/local/bin/mz"
fi

echo ""
echo "Installation complete! 🎉"
echo ""
echo "Try these commands:"
echo "  mz --version              # Check version"
echo "  mz examples/fibonacci.minz -o fib.a80  # Compile example"
echo ""
echo "Module system examples:"
echo "  import std;               # Standard library"
echo "  import math as m;         # Module aliasing"
echo "  import zx.screen as gfx;  # Platform modules"
echo ""
echo "Happy coding with MinZ! 🚀"
