# ADR-0003: Platform-Independent Compilation for Z80 Systems

## Status
Accepted

## Context
MinZ initially generated Z80 assembly code specifically for the ZX Spectrum platform, hardcoding system calls like `RST 16` for character output. However, Z80 processors were used in many different systems, each with their own conventions:

- **ZX Spectrum**: Uses `RST 16` for character output
- **CP/M**: Uses `CALL 5` with BDOS functions
- **MSX**: Uses `CALL $00A2` for BIOS CHPUT
- **Amstrad CPC**: Uses `CALL $BB5A` for TXT OUTPUT

Users needed to manually modify generated assembly for different platforms, which was error-prone and reduced code portability.

## Decision
Implement compile-time platform targeting through a `-t/--target` flag that adapts code generation for specific Z80 platforms:

1. **Add target platform parameter** to the compiler
2. **Generate platform-specific system calls** based on target
3. **Provide platform constants** accessible in MinZ code via `TARGET` variable
4. **Support conditional compilation** with `@if(TARGET == "platform")` blocks

## Implementation

### Compiler Flag
```bash
mz program.minz -t zxspectrum  # Target ZX Spectrum (default)
mz program.minz -t cpm         # Target CP/M systems
mz program.minz -t msx         # Target MSX computers
mz program.minz -t cpc         # Target Amstrad CPC
```

### Code Generation
The `@print` metafunction generates different assembly based on target:

```go
// In z80.go code generator
switch g.targetPlatform {
case "cpm":
    g.emit("    LD E, A        ; Character to E")
    g.emit("    LD C, 2        ; BDOS function 2")
    g.emit("    CALL 5         ; Call BDOS")
case "msx":
    g.emit("    CALL $00A2     ; MSX BIOS CHPUT")
default: // zxspectrum
    g.emit("    RST 16         ; Print character in A")
}
```

### MinZ Code
```minz
fun main() -> void {
    @print("Hello from MinZ!");
    
    @if(TARGET == "cpm") {
        @print("Running on CP/M!");
    }
}
```

## Consequences

### Positive
- **True portability**: Same MinZ source runs on multiple platforms
- **Zero overhead**: Platform detection at compile time, not runtime
- **Clean abstractions**: Platform details hidden from user code
- **Growing ecosystem**: Easy to add new platform targets
- **Professional toolchain**: Comparable to cross-compilers like GCC

### Negative
- **Increased complexity**: Code generator needs platform-specific knowledge
- **Testing burden**: Need to test on multiple platforms/emulators
- **Documentation**: Must document platform-specific features and limitations

### Neutral
- Binary size unchanged (only target platform code is generated)
- Compilation time negligibly affected
- Platform-specific features may not be portable

## Future Extensions
- Additional platforms: ZX81, TRS-80, Sega Master System, Game Boy
- Platform-specific standard libraries
- Hardware abstraction layer for common operations
- Platform capability detection

## Alternatives Considered

### Runtime platform detection
- **Pros**: Single binary for all platforms
- **Cons**: Runtime overhead, larger binaries, complex detection code
- **Rejected because**: Goes against MinZ philosophy of zero-cost abstractions

### Separate compilers per platform
- **Pros**: Simpler individual compilers
- **Cons**: Code duplication, maintenance nightmare, poor user experience
- **Rejected because**: Unmaintainable with multiple platforms

### Assembly macros for portability
- **Pros**: Keeps compiler simple
- **Cons**: Pushes complexity to users, error-prone, poor abstraction
- **Rejected because**: Defeats purpose of high-level language

## References
- [Z80 System Calls Across Platforms](http://www.z80.info/z80code.htm)
- [CP/M BDOS Functions](http://www.gaby.de/cpm/manuals/archive/cpm22htm/ch5.htm)
- [MSX BIOS Calls](https://www.msx.org/wiki/System_variables_and_work_area)
- [Platform Independence Achievement](../docs/150_Platform_Independence_Achievement.md)

## Related ADRs
- ADR-0002: CLI Standardization (defines `-t/--target` flag standard)

## Date
2025-08-09