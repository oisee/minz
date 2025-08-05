# MinZ Backend Configuration

## Backend Selection Precedence

1. **Command-line flag** (`-b` or `--backend`) - Highest priority
2. **Environment variable** (`MINZ_BACKEND`) - Default if no CLI flag
3. **Built-in default** (`z80`) - Fallback if neither is set

## Usage Examples

```bash
# Use default backend (z80)
minzc program.minz

# Set backend via environment variable
export MINZ_BACKEND=6502
minzc program.minz              # Uses 6502
minzc program.minz -b z80       # Override with z80

# Set backend via command line
minzc program.minz -b wasm

# List available backends
minzc --list-backends
```

## Backend Features

| Backend | SMC/TSMC | Calling Convention | Pointer Size | Special Features |
|---------|----------|-------------------|--------------|------------------|
| Z80     | ✅ Full   | Register + SMC    | 16-bit       | Shadow registers, ROM routines |
| 6502    | ✅ Zero-page | Stack/Zero-page | 16-bit    | Zero-page optimization |
| WASM    | ❌ None   | Stack-based       | 32-bit       | Standard conventions, imports |

## Backend-Specific Optimizations

### Z80
- Self-modifying code (SMC) enabled by default with `-O`
- TRUE SMC for immediate parameter patching
- Shadow register optimization for interrupts
- Supports all MinZ features

### 6502
- Zero-page SMC optimization (planned)
- Fast zero-page addressing
- Platform-specific I/O (C64, Apple II, NES)
- SMC enabled with `-O` flag

### WebAssembly
- **No SMC support** - Uses standard stack-based calling
- Automatic imports for runtime functions
- Memory managed by host environment
- Float/fixed-point emulated in software

## @target Directive

Conditional compilation based on backend:

```minz
fun platform_init() -> void {
    @target("z80") {
        // Z80-specific initialization
        asm {
            LD SP, $FF00    ; Set stack pointer
            IM 1            ; Interrupt mode 1
        }
    }
    
    @target("6502") {
        // 6502-specific initialization  
        asm {
            LDX #$FF        ; Initialize stack
            TXS
            CLI             ; Enable interrupts
        }
    }
    
    @target("wasm") {
        // WASM doesn't need low-level init
        // Runtime handles this
    }
}
```

## Configuration Files

Future support for `.minzrc` or `minz.toml`:

```toml
[defaults]
backend = "6502"
optimize = true

[backends.6502]
zero_page_start = 0x20
zero_page_size = 64

[backends.z80]
enable_true_smc = true
shadow_registers = true
```

## Debug Output

Use `-d` flag to see backend selection:

```
$ minzc program.minz -d
Using backend: z80
  (default)

$ MINZ_BACKEND=6502 minzc program.minz -d
Using backend: 6502
  (from environment variable MINZ_BACKEND)

$ minzc program.minz -b wasm -d
Using backend: wasm
```

## Platform Profiles

Each backend can have platform-specific profiles:

### Z80 Platforms
- `zx-spectrum` - ZX Spectrum (default)
- `msx` - MSX computers
- `cpc` - Amstrad CPC
- `ti-83` - TI-83/84 calculators

### 6502 Platforms
- `c64` - Commodore 64 (default)
- `apple2` - Apple II series
- `nes` - Nintendo Entertainment System
- `atari-2600` - Atari 2600

### WASM Platforms
- `browser` - Web browser (default)
- `node` - Node.js runtime
- `wasmtime` - Wasmtime runtime

## Backend Development

To add a new backend:

1. Implement the `Backend` interface
2. Register in `init()` function
3. Add feature flags
4. Update documentation

See `docs/BACKEND_DEVELOPMENT_GUIDE.md` for details.