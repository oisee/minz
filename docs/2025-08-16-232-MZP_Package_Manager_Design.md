# MZP - MinZ Package Manager Design

## Overview
A simple, efficient package manager for MinZ focused on retro computing constraints.

## Core Philosophy
- **Tiny footprint** - Packages should be KB, not MB
- **No bloat** - Only include what you use (tree-shaking)
- **Platform-aware** - Different code for different targets
- **Offline-first** - Works without internet (important for retro systems)
- **Source-based** - Distribute MinZ source, not binaries

## Command Line Interface

### Basic Commands
```bash
# Initialize new project
mzp init

# Install packages
mzp install zx-graphics         # From registry
mzp install ./local-package     # Local package
mzp install user/repo            # From GitHub

# Create and publish
mzp create my-lib
mzp publish

# Search and info
mzp search graphics
mzp info zx-graphics

# Update
mzp update                       # All packages
mzp update zx-graphics          # Specific package

# Remove
mzp remove zx-graphics
```

## Package Structure

### Package Layout
```
my-package/
├── mzp.toml                    # Package manifest
├── src/
│   ├── lib.minz               # Main library file
│   └── extras.minz            # Additional modules
├── examples/
│   └── demo.minz              # Usage examples
├── tests/
│   └── test.minz              # Test files
└── README.md
```

### Manifest Format (mzp.toml)
```toml
[package]
name = "zx-graphics"
version = "1.0.0"
authors = ["Dev Name <email>"]
description = "Graphics library for ZX Spectrum"
license = "MIT"
repository = "https://github.com/user/zx-graphics"

[dependencies]
# Version specifications
zx-core = "^1.0.0"              # Compatible with 1.x.x
math-fixed = "=0.5.0"           # Exact version
utils = ">=0.3.0"               # Minimum version

[platform-dependencies.spectrum]
zx-hardware = "^2.0.0"

[platform-dependencies.cpm]
cpm-io = "^1.0.0"

[targets]
# Platform-specific settings
default = "z80"
supported = ["z80", "6502"]

[features]
# Optional features
sprites = ["src/sprites.minz"]
tiles = ["src/tiles.minz"]
default = ["sprites"]

[metadata]
keywords = ["graphics", "spectrum", "sprites"]
categories = ["graphics", "game-dev"]
```

## Package Resolution

### Directory Structure
```
~/.mzp/
├── registry/                   # Package registry cache
│   ├── index.json             # Package index
│   └── packages/              # Downloaded packages
│       └── zx-graphics-1.0.0/
├── cache/                      # Compiled module cache
└── config.toml                 # Global configuration
```

### Project Structure
```
my-game/
├── mzp.toml                    # Project manifest
├── mzp.lock                    # Lock file (exact versions)
├── src/
│   └── main.minz
└── packages/                   # Local packages directory
    ├── zx-graphics/
    └── game-engine/
```

## Import Resolution

### How Imports Work
```minz
// Import from package
import zx_graphics;              // Looks in packages/zx_graphics/src/lib.minz
import zx_graphics.sprites;      // Looks in packages/zx_graphics/src/sprites.minz

// Import with alias
import zx_graphics as gfx;
import math.fixed as fx;

// Platform-specific imports
@if(TARGET_SPECTRUM) {
    import zx_hardware;
}
```

### Resolution Order
1. Local project files
2. `packages/` directory
3. Standard library
4. System packages (~/.mzp/packages)

## Registry Design

### Central Registry
Simple HTTP API serving static JSON files:

```json
// GET https://registry.minz-lang.org/packages/zx-graphics
{
    "name": "zx-graphics",
    "versions": {
        "1.0.0": {
            "url": "https://github.com/user/zx-graphics/archive/v1.0.0.tar.gz",
            "checksum": "sha256:abc123...",
            "dependencies": {
                "zx-core": "^1.0.0"
            }
        }
    },
    "latest": "1.0.0",
    "description": "Graphics library for ZX Spectrum",
    "keywords": ["graphics", "spectrum"],
    "author": "Dev Name",
    "license": "MIT"
}
```

### Decentralized Options
- Direct GitHub imports: `mzp install github:user/repo`
- Local file imports: `mzp install file:../mylib`
- Private registries: `mzp --registry=http://mycompany.com install internal-lib`

## Version Resolution Algorithm

### Semantic Versioning
```
MAJOR.MINOR.PATCH

^1.2.3  := >=1.2.3 <2.0.0  (compatible)
~1.2.3  := >=1.2.3 <1.3.0  (close)
1.2.x   := >=1.2.0 <1.3.0  (wildcard)
>=1.2.3 := >=1.2.3         (minimum)
=1.2.3  := exactly 1.2.3   (exact)
```

