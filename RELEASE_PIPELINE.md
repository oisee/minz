# MinZ Release Pipeline Documentation

## Overview

The MinZ release pipeline automates the build and distribution process for all supported platforms. It includes:

- Cross-platform binary compilation
- Standard library packaging
- Documentation bundling
- Docker image creation
- GitHub release automation

## Components

### 1. Local Release Build (`build/release.sh`)

Comprehensive script that creates a complete release package:

```bash
# Build release for current version
./build/release.sh

# Build with specific version
VERSION=v0.9.2 ./build/release.sh
```

**Features:**
- Cross-compiles for Darwin (Intel/ARM), Linux (x64/ARM), Windows
- Packages tree-sitter grammar and bindings
- Bundles standard library and examples
- Creates platform-specific archives
- Generates checksums and release notes

**Output Structure:**
```
release/
├── minz-v0.9.1/                    # Universal package
│   ├── bin/                        # All platform binaries
│   ├── lib/                        # Standard library & grammar
│   ├── docs/                       # Complete documentation
│   ├── examples/                   # Example programs
│   └── tools/                      # Helper scripts
├── minz-v0.9.1.tar.gz             # Universal tarball
├── minz-v0.9.1-darwin-amd64.tar.gz
├── minz-v0.9.1-linux-amd64.tar.gz
├── minz-v0.9.1-windows-amd64.zip
└── minz-v0.9.1-checksums.txt
```

### 2. GitHub Actions Workflow (`.github/workflows/release.yml`)

Automated CI/CD pipeline triggered by:
- Git tags (`v*` pattern)
- Manual workflow dispatch

**Jobs:**
1. **build-release**: Main release build on Ubuntu
2. **build-platform**: Platform-specific builds
3. **docker-image**: Container image creation

### 3. Docker Support (`build/Dockerfile`)

Multi-stage Dockerfile for containerized MinZ:

```bash
# Build image
docker build -t minz:latest -f build/Dockerfile .

# Run compiler
docker run -v $(pwd):/workspace minz:latest minzc program.minz -O --enable-smc
```

### 4. Development Package (`build/package.sh`)

Creates source packages for contributors:

```bash
./build/package.sh
```

Includes:
- Full source code with Git history
- Development setup scripts
- VS Code workspace configuration
- Contributor documentation

## Release Process

### 1. Prepare Release

```bash
# Update version in code
# Update CHANGELOG.md
# Commit changes
git add -A
git commit -m "chore: Prepare v0.9.2 release"
```

### 2. Tag Release

```bash
# Create annotated tag
git tag -a v0.9.2 -m "Release v0.9.2: Zero-cost abstractions"

# Push tag
git push origin v0.9.2
```

### 3. GitHub Actions

The workflow automatically:
- Builds all platform binaries
- Creates release archives
- Generates release notes
- Creates draft GitHub release

### 4. Finalize Release

1. Review draft release on GitHub
2. Update release notes if needed
3. Attach any additional files
4. Publish release

## Platform Support

### Binary Targets

| Platform | Architecture | Binary Name |
|----------|-------------|-------------|
| macOS | Intel | `minzc-darwin-amd64` |
| macOS | Apple Silicon | `minzc-darwin-arm64` |
| Linux | x64 | `minzc-linux-amd64` |
| Linux | ARM64 | `minzc-linux-arm64` |
| Windows | x64 | `minzc-windows-amd64.exe` |

### Installation Methods

**Unix/Linux/macOS:**
```bash
cd tools && ./install.sh
```

**Windows:**
Add `bin\` directory to system PATH

**Docker:**
```bash
docker pull ghcr.io/minz-lang/minz:latest
```

## Version Scheme

MinZ follows semantic versioning:

- **Major (X.0.0)**: Breaking language changes
- **Minor (0.X.0)**: New features, backwards compatible
- **Patch (0.0.X)**: Bug fixes, optimizations

Special suffixes:
- `-dev`: Development builds
- `-rc.N`: Release candidates
- `-beta.N`: Beta releases

## Testing Releases

Before publishing:

1. **Test binaries on each platform**
   ```bash
   ./minzc examples/working_demo.minz -O --enable-smc
   ```

2. **Verify standard library**
   ```bash
   ./minzc examples/stdlib_test.minz
   ```

3. **Check Docker image**
   ```bash
   docker run minz:v0.9.2 minzc --version
   ```

## Troubleshooting

### Common Issues

**Cross-compilation fails:**
- Ensure Go 1.21+ is installed
- Check CGO_ENABLED=0 for static builds

**Release script permissions:**
```bash
chmod +x build/*.sh
```

**Missing dependencies:**
- tree-sitter CLI: `npm install -g tree-sitter-cli`
- Go modules: `cd minzc && go mod download`

## Future Enhancements

1. **Package Managers**
   - Homebrew formula for macOS
   - APT/YUM repositories for Linux
   - Chocolatey package for Windows

2. **Binary Signing**
   - Code signing for macOS/Windows
   - GPG signatures for Linux

3. **Automated Testing**
   - Platform-specific test suites
   - Integration tests in CI

4. **CDN Distribution**
   - Mirror releases to CDN
   - Installer scripts with checksums

---

The release pipeline ensures every MinZ release is thoroughly tested, properly packaged, and easily installable across all supported platforms. This infrastructure supports our mission to bring modern programming to vintage hardware without compromise!