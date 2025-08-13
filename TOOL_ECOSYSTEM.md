# MinZ Tool Ecosystem - Standardized Naming Convention

*Last Updated: August 13, 2025*

## 🛠️ Official Tool Names

### Core Tools (Implemented)
- **`mz`** - MinZ Compiler (formerly minzc)
  - Compiles `.minz` → `.a80/.c/.wasm/.mir`
  - Multi-backend support
  - Main development tool

- **`mza`** - MinZ Assembler  
  - Assembles `.a80` → `.bin`
  - Z80 assembly to binary
  - Supports multiple output formats

- **`mze`** - MinZ Emulator
  - Z80 emulation environment
  - Runs `.bin` files
  - Built-in debugger

- **`mzr`** - MinZ REPL (formerly minzr)
  - Interactive MinZ shell
  - Immediate code execution
  - Learning and experimentation

### Proposed Tools (To Implement)
- **`mzv`** - MinZ VM (MIR Virtual Machine) 🆕
  - Executes `.mir` bytecode directly
  - Platform-independent execution
  - Debugging and profiling support
  
- **`mzd`** - MinZ Debugger 🆕
  - Source-level debugging
  - Breakpoints and stepping
  - Memory inspection

- **`mzl`** - MinZ Linker 🆕
  - Links multiple `.o` files
  - Library management
  - Symbol resolution

- **`mzp`** - MinZ Package Manager 🆕
  - Dependency management
  - Package distribution
  - Version control

## 📋 Tool Pipeline

```
.minz source
    ↓
[mz] compiler → .mir (intermediate)
    ↓           ↓
[mz -b z80]  [mzv] VM execution
    ↓
.a80 assembly
    ↓
[mza] assembler
    ↓
.bin binary
    ↓
[mze] emulator
```

## 🔄 Migration Plan

### Phase 1: Immediate Renames (Today)
1. Update all binary names:
   - `minzc` → `mz`
   - `minzr` → `mzr` (if exists)
   
2. Update all documentation:
   - README.md
   - CLAUDE.md
   - All docs/*.md files
   - All examples comments

### Phase 2: Create mzv (This Week)
```go
// cmd/mzv/main.go - MIR VM Implementation
package main

import (
    "github.com/minz/minzc/pkg/mir"
    "github.com/minz/minzc/pkg/vm"
)

func main() {
    // Load MIR bytecode
    // Execute in VM
    // Return result
}
```

### Phase 3: Tool Integration (Next Month)
- Unified CLI interface
- Shared configuration
- Cross-tool debugging

## 🎯 Usage Examples

### Current (Old Names)
```bash
# Compile
minzc program.minz -o program.a80

# Assemble  
mza program.a80 -o program.bin

# Run REPL
minzr
```

### New Standard
```bash
# Compile to assembly
mz program.minz -o program.a80

# Compile to MIR and run in VM
mz program.minz -o program.mir
mzv program.mir

# Direct compilation and execution
mz program.minz | mzv -

# Assemble to binary
mza program.a80 -o program.bin

# Emulate
mze program.bin

# Interactive REPL
mzr
```

## 📦 Installation

### Quick Install (Future)
```bash
curl -sSL https://minz-lang.org/install.sh | sh
# Installs: mz, mza, mze, mzr, mzv
```

### Manual Build
```bash
cd minzc
make all-tools
# Builds all tools with correct names
```

## 🔧 Tool Capabilities

### mz (Compiler)
- **Input**: `.minz` source files
- **Output**: `.a80`, `.c`, `.wasm`, `.mir`, `.ll`, `.s`
- **Backends**: z80, 6502, 68000, c, wasm, llvm, mir
- **Features**: Optimization, SMC, CTIE

### mza (Assembler)
- **Input**: `.a80` assembly files
- **Output**: `.bin`, `.hex`, `.tap`, `.sna`
- **Targets**: ZX Spectrum, MSX, CP/M, bare metal
- **Features**: Macros, includes, symbol tables

### mze (Emulator)
- **Input**: `.bin`, `.tap`, `.sna` files
- **Emulation**: Z80, Z180, eZ80
- **Features**: Debugger, profiler, memory viewer
- **Speed**: ~3.5MHz (authentic) to max

### mzr (REPL)
- **Input**: Interactive MinZ code
- **Output**: Immediate results
- **Features**: History, completion, help
- **Backend**: MIR VM or native

### mzv (MIR VM)
- **Input**: `.mir` bytecode files
- **Output**: Program execution results
- **Features**: JIT compilation, profiling
- **Platforms**: All (pure Go implementation)

## 🎉 Benefits of Standardization

1. **Consistency** - All tools start with `mz`
2. **Discoverability** - Type `mz` + TAB for all tools
3. **Professionalism** - Clean, memorable names
4. **Ecosystem** - Clear tool relationships
5. **Documentation** - Easier to explain and teach

## 📝 Documentation Updates Required

### Files to Update
- [x] Create TOOL_ECOSYSTEM.md (this file)
- [ ] Update README.md
- [ ] Update CLAUDE.md
- [ ] Update all docs/*.md
- [ ] Update examples/*.minz comments
- [ ] Update Makefile
- [ ] Update install scripts
- [ ] Update CI/CD pipelines

### Command Replacements
```bash
# Global find/replace needed:
s/minzc /mz /g
s/\.\/minzc/\.\/mz/g
s/minzr/mzr/g
s/"minzc"/"mz"/g
```

## 🚀 Future Vision

```
MinZ Tool Suite v1.0
├── mz    - Compiler (all backends)
├── mza   - Assembler (Z80/eZ80)
├── mze   - Emulator (authentic)
├── mzr   - REPL (interactive)
├── mzv   - VM (portable execution)
├── mzd   - Debugger (source-level)
├── mzl   - Linker (modular builds)
└── mzp   - Package Manager (ecosystem)
```

---

*"From mz to mzp, every tool has its place in the MinZ ecosystem"*