### Conflict Resolution
1. Build dependency graph
2. Find highest version satisfying all constraints
3. Fail if conflicts exist
4. Write to lock file

## Build Integration

### Compiler Integration
```bash
# mzp automatically manages dependencies
mzp build                       # Builds project with deps
mzp run                        # Build and run
mzp test                       # Run tests

# Direct compiler integration
mz --packages=./packages main.minz -o game.a80
```

### Build Script (build.minz)
```minz
@build {
    // Custom build logic
    @if(DEBUG) {
        @define("DEBUG_MODE", "1")
    }
    
    // Platform-specific builds
    @if(TARGET == "spectrum") {
        @include("packages/zx-graphics/spectrum.minz")
    }
}
```

## Implementation Plan

### Phase 1: Core (MVP)
1. **Package manifest parsing** (mzp.toml)
2. **Local package installation** (file system)
3. **Basic dependency resolution**
4. **Import path resolution**
5. **Integration with mz compiler**

### Phase 2: Registry
1. **Central registry API**
2. **Package publishing**
3. **Version resolution**
4. **Checksums and security**

### Phase 3: Advanced
1. **Platform-specific dependencies**
2. **Feature flags**
3. **Build scripts**
4. **Package caching**
5. **Parallel downloads**

## Technical Implementation

### Language Choice
**Go** - Same as compiler, single binary distribution

### Core Components
```go
// Package representation
type Package struct {
    Name         string
    Version      semver.Version
    Dependencies map[string]VersionReq
    Files        map[string][]byte
}

// Dependency resolver
type Resolver struct {
    registry Registry
    cache    Cache
}

func (r *Resolver) Resolve(manifest Manifest) (*LockFile, error) {
    // Build dependency graph
    // Resolve versions
    // Return lock file
}

// Registry interface
type Registry interface {
    Search(query string) []Package
    Fetch(name string, version VersionReq) (*Package, error)
    Publish(pkg *Package) error
}
```

### CLI Structure
```go
// cmd/mzp/main.go
func main() {
    commands := map[string]Command{
        "init":    &InitCommand{},
        "install": &InstallCommand{},
        "build":   &BuildCommand{},
        "publish": &PublishCommand{},
    }
    // ... execute
}
```

## Example Packages

### zx-graphics
```toml
[package]
name = "zx-graphics"
version = "1.0.0"
description = "ZX Spectrum graphics primitives"

[dependencies]
# None - base package

[exports]
main = "src/lib.minz"
```

```minz
// src/lib.minz
pub fun set_pixel(x: u8, y: u8) -> void {
    let addr = calc_screen_addr(x, y);
    let bit = 1 << (7 - (x & 7));
    @poke(addr, @peek(addr) | bit);
}

pub fun draw_line(x0: u8, y0: u8, x1: u8, y1: u8) -> void {
    // Bresenham's algorithm
}
```

### game-engine
```toml
[package]
name = "game-engine"
version = "0.1.0"

[dependencies]
zx-graphics = "^1.0.0"
zx-sound = "^0.5.0"

[features]
sprites = ["src/sprites.minz"]
physics = ["src/physics.minz"]
```

## Benefits for MinZ

### For Users
- **Easy dependency management**
- **Code reuse**
- **Platform-specific packages**
- **Version compatibility**

### For Ecosystem
- **Share libraries**
- **Grow community**
- **Standard practices**
- **Quality packages**

### For Language
- **Real-world testing**
- **Find missing features**
- **Drive adoption**
- **Professional feel**

## Success Metrics

### MVP Success
- Install and use 10 packages
- Resolve complex dependency trees
- Work offline with cached packages
- Build Snake/Tetris with packages

### Long-term Success
- 100+ packages in registry
- Active community contributions
- Platform-specific ecosystems
- Professional game/app development

## Open Questions

1. **Binary packages?** - Pre-compiled MIR or assembly?
2. **Security?** - How to verify package integrity?
3. **Namespacing?** - Prevent name conflicts?
4. **Private packages?** - Corporate use cases?
5. **Monorepo support?** - Multiple packages in one repo?

## Next Steps

1. **Create prototype** - Basic manifest parsing and resolution
2. **Test with games** - Use for Snake/Tetris libraries
3. **Set up registry** - Simple GitHub Pages hosting
4. **Document standards** - Package guidelines
5. **Build community** - Encourage contributions

This package manager will make MinZ feel like a real, professional language while staying true to its retro roots!