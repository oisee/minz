# MinZ Target Architecture

## Philosophy: One Backend, Multiple Targets

MinZ uses a clean separation between **backend** (CPU architecture) and **target** (system environment):

- **Backend** = CPU instruction set (Z80, 6502, 68000, etc.)
- **Target** = System environment (ZX Spectrum, CP/M, Amstrad CPC, etc.)

This enables:
- ✅ **Single Z80 backend** for all Z80-based systems
- ✅ **Conditional compilation** with `@if(TARGET == "system")`
- ✅ **Target-specific standard libraries** 
- ✅ **Clean system call integration** with `@abi()`

## Target Implementation

### 1. Compile-Time Constant

The `TARGET` constant is available for conditional compilation:

```minz
const TARGET: str = "spectrum";  // Set by compiler or user

@if(TARGET == "cpm") {
    // CP/M-specific code
} else @if(TARGET == "spectrum") {
    // ZX Spectrum-specific code  
} else {
    // Default/generic Z80 code
}
```

### 2. Standard Library Organization

Standard library modules are organized by target:

```
stdlib/
├── common/           # Common implementations
│   ├── math.minz
│   └── string.minz
├── z80/              # Z80-specific  
│   ├── spectrum/     # ZX Spectrum target
│   │   ├── screen.minz
│   │   ├── sound.minz
│   │   └── input.minz
│   ├── cpm/          # CP/M target
│   │   ├── bdos.minz
│   │   ├── files.minz
│   │   └── console.minz
│   └── amstrad/      # Amstrad CPC target
│       ├── firmware.minz
│       └── graphics.minz
└── 6502/             # 6502 backends
    └── c64/          # Commodore 64 target
```

### 3. System Call Integration

Each target defines system calls using `@abi()`:

```minz
// ZX Spectrum (RST-based)
@abi("register: A=char; call: RST 16")
fun print_char(ch: u8) -> void;

// CP/M (BDOS-based)  
@abi("register: E=char, C=2; call: 0005H")
fun print_char(ch: u8) -> void;

// Amstrad CPC (firmware-based)
@abi("register: A=char; call: 0xBB5A")
fun print_char(ch: u8) -> void;
```

### 4. Compilation Commands

Target selection happens at compile time:

```bash
# ZX Spectrum (default Z80 target)
mz program.minz -b z80 -o program.a80

# CP/M target
mz program.minz -b z80 --target=cpm -o program.z80

# Amstrad CPC target  
mz program.minz -b z80 --target=amstrad -o program.asm
```

## Supported Targets

### Z80 Backend Targets

| Target | System | Memory Layout | System Calls | File Extension |
|--------|--------|---------------|--------------|----------------|
| `spectrum` | ZX Spectrum | $8000+ | RST 16, ROM | `.a80` |
| `cpm` | CP/M 2.2/3.0 | $8000+ | BDOS 0005H | `.z80` |
| `amstrad` | Amstrad CPC | $8000+ | Firmware | `.asm` |
| `msx` | MSX/MSX2 | $8000+ | BIOS/MSX-DOS | `.z80` |

### 6502 Backend Targets

| Target | System | Memory Layout | System Calls | File Extension |
|--------|--------|---------------|--------------|----------------|
| `c64` | Commodore 64 | $0801+ | Kernal | `.asm` |
| `apple2` | Apple II | $0800+ | ProDOS/DOS 3.3 | `.s` |
| `nes` | Nintendo NES | $8000+ | None (bare metal) | `.nes.s` |

## Implementation Details

### 1. Compiler Integration

The compiler sets the `TARGET` constant based on command-line flags:

```go
// In semantic analyzer
if options.Target != "" {
    analyzer.DefineConstant("TARGET", options.Target)
} else {
    // Set default based on backend
    if backend == "z80" {
        analyzer.DefineConstant("TARGET", "spectrum")
    }
}
```

### 2. Standard Library Loading

The compiler automatically imports target-specific standard library:

```go  
// Load standard library based on backend + target
stdlibPath := fmt.Sprintf("stdlib/%s/%s/", backend, target)
if exists(stdlibPath) {
    importStandardLibrary(stdlibPath)
} else {
    importStandardLibrary(fmt.Sprintf("stdlib/%s/common/", backend))
}
```

### 3. Conditional Code Generation

All target-specific behavior happens via conditional compilation - no backend changes needed:

```minz
// In stdlib/z80/io.minz  
fun print_string(s: str) -> void {
    @if(TARGET == "cpm") {
        // CP/M implementation using BDOS
        cpm_print_string(s);
    } else @if(TARGET == "amstrad") {
        // Amstrad CPC implementation using firmware
        fw_print_string(s);
    } else {
        // Default ZX Spectrum implementation
        spectrum_print_string(s);
    }
}
```

## Benefits

### ✅ Clean Separation
- Backend handles CPU instructions
- Target handles system environment
- No code duplication between similar targets

### ✅ Easy Porting  
- Add new target = add stdlib directory + define TARGET
- No backend modifications needed
- Conditional compilation handles differences

### ✅ User Experience
- `--target=cpm` flag is intuitive
- Single Z80 backend supports all Z80 systems
- Clear error messages for unsupported combinations

### ✅ Maintainability
- Standard library organized by target
- System calls defined once per target
- Compile-time optimization removes unused code

## Example: Adding BBC Micro Target

To add BBC Micro support to the Z80 backend:

1. **Create standard library**:
   ```
   mkdir stdlib/z80/bbcmicro/
   ```

2. **Define system calls**:
   ```minz
   // stdlib/z80/bbcmicro/os.minz
   @abi("register: A=char; call: 0xFFEE")
   fun print_char(ch: u8) -> void;
   ```

3. **Use target**:
   ```bash
   mz program.minz -b z80 --target=bbcmicro
   ```

No backend code changes needed! 🎉

## Conclusion

This target architecture provides:
- **Scalability**: Easy to add new targets
- **Maintainability**: Single backend per CPU
- **User-Friendly**: Intuitive command-line interface  
- **Performance**: Compile-time optimization eliminates overhead
- **Flexibility**: Conditional compilation handles complex differences

The separation of backend and target is fundamental to MinZ's "Write Once, Deploy Everywhere" philosophy.