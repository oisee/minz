# 129: MinZ Tools - Complete Guide

## 🚀 MinZ Development Environment

The MinZ toolchain provides a complete Z80 development environment with modern tooling.

## 📦 Installation

### Quick Install (Recommended)
```bash
git clone https://github.com/oisee/minz.git
cd minz
./install.sh  # Installs to ~/.local/bin (no sudo needed)
```

### Manual Build
```bash
cd minzc
make all      # Build both compiler and REPL
make install  # Install to /usr/local/bin (requires sudo)
```

## 🛠️ Available Tools

### `mz` - MinZ Compiler
The main compiler that transforms MinZ source code to Z80 assembly.

```bash
# Basic compilation
mz program.minz -o program.a80

# With optimizations
mz program.minz -O

# With TRUE SMC (Self-Modifying Code)
mz program.minz -O --enable-smc

# Debug mode
mz program.minz -d
```

**Options:**
- `-o <file>` - Output file (default: input.a80)
- `-O` - Enable optimizations
- `--enable-smc` - Enable all SMC optimizations including TRUE SMC
- `-d` - Debug output

### `mzr` - MinZ REPL
Interactive development environment with Z80 emulation and ZX Spectrum screen.

```bash
# Start REPL
mzr

# Alternative name for compatibility
minz
```

## 🎮 REPL Features

### Interactive Compilation
```
minz> let x: u8 = 42
Variable 'x' defined

minz> fun double(n: u8) -> u8 { return n * 2; }
Function 'double' defined at 0x8000

minz> double(x)
84
```

### Command Reference

#### Quick Shortcuts
| Short | Long | Description |
|-------|------|-------------|
| `/h` | `/help` | Show help |
| `/q` | `/quit` | Exit REPL |
| `/r` | `/reg` | Show Z80 registers |
| `/rc` | `/regc` | Compact register view |
| `/s` | `/screen` | Show ZX Spectrum screen |
| `/ss` | `/screens` | Toggle auto-show screen |
| `/v` | `/vars` | Show variables |
| `/f` | `/funcs` | Show functions |
| `/m` | `/mem` | Show memory |

### ZX Spectrum Screen Emulation

The REPL includes a full 32x24 character ZX Spectrum screen emulator:

```minz
// Output via RST 16 (ROM print routine)
@abi("register: A=char")
fun putchar(c: u8) -> void {
    asm { RST 0x10 }
}

// Input via RST 18 (ROM input routine)
@abi("register: A=result")
fun getchar() -> u8 {
    asm { RST 0x18 }
}

fun main() -> void {
    putchar('H');
    putchar('i');
    putchar('!');
}
```

View the screen:
```
minz> /s
╔════════════════════════════════╗
║Hi!                             ║
║                                ║
║                                ║
...
╚════════════════════════════════╝
```

### Register Display

Full Z80 register state including shadow registers:

```
minz> /r
╔══════════════════════════════════════════════════════════════╗
║                    Z80 Register State                        ║
╠══════════════════════════════════════════════════════════════╣
║ AF =0054  BC =0000  DE =0000  HL =0054                      ║
║ AF'=0000  BC'=0000  DE'=0000  HL'=0000                      ║
║ IX =0000  IY =0000  SP =FFFF  PC =8010                      ║
║ I  =00    R  =00    IFF1=false  IFF2=false  IM=0            ║
╠══════════════════════════════════════════════════════════════╣
║ Flags: S=0 Z=0 H=0 P/V=0 N=0 C=0                            ║
╚══════════════════════════════════════════════════════════════╝
```

Compact view:
```
minz> /rc
AF=0054 BC=0000 DE=0000 HL=0054 IX=0000 IY=0000 SP=FFFF PC=8010
```

## 🏗️ Build System

### Makefile Targets

