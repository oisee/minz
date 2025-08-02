#!/bin/bash
# MinZ Development Package Builder
# Creates development packages with source code for contributors

set -e

VERSION=${VERSION:-$(git describe --tags --always --dirty 2>/dev/null || echo "0.9.1-dev")}
DEV_DIR="dev-package/minz-dev-$VERSION"

echo "Building MinZ Development Package $VERSION"
echo "========================================"

# Clean and create directory
rm -rf dev-package/
mkdir -p "$DEV_DIR"/{src,tools,docs,tests}

# Copy full source tree
echo "Copying source code..."
cp -r .git "$DEV_DIR/" 2>/dev/null || true  # Include git history if available
cp -r minzc "$DEV_DIR/src/"
cp -r stdlib "$DEV_DIR/src/"
cp -r examples "$DEV_DIR/src/"
cp -r test "$DEV_DIR/tests/"
cp grammar.js "$DEV_DIR/src/"
cp package.json "$DEV_DIR/src/"
cp -r src "$DEV_DIR/src/tree-sitter-src"
cp -r bindings "$DEV_DIR/src/"
cp -r queries "$DEV_DIR/src/"

# Copy all documentation
echo "Copying documentation..."
cp *.md "$DEV_DIR/docs/"
cp -r docs/* "$DEV_DIR/docs/"

# Copy build tools
echo "Copying build tools..."
cp -r build/* "$DEV_DIR/tools/"
cp .gitignore "$DEV_DIR/"
cp .github/workflows/* "$DEV_DIR/tools/workflows/" 2>/dev/null || true

# Create development guide
cat > "$DEV_DIR/DEVELOPMENT.md" << 'EOF'
# MinZ Development Guide

## Setting Up Development Environment

### Prerequisites
- Go 1.21+
- Node.js 20+
- tree-sitter CLI
- Git

### Quick Start
```bash
# Install dependencies
cd src/minzc && go mod download
cd ../.. && npm install
npm install -g tree-sitter-cli

# Generate parser
tree-sitter generate

# Build compiler
cd src/minzc && make build

# Run tests
make test
```

## Project Structure

```
minz-dev/
├── src/
│   ├── minzc/           # Go compiler source
│   ├── stdlib/          # Standard library
│   ├── examples/        # Example programs
│   ├── grammar.js       # Tree-sitter grammar
│   └── tree-sitter-src/ # Parser C source
├── tests/               # Test corpus
├── docs/                # Documentation
└── tools/               # Build scripts
```

## Key Development Areas

### 1. Compiler Pipeline
- `src/minzc/pkg/parser/` - Tree-sitter integration
- `src/minzc/pkg/semantic/` - Type checking, lambda transformation
- `src/minzc/pkg/ir/` - Intermediate representation
- `src/minzc/pkg/optimizer/` - Optimization passes
- `src/minzc/pkg/codegen/` - Z80 code generation

### 2. Language Features
- **Lambda Transformation**: `semantic/analyzer.go`
- **TRUE SMC**: `optimizer/smc_optimization.go`
- **Error Handling**: Design in `docs/104_Z80_Native_Error_Handling.md`

### 3. Standard Library
- Platform-specific implementations in `stdlib/`
- Zero-cost abstractions using metaprogramming

## Contributing

### Adding a New Feature
1. Update grammar in `grammar.js`
2. Run `tree-sitter generate`
3. Update AST types in `pkg/ast/`
4. Implement semantic analysis
5. Add IR generation
6. Write tests

### Testing
```bash
# Unit tests
cd src/minzc && go test ./...

# Grammar tests
tree-sitter test

# E2E tests
./tools/run_e2e_tests.sh
```

### Performance
- Always benchmark optimizations
- Use `pkg/z80testing/` for cycle-accurate testing
- Document performance claims with data

## Current Priorities

1. **Interface Monomorphization** - Zero-cost polymorphism
2. **Standard Library** - Metafunction-based I/O
3. **Pattern Matching** - Complete implementation
4. **Tail Recursion** - Loop transformation

## Resources

- [Compiler Architecture](docs/COMPILER_ARCHITECTURE.md)
- [Language Design](docs/DESIGN.md)
- [Z80 Optimization Guide](docs/018_TRUE_SMC_Design_v2.md)

Happy hacking! 🚀
EOF

# Create contributor setup script
cat > "$DEV_DIR/tools/setup-dev.sh" << 'EOF'
#!/bin/bash
# MinZ Development Environment Setup

echo "Setting up MinZ development environment..."

# Check prerequisites
check_command() {
    if ! command -v $1 &> /dev/null; then
        echo "Error: $1 is required but not installed"
        exit 1
    fi
}

check_command go
check_command node
check_command npm
check_command git

# Install Go dependencies
echo "Installing Go dependencies..."
cd src/minzc && go mod download
cd ../..

# Install Node dependencies
echo "Installing Node dependencies..."
npm install

# Install tree-sitter CLI
echo "Installing tree-sitter CLI..."
npm install -g tree-sitter-cli

# Generate parser
echo "Generating parser..."
cd src && tree-sitter generate
cd ..

# Build compiler
echo "Building compiler..."
cd src/minzc && make build
cd ../..

echo "Setup complete! Run 'src/minzc/minzc --help' to verify"
EOF

chmod +x "$DEV_DIR/tools/setup-dev.sh"

# Create VS Code workspace
cat > "$DEV_DIR/minz.code-workspace" << 'EOF'
{
    "folders": [
        {
            "path": "."
        }
    ],
    "settings": {
        "files.associations": {
            "*.minz": "minz",
            "*.a80": "asm-collection"
        },
        "editor.formatOnSave": true,
        "[go]": {
            "editor.defaultFormatter": "golang.go"
        }
    },
    "extensions": {
        "recommendations": [
            "golang.go",
            "georgewfraser.tree-sitter"
        ]
    },
    "launch": {
        "version": "0.2.0",
        "configurations": [
            {
                "name": "Debug MinZ Compiler",
                "type": "go",
                "request": "launch",
                "mode": "debug",
                "program": "${workspaceFolder}/src/minzc/cmd/minzc",
                "args": ["${file}"],
                "cwd": "${fileDirname}"
            }
        ]
    }
}
EOF

# Create test helper
cat > "$DEV_DIR/tools/test-feature.sh" << 'EOF'
#!/bin/bash
# Test a specific MinZ feature

if [ -z "$1" ]; then
    echo "Usage: test-feature.sh <feature.minz>"
    exit 1
fi

MINZC="src/minzc/minzc"
FILE="$1"
ASM="${FILE%.minz}.a80"

echo "Testing $FILE..."

# Compile
if ! $MINZC "$FILE" -O --enable-smc; then
    echo "Compilation failed"
    exit 1
fi

echo "Generated assembly:"
echo "=================="
cat "$ASM" | head -50
echo "=================="

# TODO: Add Z80 emulation/verification here

echo "Test complete"
EOF

chmod +x "$DEV_DIR/tools/test-feature.sh"

# Create archives
echo "Creating archives..."
cd dev-package

# Source tarball
tar -czf "minz-dev-$VERSION.tar.gz" "minz-dev-$VERSION"

# Source zip
zip -r "minz-dev-$VERSION.zip" "minz-dev-$VERSION"

cd ..

echo "========================================"
echo "Development package complete!"
echo "Version: $VERSION"
echo "Location: dev-package/"
echo ""
echo "Archives created:"
ls -la dev-package/*.tar.gz dev-package/*.zip
echo ""
echo "For contributors:"
echo "1. Extract the archive"
echo "2. Run tools/setup-dev.sh"
echo "3. Open in VS Code with minz.code-workspace"