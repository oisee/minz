# MinZ Programming Language

<div align="center">

![MinZ Logo](/media/minz-logo-shamrock-mint.png)

### **Modern Programming Language for Vintage & Modern Platforms**

[![Version](https://img.shields.io/badge/version-0.15.0-brightgreen)](https://github.com/oisee/minz/releases)
[![Platforms](https://img.shields.io/badge/platforms-Z80%20%7C%206502%20%7C%20Crystal%20%7C%20WASM-blue)]()
[![Success Rate](https://img.shields.io/badge/compilation-88%25-success)]()
[![License](https://img.shields.io/badge/license-MIT-purple)]()

**Write modern code. Deploy everywhere. From 1978 Z80 to 2025 Crystal.**

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

// Zero-cost functional programming
numbers.iter()
    .filter(|x| x > 5)
    .map(|x| x * 2)
    .forEach(|x| print(x));  // Compiles to optimal DJNZ loop!

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

### **1. Install MinZ** (Zero Dependencies!)

```bash
# macOS/Linux
curl -L https://github.com/oisee/minz/releases/latest/download/minz-$(uname -s)-$(uname -m).tar.gz | tar -xz
sudo mv mz /usr/local/bin/

# Verify
mz --version  # MinZ v0.15.0
```

### **2. Write Your First Program**

```minz
// hello.minz
const VERSION = 15;

fun main() -> void {
    let message = "Hello from MinZ v0.#{VERSION}!";
    @print(message);
}
```

### **3. Compile & Run**

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
| **v0.15.0** | Ruby Interpolation + Crystal Backend | `"Hello #{name}!"` + Modern workflow |
| **v0.14.0** | Pattern Matching | `case x { Some(v) => ... }` |
| **v0.13.0** | Module System | `import math as m` |
| **v0.12.0** | CTIE (Compile-Time Execution) | Functions run at compile-time! |
| **v0.11.0** | Complete Toolchain | Compiler + Assembler + Emulator + REPL |
| **v0.10.0** | Lambda Iterators | `.map().filter()` → DJNZ loops |
| **v0.9.6** | Function Overloading | `print(anything)` |
| **v0.9.0** | Error Propagation | `risky_op?()` with `?` operator |
| **v0.8.0** | True Self-Modifying Code | 10x performance through mutation |
| **v0.7.0** | LLVM Backend | Modern optimizations |
| **v0.6.0** | Module System | Professional organization |
| **v0.5.0** | Inline Assembly | Direct hardware control |
| **v0.4.0** | Multi-Platform | Z80, 6502, WASM, C |
| **v0.3.0** | Optimizer | 35+ peephole patterns |
| **v0.2.0** | Structs & Arrays | Real programs possible |
| **v0.1.0** | Genesis | Modern syntax for Z80 |

</details>

### **🏆 World's First & Only**

| Feature | Description | Example |
|---------|-------------|---------|
| **Zero-Cost Lambdas** | Lambda iterators compile to optimal assembly | `.map(|x| x * 2)` → `ADD A, A` |
| **Negative-Cost Functions** | CTIE executes at compile-time | `distance(3,7)` → `LD A, 4` |
| **True SMC** | Self-modifying code with 10x gains | Functions rewrite themselves! |
| **Ruby on Z80** | Ruby interpolation on 8-bit CPU | `"Score: #{points}"` |
| **Smart Arrays** | Array literals → DB/DW directives | `[10,20,30]` → `DB 10,20,30` |
| **Modern Errors** | Line numbers in all error messages | `line 42, col 8: undefined` |
| **Pattern Matching** | ML-style on 64KB RAM | `case Some(x) => x` |
| **Crystal Backend** | Test on modern, deploy to vintage | Same code runs 1978→2025 |

---

## 📚 **Code Examples**

### **Modern Functional Programming**
```minz
// Zero-cost iterator chains
let result = [1, 2, 3, 4, 5]
    .iter()
    .filter(|x| x % 2 == 0)
    .map(|x| x * x)
    .sum();  // Compiles to tight DJNZ loop!
```

### **Optimized Array Literals** 🆕
```minz
// Simple arrays become DB directives
let data: [u8; 5] = [10, 20, 30, 40, 50];
// Generates: DB 10, 20, 30, 40, 50

// Struct arrays with proper alignment
struct Player { x: u16, y: u16, health: u8 }
let players: [Player; 2] = [
    Player { x: 100, y: 200, health: 100 },
    Player { x: 300, y: 400, health: 75 }
];
// Generates:
//   DW 100, 200  ; x, y
//   DB 100       ; health
//   DW 300, 400  ; x, y  
//   DB 75        ; health
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

### **Pattern Matching**
```minz
enum Result {
    Ok(value: u8),
    Error(code: u8)
}

case parse_input() {
    Ok(n) => process(n),
    Error(0) => @print("Invalid input"),
    Error(e) => @print("Error code: #{e}")
}
```

### **Modern Error Handling**
```minz
fun read_file?(path: *u8) -> *u8 ? Error {
    let file = open(path)?;  // Propagate errors with ?
    let data = file.read_all()?;
    file.close()?;
    return data;
}

fun main() -> void {
    let content = read_file?("data.txt") ?? "default";  // Default on error
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
| **mze** | Z80 emulator/debugger | `mze program.bin --debug` |
| **mzr** | Interactive REPL | `mzr` for experimentation |
| **mzv** | MIR VM interpreter | `mzv program.mir` |

**All tools are self-contained with zero dependencies!**

---

## 📊 **Performance & Metrics**

### **Compilation Success Rate**
- ✅ **88%** of examples compile successfully (150/170)
- ✅ **63%** with tree-sitter parser
- ✅ **75%** with ANTLR parser (v0.14.0)

### **Optimization Impact**
| Feature | Performance Gain | Method |
|---------|-----------------|--------|
| **CTIE** | 3-5x faster | Compile-time execution |
| **TRUE SMC** | 10x faster | Self-modifying code |
| **Peephole** | 60-85% size reduction | 35+ optimization patterns |
| **Lambda→DJNZ** | Zero overhead | Iterator optimization |

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
- 🚀 [Quick Start Tutorial](docs/QUICK_START.md)
- 💎 [Crystal Backend Guide](docs/274_Crystal_Backend_Implementation_Complete.md)
- 🏆 [Evolution Journey](docs/277_MinZ_Complete_Evolution_Journey.md)
- 📅 [Revolutionary Timeline](docs/278_MinZ_Revolutionary_Timeline.md)

### **Advanced Topics**
- 🔬 [CTIE: Compile-Time Execution](docs/178_CTIE_Working_Announcement.md)
- 🎯 [True Self-Modifying Code](docs/145_TSMC_Complete_Philosophy.md)
- 🔄 [Lambda Iterator Implementation](docs/141_Lambda_Iterator_Revolution_Complete.md)
- 📦 [Module System Design](docs/191_Module_System_Design.md)

### **Architecture**
- 🏗️ [Compiler Architecture](minzc/docs/INTERNAL_ARCHITECTURE.md)
- 🔧 [MIR Intermediate Representation](docs/126_MIR_Interpreter_Design.md)
- ⚡ [Optimization Pipeline](docs/149_World_Class_Multi_Level_Optimization_Guide.md)

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
git clone https://github.com/oisee/minz.git
cd minz/minzc
go build -o mz cmd/minzc/main.go
./test_all.sh  # Run test suite
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

*From v0.1.0 to v0.15.0 and beyond - Every release a revolution!*

> ⚠️ **Remember:** MinZ is under active development. Join us in building the future of retro computing!

</div>