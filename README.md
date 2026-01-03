# MinZ Programming Language

<div align="center">

![MinZ Logo](/media/minz-logo-shamrock-mint.png)

### **Modern Programming Language for Vintage & Modern Platforms**

[![Version](https://img.shields.io/badge/version-0.16.1-brightgreen)](https://github.com/oisee/minz/releases)
[![Platforms](https://img.shields.io/badge/platforms-Z80%20%7C%20Z80N%20%7C%206502%20%7C%20Crystal%20%7C%20WASM-blue)]()
[![Success Rate](https://img.shields.io/badge/compilation-92%25-brightgreen)]()
[![License](https://img.shields.io/badge/license-MIT-purple)]()

**Write modern code. Deploy everywhere. From 1978 Z80 to 2026 Crystal.**

[Quick Start](#-quick-start) • [Features](#-revolutionary-features) • [Install](#-installation) • [Examples](#-code-examples) • [Documentation](#-documentation)

</div>

---

## 🎯 **What is MinZ?**

MinZ is a **revolutionary programming language** that brings modern abstractions to vintage hardware while enabling cutting-edge development workflows. Write Ruby-style code that compiles to blazing-fast Z80 assembly or modern Crystal.

### **Why MinZ?**

```minz
// Modern Ruby-style syntax
const NAME = "MinZ";
let greeting = "Hello from #{NAME}!";  // Ruby interpolation!

// Structs and field access
struct Point { x: u8, y: u8 }
let p = Point { x: 10, y: 20 };
let sum = p.x + p.y;

// Compile-time execution (faster than zero-cost!)
@ctie
fun distance(x: u8, y: u8) -> u8 {
    return abs(x - y);
}
let d = distance(10, 3);  // Becomes: LD A, 7 (computed at compile-time!)
```

**Then deploy to:**
- 🎮 **ZX Spectrum** (1982)
- 💾 **Commodore 64** (1982)  
- 💎 **Crystal/Ruby** (2025)
- 🌐 **WebAssembly** (Modern browsers)
- 🖥️ **Native C** (Any platform)

---

## 🚀 **Quick Start**

### **1. Install Dependencies**

MinZ requires tree-sitter for parsing. Install it first:

```bash
# Install tree-sitter CLI
npm install -g tree-sitter-cli

# Initialize tree-sitter config (REQUIRED!)
tree-sitter init-config
```

### **2. Install MinZ**

```bash
# macOS/Linux - from releases
curl -L https://github.com/oisee/minz/releases/latest/download/minz-$(uname -s)-$(uname -m).tar.gz | tar -xz
sudo mv mz /usr/local/bin/

# Or build from source (recommended)
git clone https://github.com/oisee/minz.git
cd minz/minzc
go build -o mz cmd/minzc/main.go
sudo mv mz /usr/local/bin/

# Verify
mz --version  # MinZ v0.16.0
```

> **Note:** Pre-built binaries may be for a different architecture. Building from source is recommended.

### **3. Write Your First Program**

```minz
// hello.minz
const VERSION = 15;

fun main() -> void {
    let message = "Hello from MinZ v0.#{VERSION}!";
    @print(message);
}
```

### **4. Compile & Run**

```bash
# For vintage hardware (Z80)
mz hello.minz -o hello.a80

# For modern testing (Crystal)
mz hello.minz -b crystal -o hello.cr
crystal run hello.cr  # Test instantly!
```

---

## ✨ **Revolutionary Features**

### **🎊 15 Major Revolutions Since v0.1.0**

<details>
<summary><b>Click to see the complete evolution journey</b></summary>

| Version | Revolution | Impact |
|---------|------------|--------|
| **v0.16.1** | DZRP Live Testing + `loop {}` | Remote emulator execution, proper halt pattern |
| **v0.16.0** | DAP Debugger + Z80N Vision | VS Code debugging, ZX Next roadmap |
| **v0.15.4** | Comprehensive Stdlib | 10 modules, 2400+ lines, game-ready |
| **v0.15.3** | Critical Fixes | ANSI filtering, cross-platform, clean array codegen |
| **v0.15.2** | Enum Error Values | `@error(EnumType.Variant)` compile-time constants |
| **v0.15.1** | Error Propagation | `@error(code)` sets CY flag + A register |
| **v0.15.0** | Ruby Interpolation + Crystal Backend | `"Hello #{name}!"` + Modern workflow |
| **v0.14.0** | Pattern Matching (partial) | Basic case syntax (in progress) |
| **v0.13.0** | Module System | `import` syntax (stdlib WIP) |
| **v0.12.0** | CTIE (Compile-Time Execution) | Functions run at compile-time! |
| **v0.11.0** | Complete Toolchain | Compiler + Assembler + Emulator + REPL |
| **v0.10.0** | Lambda Support | Lambda syntax and transformation |
| **v0.9.6** | Function Overloading | `print(anything)` |
| **v0.9.0** | Error Handling | CY flag + A register ABI (partial) |
| **v0.8.0** | True Self-Modifying Code | 10x performance through mutation |
| **v0.7.0** | LLVM Backend | Modern optimizations |
| **v0.6.0** | Module System | Professional organization |
| **v0.5.0** | Inline Assembly | Direct hardware control |
| **v0.4.0** | Multi-Platform | Z80, 6502, WASM, C |
| **v0.3.0** | Optimizer | 35+ peephole patterns |
| **v0.2.0** | Structs & Arrays | Real programs possible |
| **v0.1.0** | Genesis | Modern syntax for Z80 |

</details>

### **🏆 Working Features**

| Feature | Status | Example |
|---------|--------|---------|
| **CTIE** | ✅ Working | `@ctie fun add(a,b) -> ...` executes at compile-time |
| **Ruby Interpolation** | ✅ Working | `@print("Hello #{NAME}!")` |
| **True SMC** | ✅ Working | Self-modifying code optimizations |
| **Structs** | ✅ Working | `Point { x: 10, y: 20 }` with field access |
| **Crystal Backend** | ✅ Working | Test on modern, deploy to vintage |
| **Multi-Backend** | ✅ Working | Z80, 6502, C, WASM, Crystal, LLVM |
| **Pattern Matching** | ✅ Working | `case s { State.IDLE => State.RUNNING }` |
| **Enum Values** | ✅ Working | `State.IDLE`, `@error(MathError.DivByZero)` |
| **Error Propagation** | ✅ Working | `@error(code)` sets CY flag + A register |
| **Array Literals** | ✅ Working | `[10,20,30]` → clean `DB 10,20,30` |
| **Standard Library** | ✅ Working | 10 modules: math, graphics, input, text, sound, time, mem |
| **@extern FFI** | ✅ Working | `@extern(0x0010) fun rom_print()` with RST optimization |
| **DZRP Remote Run** | ✅ Working | `mzrun game.minz --reset` live emulator testing |
| **Infinite Loops** | ✅ Working | `loop { asm{EI;HALT} }` for explicit halt |
| **Iterator Chains** | 🚧 Partial | Compiles, DJNZ optimization in progress |

---

## 📚 **Code Examples**

### **Functions and Structs**
```minz
// Struct with field access
struct Point { x: u8, y: u8 }

fun distance(p1: Point, p2: Point) -> u8 {
    let dx = p2.x - p1.x;
    let dy = p2.y - p1.y;
    return dx + dy;  // Manhattan distance
}

fun main() -> void {
    let a = Point { x: 10, y: 20 };
    let b = Point { x: 30, y: 40 };
    let d = distance(a, b);
}
```

### **Arrays and Loops**
```minz
fun main() -> void {
    let data: [u8; 5] = [10, 20, 30, 40, 50];

    for i in 0..5 {
        let val = data[i];
        @print("Value: #{val}");
    }
}
```

### **Ruby-Style String Interpolation**
```minz
const USER = "Alice";
const SCORE = 9001;

fun show_status() -> void {
    @print("Player #{USER} scored #{SCORE} points!");
}
```

### **Compile-Time Execution (CTIE)**
```minz
@ctie
fun fibonacci(n: u8) -> u8 {
    if n <= 1 { return n; }
    return fibonacci(n-1) + fibonacci(n-2);
}

let fib10 = fibonacci(10);  // Computed at compile-time!
// Generates: LD A, 55  (no runtime calculation!)
```

### **Pattern Matching** (In Progress)
```minz
// Basic enum and case syntax - codegen in development
enum State { IDLE, RUNNING, STOPPED }

fun next_state(s: State) -> State {
    case s {
        State.IDLE => return State.RUNNING,
        State.RUNNING => return State.STOPPED,
        _ => return State.IDLE
    }
}
```
> **Note:** Pattern matching syntax parses but code generation is still in development.

### **Error Propagation**
```minz
enum FileError { None, NotFound, Permission }

// Function that can fail - uses @error with enum value
fun read_file?(path: u8) -> u8 ? FileError {
    if path == 0 {
        @error(FileError.NotFound);  // Proper enum syntax!
    }
    return path;
}

// Caller handles error via CY flag
fun main() -> void {
    let data: u8 = read_file?(5);
    // CY flag indicates success/failure
    // A register contains error code on failure
}
```
> **Note:** `@error(EnumType.Variant)` resolves to a compile-time constant for optimal code generation.

### **Infinite Loops**
```minz
fun main() -> void {
    @print("Hello World!");

    // Explicit halt intent - generates EI; HALT; JR loop
    loop {
        asm { EI; HALT }
    }
}
```
> **Note:** Empty `loop { }` is optimized away. Use explicit `asm { EI; HALT }` for ZX Spectrum-friendly halt that keeps interrupts enabled.

### **Inline Assembly**
```minz
fun fast_multiply(a: u8) -> u8 {
    asm {
        LD A, (a)
        SLA A        ; Multiply by 2
        SLA A        ; Multiply by 4
    }
    return a;
}
```

### **Self-Modifying Code (TRUE SMC)**
```minz
@smc
fun draw_sprite(x: u8, y: u8, sprite: *u8) -> void {
    // This function rewrites its own code for 10x performance!
    // Parameters are patched directly into instructions
}
```

---

## 💎 **Crystal Backend: Modern Development Workflow**

### **Revolutionary Dual-Platform Development**

```bash
# 1. Write MinZ with Ruby syntax
cat > game.minz << 'EOF'
const LIVES = 3;
fun game_loop() -> void {
    @print("Lives remaining: #{LIVES}");
}
EOF

# 2. Test on modern platform (Crystal/Ruby ecosystem)
mz game.minz -b crystal -o game.cr
crystal run game.cr  # Instant testing with modern tools!

# 3. Deploy to vintage hardware
mz game.minz -o game.a80  # Same code for ZX Spectrum!
```

**Benefits:**
- 🚀 **10x faster development** - No emulator needed for testing
- 🐛 **Modern debugging** - Use Crystal's debugger and profiler
- 📦 **Rich ecosystem** - Access Crystal/Ruby libraries during development
- ✅ **Proven E2E** - Crystal backend tested with complex programs

---

## 🛠️ **Complete Professional Toolchain**

| Tool | Purpose | Usage |
|------|---------|-------|
| **mz** | Multi-backend compiler | `mz program.minz -o program.a80` |
| **mza** | Native Z80 assembler | `mza program.a80 -o program.bin` |
| **mze** | Z80 emulator (100% coverage!) | `mze program.bin -v` |
| **mze debug** | DAP server for VS Code | `mze debug` |
| **mzrun** | Remote runner via DZRP | `mzrun program.minz --reset` |
| **mzr** | Interactive REPL | `mzr` for experimentation |
| **mzv** | MIR VM interpreter | `mzv program.mir` |

### **Live Testing via DZRP**

**DZRP (DeZog Remote Protocol)** enables revolutionary live development - compile, upload, and run on a real emulator in one command!

```bash
# Compile, assemble, upload and run - all in one command!
mzrun hello.minz --reset -v
```

**Recommended Emulator: [ZXSpeculator](https://github.com/oisee/ZXSpeculator)** - A modern ZX Spectrum emulator with native DZRP support, beautiful shaders, and instant feedback.

```bash
# Start ZXSpeculator with DZRP enabled
./ZXSpeculator --dzrp-port 11000

# Then from another terminal:
mzrun game.minz --reset
# Your code is running on the emulator!
```

**Why DZRP is Amazing:**
- **Instant feedback** - See your changes immediately on a real emulator
- **No file juggling** - mzrun handles compile → assemble → upload → run
- **Debug-friendly** - Works with DeZog for full source-level debugging
- **Multiple emulators** - ZXSpeculator, ZEsarUX, and more support DZRP

### **Debug & Analysis Flags**
```bash
mz program.minz --dump-mir          # Output MIR to stdout
mz program.minz --viz graph.dot     # MIR visualization (Graphviz)
mz program.minz --dump-ast          # AST in JSON format
mz program.minz -d                  # Compilation details
```

**All tools are self-contained with zero dependencies!**

> **Coming Soon:** `--debug-info` flag for source maps (MinZ → MIR → Z80 address mapping) to enable full source-level debugging in VS Code.

---

## 📚 **Standard Library**

MinZ includes a comprehensive stdlib optimized for Z80/retro game development:

| Module | Description | Functions |
|--------|-------------|-----------|
| `math/fast` | Lookup tables | `fast_sin()`, `fast_cos()`, `fast_sqrt()` |
| `math/random` | PRNG & noise | `rand8()`, `rand_range()`, `noise2d()` |
| `graphics/screen` | Pixel graphics | `set_pixel()`, `draw_line()`, `draw_circle()` |
| `input/keyboard` | Input handling | `is_key_pressed()`, `get_key_dx()` |
| `text/string` | String ops | `strlen()`, `strcmp()`, `strcpy()` |
| `text/format` | Formatting | `u8_to_str()`, `u16_to_hex()` |
| `sound/beep` | Beeper SFX | `beep()`, `sfx_jump()`, `sfx_explosion()` |
| `time/delay` | Timing | `wait_frame()`, `delay_ms()`, `anim_frame()` |
| `mem/copy` | Fast memory | `memcpy()`, `memset()`, `memcmp()` |
| `cpm/bdos` | CP/M calls | `getchar()`, `putchar()`, `file_open()` |

### **Example: Simple Game Loop**
```minz
import stdlib.graphics.screen;
import stdlib.input.keyboard;
import stdlib.time.delay;

fun main() -> void {
    clear_screen();
    set_ink(COLOR_CYAN);

    let x: u8 = 128;
    let y: u8 = 96;

    loop {
        wait_frame();
        clear_pixel(x, y);
        x = (x + get_key_dx()) as u8;
        y = (y + get_key_dy()) as u8;
        set_pixel(x, y);
    }
}
```

---

## 📊 **Performance & Metrics**

### **Compilation Success Rate**
- ✅ **100%** of core examples compile successfully (72/72)
- 18 experimental examples in `examples/experimental/` (4 now passing with generics!)
- **Generics with monomorphization** now working!
- Zero-cost interface methods working!
- See [Release Notes](reports/2025-12-18-003-release-notes.md) for details

### **Optimization Impact**
| Feature | Performance Gain | Method |
|---------|-----------------|--------|
| **CTIE** | 3-5x faster | Compile-time execution |
| **TRUE SMC** | 10x faster | Self-modifying code |
| **MIR Optimizer** | 15-30% faster | Constant folding, strength reduction, DCE |
| **Assembly Peephole** | 60-85% size reduction | 35+ Z80-specific patterns |

**NEW in v0.15.5:** Two-level optimization pipeline with MIR-level optimizations (constant folding, algebraic simplification, strength reduction, copy propagation, dead code elimination) running before assembly peephole. SDCC-competitive code generation!

### **Language Statistics**
- **15 major versions** in 14 months
- **8 backends** (Z80, 6502, 68000, GB, WASM, C, Crystal, LLVM)
- **280+ documentation** files
- **Zero dependencies** - Pure Go implementation

---

## 🎯 **Platform Support**

### **Vintage Targets**
| Platform | CPU | Status | Usage |
|----------|-----|--------|-------|
| ZX Spectrum | Z80 | ✅ Stable | `mz -t spectrum` |
| ZX Spectrum Next | Z80N | 🚧 Planned | `mz -t specnext` |
| Agon Light | eZ80 | 🚧 Planned | `mz -t agon` |
| Commodore 64 | 6502 | ✅ Stable | `mz -b 6502` |
| CP/M Systems | Z80 | ✅ Stable | `mz -t cpm` |
| MSX | Z80 | ✅ Stable | `mz -t msx` |
| Amstrad CPC | Z80 | ✅ Stable | `mz -t cpc` |
| Game Boy | SM83 | 🚧 Beta | `mz -b gb` |

### **Modern Targets**
| Platform | Backend | Status | Usage |
|----------|---------|--------|-------|
| Crystal | Crystal | ✅ Stable | `mz -b crystal` |
| WebAssembly | WASM | ✅ Stable | `mz -b wasm` |
| Native C | C99 | ✅ Stable | `mz -b c` |
| LLVM IR | LLVM | ✅ Stable | `mz -b llvm` |

---

## 📖 **Documentation**

### **Essential Guides**
- 📚 [Complete Language Specification](docs/230_MinZ_Complete_Language_Specification.md)
- 🚀 [Quick Start](#-quick-start) (see above)
- 💎 [Crystal Backend Guide](docs/274_Crystal_Backend_Implementation_Complete.md)
- 🏆 [Evolution Journey](docs/277_MinZ_Complete_Evolution_Journey.md)
- 📅 [Revolutionary Timeline](docs/278_MinZ_Revolutionary_Timeline.md)

### **Advanced Topics**
- 🔬 [CTIE: Compile-Time Execution](docs/178_CTIE_Working_Announcement.md)
- 🎯 [True Self-Modifying Code](docs/145_TSMC_Complete_Philosophy.md)
- 🔄 [Lambda Iterator Implementation](docs/141_Lambda_Iterator_Revolution_Complete.md)
- 📦 [Module System Design](docs/191_Module_System_Design.md)

### **Architecture & Roadmap**
- 🏗️ [Compiler Architecture](minzc/docs/INTERNAL_ARCHITECTURE.md)
- 🔧 [MIR Intermediate Representation](docs/126_MIR_Interpreter_Design.md)
- ⚡ [Optimization Pipeline](docs/149_World_Class_Multi_Level_Optimization_Guide.md)
- 🐛 [DAP Debugger Architecture](docs/262_Debugger_Architecture_Plan.md)
- 🎮 [Z80N & eZ80 Support Vision](docs/266_Z80N_eZ80_Extended_Support_Vision.md)
- 🔮 [MinZ 2026 Vision](docs/263_MinZ_2026_Vision.md)

### **Current Status & Analysis**
- 📊 [Parser Analysis Report](docs/264_Parser_Analysis_Report.md) - Tree-sitter at 100% accuracy
- ⚠️ [Known Semantic Issues](docs/265_Known_Semantic_Issues.md) - 4 documented issues, 92% success
- 📈 [Code Generation Analysis](docs/260_MinZ_Code_Generation_Analysis.md) - Assembly output quality

---

## 🌟 **Why Choose MinZ?**

### **For Retro Enthusiasts**
- ✅ Modern syntax for vintage hardware
- ✅ Professional tooling for hobby projects
- ✅ No assembly knowledge required
- ✅ Active community and development

### **For Modern Developers**
- ✅ Learn retro computing with familiar syntax
- ✅ Test algorithms on constrained hardware
- ✅ Bridge to embedded systems programming
- ✅ Unique resume skill

### **For Educators**
- ✅ Teach systems programming concepts
- ✅ Demonstrate optimization techniques
- ✅ Show evolution of computing
- ✅ Hands-on hardware interaction

---

## 🤝 **Contributing**

We welcome contributions! MinZ is built by a passionate community.

### **How to Contribute**
1. **Report Issues** - Found a bug? [Open an issue](https://github.com/oisee/minz/issues)
2. **Submit PRs** - Fix bugs or add features
3. **Write Docs** - Help others learn MinZ
4. **Share Projects** - Show what you built!

### **Development Setup**
```bash
# Prerequisites
npm install -g tree-sitter-cli
tree-sitter init-config  # REQUIRED for parsing

# Clone and build
git clone https://github.com/oisee/minz.git
cd minz

# Generate tree-sitter parser
tree-sitter generate

# Build compiler and tools
cd minzc
go build -o mz cmd/minzc/main.go
go build -o mza cmd/mza/main.go   # Assembler
go build -o mze cmd/mze/main.go   # Emulator

# Test
./mz ../examples/simple_add.minz -o /tmp/test.a80
```

---

## 📜 **License**

MinZ is MIT licensed. See [LICENSE](LICENSE) for details.

---

## 🎉 **Join the Revolution!**

MinZ proves that modern programming belongs on vintage hardware. Join us in building the future of retro computing!

<div align="center">

**⭐ Star this repo to support the project!**

[Discord](https://discord.gg/minz) • [Twitter](https://twitter.com/minzlang) • [Website](https://minz-lang.org)

### **MinZ: Where Modern Dreams Meet Vintage Reality™**

*From v0.1.0 to v0.16.0 and beyond - Every release a revolution!*

> **v0.16.0 Highlights:** DAP Debugger for VS Code, Z80N/eZ80 roadmap, 92% compilation success!

> ⚠️ **Remember:** MinZ is under active development. Join us in building the future of retro computing!

</div>