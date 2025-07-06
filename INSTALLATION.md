# MinZ Installation Guide

This guide covers the complete installation process for MinZ, including the compiler and VS Code extension.

## Prerequisites

- **Go 1.21+** (for building the compiler)
- **Node.js 16+** and npm (for tree-sitter and VS Code extension)
- **Git** (for cloning the repository)
- **VS Code** (for the extension)

## Quick Installation

### 1. Clone the Repository
```bash
git clone https://github.com/minz-lang/minz.git
cd minz
```

### 2. Install Tree-sitter CLI
```bash
npm install -g tree-sitter-cli
```

### 3. Build the Parser
```bash
# In the root directory
npm install
tree-sitter generate
```

### 4. Build the MinZ Compiler
```bash
cd minzc
make build

# Test the compiler
./minzc --version

# Add to PATH (optional)
echo 'export PATH="$PATH:'$(pwd)'"' >> ~/.bashrc
source ~/.bashrc
```

### 5. Install VS Code Extension

#### Option A: From Source (Recommended)
```bash
# From the repository root
cd vscode-minz
npm install
npm run compile

# Package the extension
npm install -g vsce
vsce package

# Install the extension
code --install-extension minz-language-support-0.1.0.vsix
```

#### Option B: Manual Installation
1. Copy the `vscode-minz` folder to VS Code extensions directory:
   - **macOS/Linux**: `~/.vscode/extensions/minz-language-support`
   - **Windows**: `%USERPROFILE%\.vscode\extensions\minz-language-support`
2. Restart VS Code

## Detailed Build Instructions

### Building the MinZ Compiler

The MinZ compiler (`minzc`) is written in Go and uses tree-sitter for parsing.

1. **Install Go** (if not already installed):
   ```bash
   # macOS
   brew install go

   # Linux
   sudo apt install golang-go  # Debian/Ubuntu
   sudo dnf install golang      # Fedora

   # Windows
   # Download from https://golang.org/dl/
   ```

2. **Build the compiler**:
   ```bash
   cd minzc
   
   # Install dependencies
   make deps
   
   # Build the compiler
   make build
   
   # Run tests
   make test
   ```

3. **Verify installation**:
   ```bash
   ./minzc ../examples/fibonacci.minz
   # Should generate fibonacci.a80
   ```

### Building the VS Code Extension

1. **Install dependencies**:
   ```bash
   cd vscode-minz
   npm install
   ```

2. **Compile TypeScript**:
   ```bash
   npm run compile
   ```

3. **Package the extension**:
   ```bash
   # Install VS Code Extension Manager
   npm install -g vsce

   # Package the extension
   vsce package
   ```

4. **Install the extension**:
   ```bash
   code --install-extension minz-language-support-0.1.0.vsix
   ```

## Platform-Specific Instructions

### macOS

```bash
# Install Homebrew if needed
/bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"

# Install dependencies
brew install go node

# Clone and build
git clone https://github.com/minz-lang/minz.git
cd minz
npm install -g tree-sitter-cli
npm install
tree-sitter generate
cd minzc && make build
```

### Linux (Ubuntu/Debian)

```bash
# Install dependencies
sudo apt update
sudo apt install golang-go nodejs npm build-essential

# Clone and build
git clone https://github.com/minz-lang/minz.git
cd minz
sudo npm install -g tree-sitter-cli
npm install
tree-sitter generate
cd minzc && make build
```

### Windows

1. Install [Go](https://golang.org/dl/) and [Node.js](https://nodejs.org/)
2. Install Git for Windows
3. Open Git Bash or PowerShell:
```powershell
# Clone repository
git clone https://github.com/minz-lang/minz.git
cd minz

# Install tree-sitter
npm install -g tree-sitter-cli

# Build parser
npm install
tree-sitter generate

# Build compiler
cd minzc
go build -o minzc.exe cmd/minzc/main.go
```

## Verifying Your Installation

1. **Test the compiler**:
   ```bash
   minzc --help
   # Should show usage information
   ```

2. **Compile a test program**:
   ```bash
   cd examples
   minzc fibonacci.minz
   # Should create fibonacci.a80
   ```

3. **Test VS Code extension**:
   - Open VS Code
   - Create a new file with `.minz` extension
   - You should see syntax highlighting
   - Right-click to see MinZ compilation commands

## Configuration

### Setting up VS Code

1. Open VS Code settings (`Cmd+,` or `Ctrl+,`)
2. Search for "MinZ"
3. Configure:
   - `minz.compilerPath`: Path to minzc (if not in PATH)
   - `minz.outputDirectory`: Where to place compiled files
   - `minz.enableOptimizations`: Enable optimizations by default

### Environment Variables

Add to your shell profile (`.bashrc`, `.zshrc`, etc.):
```bash
# MinZ compiler path
export PATH="$PATH:/path/to/minz/minzc"

# Optional: Set default output directory
export MINZ_OUTPUT_DIR="./build"
```

## Troubleshooting

### Common Issues

1. **"minzc: command not found"**
   - Ensure minzc is in your PATH
   - Or use the full path: `/path/to/minz/minzc/minzc`

2. **"tree-sitter: command not found"**
   - Install globally: `npm install -g tree-sitter-cli`

3. **VS Code extension not working**
   - Reload VS Code window: `Cmd+Shift+P` → "Developer: Reload Window"
   - Check the MinZ output channel for errors

4. **Compilation errors**
   - Ensure Go version is 1.21 or higher: `go version`
   - Update dependencies: `cd minzc && make deps`

### Getting Help

- Check the [documentation](https://github.com/minz-lang/minz/wiki)
- Report issues: https://github.com/minz-lang/minz/issues
- Join the community: [Discord/Forum link]

## Next Steps

1. Read the [MinZ Language Guide](README.md)
2. Explore the [examples](examples/) directory
3. Try the [tutorials](docs/tutorials/)
4. Start building your Z80 programs!

## Updating MinZ

To update to the latest version:
```bash
cd minz
git pull origin master
npm install
tree-sitter generate
cd minzc && make clean && make build
cd ../vscode-minz && npm install && npm run compile
```

Happy coding with MinZ! 🚀