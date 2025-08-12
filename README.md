# MinZ Programming Language
![](/media/minz-logo-shamrock-mint.png)

**A modern systems programming language for retro computers** (Z80, 6502, Game Boy, WebAssembly, LLVM)

## 🎉 v0.13.1 Hotfix (January 2025)

**Installation Fix for Ubuntu/Linux** - Resolved "Expected source code but got an atom" error. Includes dependency installer script and better error messages. [Get v0.13.1](https://github.com/oisee/minz/releases/tag/v0.13.1)

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

## 💻 **Installation & Usage**

### Quick Install (Ubuntu/Linux)
```bash
# Download v0.13.1 (includes dependency installer)
wget https://github.com/oisee/minz/releases/download/v0.13.1/minz-v0.13.1-linux-amd64.tar.gz
tar -xzf minz-v0.13.1-linux-amd64.tar.gz
cd linux-amd64
./install-dependencies.sh  # Install tree-sitter (one-time)
./install.sh               # Install MinZ

# Compile a program
mz program.minz -o program.a80

# With optimization
mz program.minz -O --enable-ctie -o program.a80

# Target specific platform
mz program.minz --target=cpm -o program.com
```

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

## 🎉 **Release History**

### Recent Releases
- **v0.13.0** (Aug 2025) - Module System Revolution
- **v0.12.0** (Aug 2025) - Compile-Time Interface Execution (CTIE)
- **v0.10.0** (Aug 2025) - Zero-Cost Lambda Iterators
- **v0.9.6** (Jul 2025) - Function Overloading & Interface Methods
- **v0.9.0** (Jul 2025) - Error Propagation System

[Full changelog](CHANGELOG.md) | [All release notes](docs/RELEASE_NOTES.md)

---

**MinZ**: Modern abstractions, vintage performance. The future of retro computing! 🚀