| Target | Description |
|--------|-------------|
| `make all` | Build both compiler and REPL |
| `make build` | Build compiler only |
| `make repl` | Build REPL only |
| `make test` | Run test suite |
| `make clean` | Clean build artifacts |
| `make install` | Install to /usr/local/bin |
| `make run` | Build and test on example |
| `make run-repl` | Build and run REPL |
| `make benchmark` | Run performance benchmarks |
| `make release` | Build for all platforms |

## 🔧 Configuration

### Environment Variables
```bash
# Add to PATH if using local install
export PATH="$HOME/.local/bin:$PATH"

# Set default optimization level
export MINZ_OPT_LEVEL=2

# Enable SMC by default
export MINZ_ENABLE_SMC=1
```

### REPL Configuration
Future versions will support `~/.minzrc` for REPL settings:
```
# .minzrc
auto_show_screen = true
default_origin = 0x8000
verbose_mode = false
```

## 📊 Performance Features

### TRUE SMC (Self-Modifying Code)
Parameters are patched directly into instruction immediates:
- 3-5x faster function calls
- Zero memory overhead
- Automatic detection of eligible functions

### String Architecture
Length-prefixed strings for O(1) operations:
- 25-40% performance improvement
- Compile-time optimization
- Smart instruction selection

### Register Allocation
Hierarchical allocation strategy:
1. Physical registers (A, B, C, D, E, H, L)
2. Shadow registers (via EXX)
3. Memory spill only when necessary

## 🎯 Workflow Examples

### Development Cycle
```bash
# 1. Write code
echo 'fun main() -> void { @print("Hello!\n"); }' > hello.minz

# 2. Compile
mz hello.minz -o hello.a80 -O

# 3. Test in REPL
mzr
minz> /load hello.minz
minz> main()
Hello!

# 4. Check assembly
minz> /asm main

# 5. Profile performance
minz> /profile main()
[142 T-states]
```

### Interactive Development
```bash
mzr
# Experiment with code
minz> fun fib(n: u8) -> u16 {
...>     if n <= 1 { return n; }
...>     return fib(n-1) + fib(n-2);
...> }
Function 'fib' defined

minz> fib(10)
55

minz> /reg
# See register state after execution
```

## 🐛 Debugging

### REPL Debugging Commands
```
/mem 0x8000 32    # Show 32 bytes at 0x8000
/asm fib          # Show assembly for function
/profile fib(10)  # Profile execution
/vars             # Show all variables
/funcs            # Show all functions
```

### Screen Debugging
```
/cls              # Clear ZX Spectrum screen
/screen           # Show current screen
/screens          # Toggle auto-show after execution
```

## 📚 File Formats

### Input: `.minz`
MinZ source files with modern syntax

### Output: `.a80`
Z80 assembly in sjasmplus format

### Intermediate: `.mir`
MinZ Intermediate Representation (planned)

## 🚧 Current Limitations

### REPL
- Assembly generation uses placeholder (z80asm integration pending)
- No persistent session state
- Input hooks ready but not fully connected

### Compiler
- Module imports not fully implemented
- Generic functions not yet supported
- Pattern matching syntax only

## 🔮 Future Plans

### Near Term
- Full z80asm integration in REPL
- @mir blocks for CPU-independent code
- TUI mode with multiple windows
- Session recording/playback

### Long Term
- Package manager
- IDE plugins (VSCode, Vim, Emacs)
- Cross-compilation to other 8-bit CPUs
- Network collaboration features

## 💡 Tips & Tricks

### Quick Testing
```bash
# One-liner compilation and test
echo 'fun main() -> void { @print("Test\n"); }' | mz -o - | hexdump -C
```

### REPL Aliases
```bash
# Add to .bashrc/.zshrc
alias m='mzr'     # Even shorter!
alias mc='mz -O'  # Compile with optimization
```

### Performance Testing
```bash
# Compare with/without SMC
mz prog.minz -o normal.a80
mz prog.minz -o smc.a80 --enable-smc
# Compare assembly output
```

---

*The complete MinZ development environment - from idea to Z80 machine code!*