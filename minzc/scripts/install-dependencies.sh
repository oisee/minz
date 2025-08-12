#!/bin/bash

# MinZ Dependencies Installer
# Helps install tree-sitter CLI which is required for full MinZ functionality

echo "═══════════════════════════════════════════════════════"
echo "          MinZ Dependencies Installer v0.13.1          "
echo "═══════════════════════════════════════════════════════"
echo ""

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Detect OS
OS="unknown"
if [[ "$OSTYPE" == "linux-gnu"* ]]; then
    OS="linux"
    # Detect Linux distribution
    if [ -f /etc/debian_version ]; then
        DISTRO="debian"
    elif [ -f /etc/redhat-release ]; then
        DISTRO="redhat"
    elif [ -f /etc/arch-release ]; then
        DISTRO="arch"
    else
        DISTRO="unknown"
    fi
elif [[ "$OSTYPE" == "darwin"* ]]; then
    OS="macos"
elif [[ "$OSTYPE" == "msys" || "$OSTYPE" == "cygwin" || "$OSTYPE" == "win32" ]]; then
    OS="windows"
fi

echo "Detected OS: $OS"
if [ "$OS" == "linux" ]; then
    echo "Linux distribution: $DISTRO"
fi
echo ""

# Check if tree-sitter is already installed
if command -v tree-sitter &> /dev/null; then
    echo -e "${GREEN}✅ tree-sitter CLI is already installed${NC}"
    tree-sitter --version
    echo ""
    echo "MinZ is ready to use!"
    exit 0
fi

echo -e "${YELLOW}⚠️ tree-sitter CLI is not installed${NC}"
echo "MinZ requires tree-sitter for parsing source files."
echo ""

# Check for Node.js/npm
if ! command -v npm &> /dev/null; then
    echo -e "${RED}❌ npm is not installed${NC}"
    echo ""
    echo "Installing npm first..."
    
    case $OS in
        linux)
            case $DISTRO in
                debian)
                    echo "For Ubuntu/Debian:"
                    echo "  sudo apt-get update"
                    echo "  sudo apt-get install -y nodejs npm"
                    
                    read -p "Do you want to run these commands now? (y/n) " -n 1 -r
                    echo
                    if [[ $REPLY =~ ^[Yy]$ ]]; then
                        sudo apt-get update
                        sudo apt-get install -y nodejs npm
                    fi
                    ;;
                redhat)
                    echo "For RedHat/CentOS/Fedora:"
                    echo "  sudo yum install -y nodejs npm"
                    
                    read -p "Do you want to run this command now? (y/n) " -n 1 -r
                    echo
                    if [[ $REPLY =~ ^[Yy]$ ]]; then
                        sudo yum install -y nodejs npm
                    fi
                    ;;
                arch)
                    echo "For Arch Linux:"
                    echo "  sudo pacman -S nodejs npm"
                    
                    read -p "Do you want to run this command now? (y/n) " -n 1 -r
                    echo
                    if [[ $REPLY =~ ^[Yy]$ ]]; then
                        sudo pacman -S nodejs npm
                    fi
                    ;;
                *)
                    echo "Please install Node.js and npm for your distribution"
                    echo "Visit: https://nodejs.org/"
                    exit 1
                    ;;
            esac
            ;;
        macos)
            if command -v brew &> /dev/null; then
                echo "Installing with Homebrew..."
                read -p "Run 'brew install node'? (y/n) " -n 1 -r
                echo
                if [[ $REPLY =~ ^[Yy]$ ]]; then
                    brew install node
                fi
            else
                echo "Please install Node.js from https://nodejs.org/"
                echo "Or install Homebrew first: https://brew.sh/"
                exit 1
            fi
            ;;
        windows)
            echo "Please install Node.js from https://nodejs.org/"
            exit 1
            ;;
        *)
            echo "Please install Node.js and npm for your system"
            exit 1
            ;;
    esac
fi

# Check again if npm is now available
if ! command -v npm &> /dev/null; then
    echo -e "${RED}❌ npm installation failed or was skipped${NC}"
    echo "Please install npm manually and run this script again"
    exit 1
fi

echo ""
echo "Installing tree-sitter CLI..."
echo ""

# Install tree-sitter globally
if npm install -g tree-sitter-cli; then
    echo ""
    echo -e "${GREEN}✅ tree-sitter CLI installed successfully!${NC}"
    tree-sitter --version
    echo ""
    echo "MinZ is now ready to use!"
    echo ""
    echo "You can compile MinZ programs with:"
    echo "  mz program.minz -o program.a80"
else
    echo ""
    echo -e "${RED}❌ Failed to install tree-sitter CLI${NC}"
    echo ""
    echo "Try installing with sudo:"
    echo "  sudo npm install -g tree-sitter-cli"
    echo ""
    echo "Or install locally:"
    echo "  npm install tree-sitter-cli"
    echo "  export PATH=\"$PWD/node_modules/.bin:\$PATH\""
    exit 1
fi