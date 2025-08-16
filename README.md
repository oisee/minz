# MinZ Programming Language
![](/media/minz-logo-shamrock-mint.png)

**A modern systems programming language for retro computers** (Z80, 6502, Game Boy, WebAssembly, LLVM)

> ⚠️ **Under Construction** - MinZ is in active development (v0.14.0 Alpha). APIs and syntax may change.

## 📚 Quick Links
- **[Complete Language Specification](docs/230_MinZ_Complete_Language_Specification.md)** - Full syntax, features, and implementation details
- **[Metafunction Design Guide](docs/226_Metafunction_Design_Decisions.md)** - @minz, @define, @lua explained
- **[Tree-Shaking Report](docs/225_Tree_Shaking_Implementation_E2E_Report.md)** - 74% size reduction achieved
- **[Development Roadmap](docs/228_MinZ_Specification_Article_Plan.md)** - Architecture and future plans

## 🎊 v0.14.0: Tree-Shaking & Metafunction Revolution! (January 2025)

### 🚀 Major Achievements

**Tree-Shaking Optimization** - 74% size reduction!
- Only includes used stdlib functions
- 324 lines → 85 lines typical output
- Critical for Z80 where every byte counts

**Metafunction System Clarified**
- `@minz[[[...]]]` - Immediate compile-time execution (no args!)
- `@define(...)` - Template preprocessor (fully working!)
- `@lua[[[...]]]` - Lua scripting at compile-time
- See [Metafunction Design Guide](docs/226_Metafunction_Design_Decisions.md)

**Complete Toolchain**
- `mz` - Compiler with 8 backends
- `mza` - Assembler with macro support
- `mze` - Emulator with full debugger
- `mzr` - REPL with history
- `mzv` - MIR VM interpreter

