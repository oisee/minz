# ADR-001: eZ80 ABI and CPU Mode Support

**Status:** Accepted
**Date:** 2026-02-10
**Author:** MinZ Team

## Context

MinZ is adding support for the Agon Light 2 computer which uses the Zilog eZ80 processor. The eZ80 has two operating modes:

1. **ADL Mode (ADL=1)** - Native 24-bit mode with 16MB address space
2. **Z80 Mode (ADL=0)** - Legacy 16-bit mode, compatible with classic Z80

The Agon's MOS (Machine Operating System) runs in ADL mode. User programs can run in either mode, but cross-mode calls require special instruction suffixes.

## Decision

### 1. CPU Mode Tracking

The compiler will track the current CPU mode and emit appropriate instruction suffixes:

```go
type CPUMode int

const (
    CPUModeZ80      CPUMode = iota  // Classic Z80 (not eZ80)
    CPUModeEZ80Z80                   // eZ80 in Z80 mode (ADL=0)
    CPUModeEZ80ADL                   // eZ80 in ADL mode (ADL=1)
)
```

### 2. Cross-Mode Calling Convention

| Caller Mode | Callee Mode | CALL/RST Suffix | Return |
|-------------|-------------|-----------------|--------|
| ADL | ADL | (none) | Normal |
| ADL | Z80 | `.SIS` | Returns to ADL |
| Z80 | ADL | `.LIL` | Returns to Z80 |
| Z80 | Z80 | (none) | Normal |

### 3. ABI Annotations

New `@abi` values for eZ80:

```minz
// ADL mode stack-based ABI (24-bit)
@abi("ez80_adl")
fun adl_function(addr: u24, count: u16) -> u24 {
    // Uses SP+offset, 24-bit return in UHL
}

// Z80 mode on eZ80 (16-bit compatible)
@abi("ez80_z80")
fun z80_function(addr: u16) -> u16 {
    // Uses IX+offset, 16-bit return in HL
}

// MOS API (always ADL mode target)
@abi("mos")
@extern(0x10)
fun mos_putchar(c: u8);
```

### 4. Mode Declaration

Functions can declare their execution mode:

```minz
// Per-function mode
@mode("adl")
fun my_adl_func() { ... }

@mode("z80")
fun legacy_func() { ... }
```

Module-level default (in file header or via compiler flag):
```minz
@cpu("ez80_adl")  // Default mode for this file
```

### 5. Instruction Suffix Emission

```go
func (g *Z80Gen) emitRST(addr uint8, targetMode CPUMode) {
    switch {
    case g.cpuMode == CPUModeEZ80Z80 && targetMode == CPUModeEZ80ADL:
        g.emit("    RST.LIL 0x%02X", addr)
    case g.cpuMode == CPUModeEZ80ADL && targetMode == CPUModeEZ80Z80:
        g.emit("    RST.SIS 0x%02X", addr)
    default:
        g.emit("    RST 0x%02X", addr)
    }
}

func (g *Z80Gen) emitCALL(target string, targetMode CPUMode) {
    switch {
    case g.cpuMode == CPUModeEZ80Z80 && targetMode == CPUModeEZ80ADL:
        g.emit("    CALL.LIL %s", target)
    case g.cpuMode == CPUModeEZ80ADL && targetMode == CPUModeEZ80Z80:
        g.emit("    CALL.SIS %s", target)
    default:
        g.emit("    CALL %s", target)
    }
}
```

### 6. Stack Frame Differences

**ADL Mode (24-bit):**
- Return address: 3 bytes
- Pointers: 3 bytes
- Use `LEA IX, SP-n` for frame setup (efficient!)
- Parameters at `SP+offset`

**Z80 Mode (16-bit):**
- Return address: 2 bytes
- Pointers: 2 bytes
- Use `LD IX, SP` / `ADD IX, n` for frame setup
- Parameters at `IX+offset`

### 7. Default Behavior

For `--target=agon`:
- Default mode: ADL (24-bit)
- MOS calls: Automatically use correct suffix
- `u24`/`i24` types: Enabled
- Address size: 24-bit

## Consequences

### Positive
- Full eZ80 support with both modes
- Automatic cross-mode call handling
- Clean integration with existing `@abi` system
- Efficient code generation using LEA in ADL mode

### Negative
- Increased compiler complexity
- Need to track mode through call graph
- Testing requires eZ80 emulator or hardware

### Neutral
- Existing Z80 code unchanged
- New types (u24/i24) only available in eZ80 mode

## References

- [eZ80 CPU User Manual](https://www.zilog.com/docs/um0077.pdf)
- [agon-ez80asm](https://github.com/AgonPlatform/agon-ez80asm) - MIT License
- [Agon MOS Documentation](https://github.com/breakintoprogram/agon-mos)
- MinZ `@abi` system: `pkg/semantic/analyzer.go:8834`
