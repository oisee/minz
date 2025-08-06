#!/bin/bash
# MinZ Compiler Installation Script

set -e

echo "MinZ Compiler Installation"
echo "=========================="
echo

# Detect MinZ directory
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
MINZ_DIR="$SCRIPT_DIR"

if [ ! -f "$MINZ_DIR/grammar.js" ]; then
    echo "Error: Cannot find grammar.js in $MINZ_DIR"
    echo "Please run this script from the minz-ts directory."
    exit 1
fi

echo "MinZ directory: $MINZ_DIR"
echo

# Check if tree-sitter is installed
if ! command -v tree-sitter &> /dev/null; then
    echo "⚠️  tree-sitter is not installed!"
    echo
    echo "MinZ requires tree-sitter to parse source files."
    echo "Would you like to install it now? (y/n)"
    read -r response
    
    if [[ "$response" == "y" || "$response" == "Y" ]]; then
        # Check if npm is available
        if command -v npm &> /dev/null; then
            echo "Installing tree-sitter via npm..."
            npm install -g tree-sitter-cli
        # Check if brew is available (macOS)
        elif command -v brew &> /dev/null; then
            echo "Installing tree-sitter via Homebrew..."
            brew install tree-sitter
        else
            echo "Error: Neither npm nor Homebrew is available."
            echo "Please install tree-sitter manually:"
            echo "  https://github.com/tree-sitter/tree-sitter/tree/master/cli"
            exit 1
        fi
    else
        echo "Installation cancelled."
        echo "Please install tree-sitter before using MinZ:"
        echo "  npm install -g tree-sitter-cli"
        echo "  OR"
        echo "  brew install tree-sitter"
        exit 1
    fi
else
    echo "✅ tree-sitter is already installed: $(which tree-sitter)"
fi

echo
echo "Building MinZ compiler..."
cd "$MINZ_DIR/minzc" && make build

echo
echo "Installation Options:"
echo "1) System-wide install to /usr/local/bin (requires sudo)"
echo "2) User install to ~/.local/bin (no sudo required)"
echo "3) Development mode (use from current directory)"
echo
echo -n "Choose installation type (1/2/3): "
read -r install_choice

case "$install_choice" in
    1)
        echo "Installing to /usr/local/bin..."
        
        # Create the actual mz binary wrapper
        sudo tee /usr/local/bin/mz >/dev/null <<EOF
#!/bin/bash
# MinZ compiler wrapper
export MINZ_GRAMMAR="$MINZ_DIR"
exec "$MINZ_DIR/minzc/mz" "\$@"
EOF
        sudo chmod +x /usr/local/bin/mz
        sudo ln -sf /usr/local/bin/mz /usr/local/bin/minzc
        
        echo "✅ MinZ installed to /usr/local/bin/mz"
        echo "   You can now use 'mz' from anywhere!"
        ;;
        
    2)
        echo "Installing to ~/.local/bin..."
        mkdir -p ~/.local/bin
        
        # Create the wrapper script
        cat > ~/.local/bin/mz <<EOF
#!/bin/bash
# MinZ compiler wrapper
export MINZ_GRAMMAR="$MINZ_DIR"
exec "$MINZ_DIR/minzc/mz" "\$@"
EOF
        chmod +x ~/.local/bin/mz
        ln -sf ~/.local/bin/mz ~/.local/bin/minzc
        
        echo "✅ MinZ installed to ~/.local/bin/mz"
        echo
        
        # Check if ~/.local/bin is in PATH
        if [[ ":$PATH:" != *":$HOME/.local/bin:"* ]]; then
            echo "⚠️  ~/.local/bin is not in your PATH!"
            echo
            echo "Add this to your shell config (~/.bashrc or ~/.zshrc):"
            echo '    export PATH="$HOME/.local/bin:$PATH"'
            echo
            echo "Then reload your shell or run:"
            echo '    source ~/.bashrc  # or ~/.zshrc'
        else
            echo "   You can now use 'mz' from anywhere!"
        fi
        ;;
        
    3)
        echo "Development mode - creating aliases..."
        echo
        echo "Add these lines to your shell config (~/.bashrc or ~/.zshrc):"
        echo
        echo "    # MinZ compiler"
        echo "    export MINZ_GRAMMAR=\"$MINZ_DIR\""
        echo "    alias mz=\"$MINZ_DIR/minzc/mz\""
        echo "    alias minzc=\"$MINZ_DIR/minzc/mz\""
        echo
        echo "Then reload your shell or run:"
        echo '    source ~/.bashrc  # or ~/.zshrc'
        ;;
        
    *)
        echo "Invalid choice. Installation cancelled."
        exit 1
        ;;
esac

echo
echo "================================"
echo "Installation complete! 🎉"
echo "================================"
echo
echo "Test your installation:"
echo "  mz --version"
echo
echo "Compile a MinZ program:"
echo "  mz examples/simple_add.minz -o test.a80"
echo
echo "MinZ compiler is ready to use!"