# ADR-0007: Platform-Specific Newline Handling

## Status
Accepted

## Date
2026-02-14

## Context

The MinZ codegen had a hard-coded `LF (10) -> CR (13)` conversion applied to all string data and direct character output. This was originally correct when ZX Spectrum was the only target — the Spectrum uses CR (13) for newlines, and MinZ source code uses LF (10) in string literals like `"\n"`.

With the addition of CP/M and Agon Light 2 targets, this conversion became incorrect:
- **CP/M** expects CR+LF (`\r\n`, bytes 13+10) for newlines
- **Agon Light 2** (VT100 terminal) expects LF (10) or CR+LF
- **ZX Spectrum** expects CR (13) only

The unconditional conversion was silently corrupting CP/M output — any `\n` in rerolled strings became `\r` (CR), producing double-CR instead of CR+LF.

## Decision

Gate the LF-to-CR conversion on `targetPlatform == "zxspectrum"`. All other platforms preserve LF as-is.

This applies in two code paths:
1. **String data emission** — when generating `DB` directives for string constants
2. **OpPrintStringDirect** — when generating inline character-by-character output

The conversion is purely a codegen concern — the IR and optimizer work with raw byte values from source code. Platform-specific character mapping happens only at the final assembly emission stage.

## Consequences

### Positive
- CP/M programs correctly output CR+LF when source has `\r\n`
- Agon programs preserve LF in strings (correct for VT100 terminals)
- Each platform gets the newline convention its OS/firmware expects
- Clean separation: IR stores source-faithful values, codegen applies platform rules

### Negative
- ZX Spectrum string literals with `\n` still auto-convert to `\r` — if a user explicitly wants byte 10 on Spectrum, they must use `putchar(10)` directly
- No centralized "character mapping table" per platform — currently just an if-check

### Neutral
- Existing ZX Spectrum programs produce identical output (no regression)
- The check is compile-time only — zero runtime cost

## Alternatives Considered

1. **Platform newline mapping table**: A `map[byte]byte` per platform for character transformations. Over-engineered for a single conversion rule. Could revisit if more platform-specific character mappings emerge.

2. **Never convert — let source handle it**: Require users to write `\r` explicitly for Spectrum. Breaking change for all existing ZX Spectrum programs that use `\n`.

3. **Convert in the optimizer/IR layer**: Would affect all backends. The conversion is a Z80 backend concern specific to certain platforms — doesn't belong in the IR.

## References
- ZX Spectrum ROM: RST 16 treats byte 13 as newline (carriage return + line feed)
- CP/M BDOS function 2: outputs byte as-is, expects 13+10 for newline
- Agon MOS VDP: VT100-compatible terminal, LF (10) moves cursor down
