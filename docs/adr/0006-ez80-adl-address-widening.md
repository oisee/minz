# ADR-0006: eZ80 ADL Address Widening Strategy

## Status
Accepted

## Date
2026-02-14

## Context

The MinZ Z80 assembler (`z80asm` package) used `uint16` for all address-related types: symbol values, instruction targets, memory layout parameters, assembled instruction addresses, and result metadata. This was sufficient for classic Z80 (16-bit, max `$FFFF`) but incompatible with the Agon Light 2's eZ80 processor running in ADL mode, which uses 24-bit addresses (up to `$FFFFFF`).

Agon MOS loads executables at `$040000` — already exceeding `uint16` range. Without widening, all Agon addresses silently truncated to 16 bits, producing corrupt binaries.

## Decision

Widen all address-related types from `uint16` to Go's native `int` (64-bit on modern platforms). This is a "big bang" refactoring touching 12 files across the z80asm package.

Key design choices:

1. **Use `int` not `uint32` or `uint24`**: Go's `int` is the idiomatic type for array indexing and arithmetic. It avoids casting friction everywhere and is future-proof for any conceivable address width.

2. **`emitWord()` helper**: A new assembler method that emits 2 bytes in Z80 mode or 3 bytes in ADL mode, selected by `CPUMode`. All instruction encoders call this instead of manual byte splitting.

3. **No separate ADL type**: Rather than creating an `Address` type with Z80/ADL variants, we keep plain `int` and let `emitWord()` handle the encoding difference. This keeps the assembler code simple.

4. **Target-driven mode selection**: `CPUMode` is set from the target config (`CPUModeEZ80ADL` for Agon), not per-instruction. This matches MOS's behavior where the entire executable runs in ADL mode.

## Consequences

### Positive
- Agon Light 2 executables assemble correctly at `$040000` with 3-byte addresses
- No `uint16` overflow bugs possible for any target
- Clean foundation for potential future eZ80 Z80-mode segments (mixed ADL/Z80)
- All existing Z80 targets continue to work unchanged (2-byte emission)

### Negative
- Large refactoring surface (12 files, ~60 type changes) — risk of missed spots
- `int` is wider than necessary (64-bit vs 24-bit) — negligible memory impact
- Format strings changed from `$%04X` to `$%06X` — slightly different debug output for Z80 targets

### Neutral
- Runtime performance unchanged — `int` operations are native width on all Go platforms
- Binary output unchanged for Z80 targets (still 2 bytes per address)
- The `emitWord()` abstraction could later support other variable-width encodings

## Alternatives Considered

1. **`uint32`**: More natural width but requires casting for array indexing and length calculations. Go idiom strongly favors `int`.

2. **Custom `Address` type**: `type Address uint32` with methods. Too much ceremony for what is essentially "an integer that holds an address". Would require converting at every boundary.

3. **Separate ADL instruction encoders**: Fork instruction_table.go for ADL mode. Massive code duplication for a difference that only affects immediate/address width.

4. **Keep `uint16` with ADL offset**: Store ADL addresses as `addr - 0x040000` internally. Fragile, error-prone, and breaks any assumption about address ordering.

## References
- [ADR-0003](0003-platform-independent-compilation.md) — Platform-Independent Compilation
- Agon Light 2 MOS documentation: executables load at `0x040000` in ADL mode
- eZ80 CPU User Manual: ADL mode uses 24-bit MBASE+address scheme