[Get v0.14.0](https://github.com/oisee/minz/releases/tag/v0.14.0) | [Full Report](docs/227_E2E_Super_Session_Complete_Implementation_Report.md)

## 📦 v0.13.0 Alpha "Module Revolution" (January 2025)

### 🚀 **NEW: Complete Module System with Aliasing!**

MinZ now has a **professional module system** with aliasing and file-based imports!

```minz
import std as io;         // Alias standard library
import math as m;         // File-based module from stdlib/
import zx.screen as gfx;  // Platform module with alias

fun main() -> void {
    io.cls();                     // Using alias
    io.println("Hello, MinZ!");   
    let sq = m.square(5);         // Math module via alias
    gfx.set_border(2);            // Platform graphics
}
```

### ✨ **Key Features in v0.13.0**

- **📦 Module Aliasing** - Import with custom names: `import math as m`
- **📁 File-Based Modules** - Load from `stdlib/` directory
- **🎯 Expanded Standard Library** - 20+ functions across modules
- **🖥️ Complete Platform Support** - Input, sound, graphics for ZX Spectrum
- **✅ 85% Compilation Success** - Major improvement from 69%!
- **🔥 Zero-Cost Abstractions** - Modules compile to direct calls

### 📊 **Current Metrics**
- **Module System**: File-based + aliasing complete
- **Compilation Success**: **85%** (126/148 examples)
- **Standard Library**: 25+ functions across modules
- **Platform Support**: ZX Spectrum (complete I/O), CP/M, MSX, CPC
- **Optimization**: 3-5x with CTIE, 60-85% with peephole

## 🚀 Quick Start

### Hello World
```minz
fun main() -> u8 {
    @print("Hello, MinZ!");
    return 0;
}
```

### Core Features at a Glance
```minz
// Modern syntax, zero-cost abstractions
let numbers = [1, 2, 3, 4, 5];
let sum = numbers.iter()
    .filter(|x| x > 2)
    .map(|x| x * 2)
    .sum();  // Compiles to optimal DJNZ loop!

// Metaprogramming
@define(getter, field, type)[[[
    fun get_{0}() -> {1} { return self.{0}; }
]]]
@define("x", "u8")  // Generates get_x() function

// Compile-time code generation
@minz[[[
    for i in 0..4 {
        @emit("fun handler_" + i + "() -> void { }")
    }
]]]
```

See the **[Complete Language Specification](docs/230_MinZ_Complete_Language_Specification.md)** for full syntax and features.

## 💻 **Installation & Usage**

### Quick Install (All Platforms) - v0.14.0

```bash
# Linux/macOS
wget https://github.com/oisee/minz/releases/download/v0.14.0/minz-v0.14.0-$(uname -s)-$(uname -m).tar.gz
tar -xzf minz-v0.14.0-*.tar.gz
sudo cp mz /usr/local/bin/

# Windows (PowerShell)
Invoke-WebRequest -Uri "https://github.com/oisee/minz/releases/download/v0.14.0/minz-v0.14.0-windows-amd64.zip" -OutFile "minz.zip"
Expand-Archive minz.zip
Copy-Item minz\mz.exe C:\Windows\System32\

# Verify installation (zero dependencies!)
mz --version

# Compile a program (uses ANTLR by default - 75% success rate!)
mz program.minz -o program.a80

# Parser options (v0.14.0)
mz program.minz -o program.a80              # Default: ANTLR (75% success)
MINZ_USE_TREE_SITTER=1 mz program.minz      # Fallback: tree-sitter (70% success)

# With optimization
mz program.minz -O --enable-ctie -o program.a80

# Target specific platform  
mz program.minz --target=cpm -o program.com
```

### Other Platforms

- **macOS**: [Download](https://github.com/oisee/minz/releases/download/v0.13.2/minz-v0.13.2-darwin-arm64.tar.gz)
- **Windows**: [Download](https://github.com/oisee/minz/releases/download/v0.13.2/minz-v0.13.2-windows-amd64.zip)
- **All Platforms**: [View all builds](https://github.com/oisee/minz/releases/tag/v0.13.2)

## 🔧 **Parser System (v0.14.0 Update)**

MinZ now defaults to the **ANTLR parser** for better compatibility and zero dependencies:

### ANTLR Parser (DEFAULT since v0.14.0)
```bash
# 75% success rate, zero dependencies, pure Go
mz program.minz -o program.a80
# No external tools needed - fully self-contained!
```

### Tree-sitter Parser (Legacy Fallback)
```bash
# Zero CGO dependencies, works everywhere
MINZ_USE_ANTLR_PARSER=1 mz program.minz -o program.a80
# Perfect for Docker, CI/CD, and cross-compilation
```

### Automatic Fallback
The compiler automatically falls back to ANTLR if the native parser fails, ensuring **100% compatibility** across all environments.

### Performance Comparison
| Parser | Speed | Memory | Dependencies | Use Case |
|--------|-------|---------|-------------|----------|
| Native | ⚡ Fastest | 💾 Low | CGO required | Development |
| ANTLR | 🚀 Fast | 💾 Medium | None | Production/CI |

## 📖 **Quick Language Tour**

### Modern Module System (NEW!)
```minz
import std;              // Standard library
import zx.screen as gfx; // Aliased import

fun draw_border(color: u8) -> void {
    gfx.set_border(color);
    std.print("Border color: ");
    std.hex(color);
}
```

### Zero-Cost Lambda Iterators
```minz
// Compiles to optimal DJNZ loops!
enemies.iter()
    .filter(|e| e.alive)
    .map(|e| update_ai(e))
    .forEach(|e| e.draw());
```

### Compile-Time Interface Execution (CTIE)
```minz
// This function executes at compile-time and vanishes!
@ctie
fun distance(x1: u8, y1: u8, x2: u8, y2: u8) -> u8 {
    let dx = abs(x2 - x1);
    let dy = abs(y2 - y1);
    return max(dx, dy);  // Chebyshev distance
}

// Compiled as: LD A, 7  (result computed at compile-time!)
let d = distance(3, 4, 10, 8);
```

### Function Overloading & Interfaces
```minz
// Clean overloaded print
print(42);        // Calls print$u8
print("Hello");   // Calls print$String
print(true);      // Calls print$bool

// Natural interface methods
circle.draw();    // Zero-cost dispatch
rect.get_area();  // No vtables!
```

### Error Propagation
```minz
fun risky_op?() -> u8 ? Error {
    let result = dangerous_call?() ?? 0;  // Default on error
    return result;
}
```

## 🎯 **Platform Support**

| Platform | Backend | Status | Target Flag |
|----------|---------|--------|-------------|
| ZX Spectrum | Z80 | ✅ Stable | `--target=zx` (default) |
| CP/M | Z80 | ✅ Stable | `--target=cpm` |
| MSX | Z80 | ✅ Stable | `--target=msx` |
| Amstrad CPC | Z80 | ✅ Stable | `--target=cpc` |
| Commodore 64 | 6502 | 🚧 Beta | `-b 6502` |
| Game Boy | GB | 🚧 Beta | `-b gb` |
| WebAssembly | WASM | 🚧 Alpha | `-b wasm` |

## 📚 **Documentation**

### 📖 **Essential Guides**
- **[Quick Start Guide](docs/QUICK_START.md)** - Get coding in 5 minutes
- **[Language Reference](docs/LANGUAGE_REFERENCE.md)** - Complete syntax guide
- **[Module System Guide](docs/191_Module_System_Design.md)** - Using modules and imports
- **[Platform Guide](docs/150_Platform_Independence_Achievement.md)** - Multi-platform development
- **[Optimization Guide](docs/149_World_Class_Multi_Level_Optimization_Guide.md)** - Performance tuning

### 🏗️ **Architecture & Internals**
- [Compiler Architecture](minzc/docs/INTERNAL_ARCHITECTURE.md) - How MinZ works internally
- [CTIE Design](docs/178_CTIE_Working_Announcement.md) - Compile-time execution system
- [Lambda Implementation](docs/141_Lambda_Iterator_Revolution_Complete.md) - Zero-cost iterators
- **[VM & Bytecode Vision](docs/198_VM_Bytecode_Targets_and_MIR_Runtime_Vision.md)** - Future runtime plans

### 🎯 **Next Goals (v0.14.0)**
- **String Manipulation** - Complete string library
- **File I/O** - Platform-independent file operations  
- **Collections** - Lists, maps, sets with zero-cost abstractions
- **Package Manager** - Dependency management system
- **MIR VM** - Universal runtime from CTIE interpreter

### 🚀 **Roadmaps**
- [Stability Roadmap](STABILITY_ROADMAP.md) - Path to v1.0
- [Development Roadmap 2025](docs/129_Development_Roadmap_2025.md) - Current priorities
- [Feature Status](FEATURE_STATUS.md) - Detailed completion tracking

## 🏆 **Revolutionary Features**

### **World's First on 8-bit:**
- ✅ **Module System** - Clean imports and namespaces (v0.13.0)
- ✅ **Negative-Cost Abstractions** - CTIE executes at compile-time (v0.12.0)
- ✅ **Zero-Cost Lambdas** - Functional programming without overhead (v0.10.0)
- ✅ **Function Overloading** - Multiple dispatch on Z80 (v0.9.6)
- ✅ **Error Propagation** - Modern error handling with `?` operator (v0.9.0)

## 🔧 **Build from Source**

```bash
# Clone repository
git clone https://github.com/minz-lang/minz.git
cd minz/minzc

# Build compiler
go build -o mz cmd/minzc/main.go

# Run tests
./test_all.sh

# Install
sudo cp mz /usr/local/bin/
```

## 📈 **Performance**

MinZ generates **hand-optimized** Z80 assembly:

| Feature | Performance | Notes |
|---------|------------|-------|
| Module imports | Zero-cost | Resolved at compile-time |
| CTIE functions | -100% cost | Execute during compilation |
| Lambda iterators | 0% overhead | Identical to manual loops |
| Interface calls | 0% overhead | Direct dispatch, no vtables |
| Error propagation | ~5 cycles | Minimal branching |
| Function overload | 0% overhead | Resolved at compile-time |

## 🎮 **Example: Game Loop with Modules**

```minz
import std;
import zx.screen;
import zx.input;

struct Player {
    x: u8,
    y: u8,
    score: u16
}

fun main() -> void {
    std.cls();
    zx.screen.set_border(1);  // Blue border
    
    let mut player = Player { x: 128, y: 96, score: 0 };
    
    loop {
        // Read input
        if zx.input.is_key_pressed('W') { player.y -= 1; }
        if zx.input.is_key_pressed('S') { player.y += 1; }
        
        // Update game
        player.score += 1;
        
        // Draw
        zx.screen.set_pixel(player.x, player.y);
        std.print("Score: ");
        std.println(player.score);
        
        // 50Hz frame sync
        wait_vblank();
    }
}
```

## 🤝 **Contributing**

We welcome contributions! See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

**Key areas needing help:**
- Standard library functions
- Platform-specific modules
- Documentation and examples
- Backend implementations (6502, Game Boy)

## 📜 **License**

MIT License - See [LICENSE](LICENSE) for details.

## 📖 **Documentation**

### Essential Reading
- **[Language Specification](docs/230_MinZ_Complete_Language_Specification.md)** - Complete reference with cheatsheet
- **[TSMC Philosophy](docs/145_TSMC_Complete_Philosophy.md)** - True Self-Modifying Code explained
- **[Optimization Guide](docs/149_World_Class_Multi_Level_Optimization_Guide.md)** - How we achieve zero-cost
- **[Internal Architecture](minzc/docs/INTERNAL_ARCHITECTURE.md)** - Compiler internals
- **[Session Report](docs/229_Session_Achievement_Report_v0_14_0.md)** - Latest achievements

### Implementation Status
- **Core Features:** 80% complete
- **Compilation Success:** 63% (tree-sitter), 75% (ANTLR)
- **Tree-Shaking:** 74% size reduction
- **Documentation:** 230+ detailed documents

## 🎉 **Release History**

### Recent Releases
- **v0.14.0** (Jan 2025) - Tree-Shaking & Metafunction Clarification
- **v0.13.0** (Jan 2025) - Module System Revolution
- **v0.12.0** (Dec 2024) - Compile-Time Interface Execution (CTIE)
- **v0.10.0** (Nov 2024) - Zero-Cost Lambda Iterators
- **v0.9.6** (Oct 2024) - Function Overloading & Interface Methods
- **v0.9.0** (Sep 2024) - Error Propagation System

[Full changelog](CHANGELOG.md) | [All releases](https://github.com/oisee/minz/releases)

---

**MinZ**: Modern abstractions, vintage performance, zero compromises. 🚀

> ⚠️ **Remember:** MinZ is under active development. Join us in building the future of retro computing!