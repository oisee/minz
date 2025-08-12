# MinZ Programming Language
![](/media/minz-logo-shamrock-mint.png)

**A modern systems programming language for retro computers** (Z80, 6502, Game Boy, WebAssembly, LLVM)

## 🎉 v0.13.0 Alpha "Module Revolution" (August 2025)

### 🚀 **NEW: Complete Module System!**

MinZ now has a **modern module import system** with standard library support!

```minz
import std;           // Standard library
import zx.screen;     // ZX Spectrum graphics

fun main() -> void {
    std.cls();                    // Clear screen
    std.println("Hello, MinZ!");  // Print with newline
    zx.screen.set_border(2);      // Red border on ZX Spectrum
}
```

### ✨ **Key Features in v0.13.0**

- **📦 Module Imports** - Clean namespace with `import` statements
- **🎯 Standard Library** - Built-in functions: `print`, `println`, `cls`, `hex`, `abs`, `min`, `max`
- **🖥️ Platform Modules** - Hardware-specific: `zx.screen`, `zx.input`, `zx.sound` (ZX Spectrum)
- **✅ 70% Compilation Success** - Major stability improvements
- **🔥 All Previous Features** - CTIE, lambdas, overloading, interfaces still working!

### 📊 **Current Metrics**
- **Module System**: 100% functional
- **Compilation Success**: 70% (up from 69%)
- **Standard Library**: 15+ functions implemented
- **Platform Support**: ZX Spectrum, CP/M, MSX, CPC
- **Optimization**: 3-5x with CTIE, 60-85% with peephole

## 💻 **Installation & Usage**

```bash
# Install MinZ compiler
curl -L https://github.com/minz-lang/minz/releases/latest/download/minz-installer.sh | bash

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

- **[Quick Start Guide](docs/QUICK_START.md)** - Get coding in 5 minutes
- **[Language Reference](docs/LANGUAGE_REFERENCE.md)** - Complete syntax guide
- **[Module System Guide](docs/191_Module_System_Design.md)** - Using modules and imports
- **[Platform Guide](docs/150_Platform_Independence_Achievement.md)** - Multi-platform development
- **[Optimization Guide](docs/149_World_Class_Multi_Level_Optimization_Guide.md)** - Performance tuning

### 🏗️ **Architecture & Internals**
- [Compiler Architecture](minzc/docs/INTERNAL_ARCHITECTURE.md) - How MinZ works internally
- [CTIE Design](docs/178_CTIE_Working_Announcement.md) - Compile-time execution system
- [Lambda Implementation](docs/141_Lambda_Iterator_Revolution_Complete.md) - Zero-cost iterators

### 🚀 **Roadmaps**
- [Stability Roadmap](STABILITY_ROADMAP.md) - Path to v1.0
- [Development Roadmap 2025](docs/129_Development_Roadmap_2025.md) - Current priorities

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