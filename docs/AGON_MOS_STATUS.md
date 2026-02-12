# Agon MOS Support Status Report

**Date:** 2026-02-12
**Version:** v0.18.x

---

## Summary

Implemented initial Agon MOS binary header generation and eZ80 ADL suffix support in the MinZ assembler (MZA).

---

## Implemented Features

### 1. MOS Binary Header Generation (`pkg/z80asm/targets.go`)

The `generateAgonBin()` function now produces MOS-compatible executables with a 69-byte header:

```
Offset  Contents                Description
------  --------                -----------
0x00    C3 45 00                JP 0x0045 (jump over header)
0x03    NN                      Program name length (incl null)
0x04    "program.bin\0"         Null-terminated program name
0x10    FF FF FF...             Padding to offset 0x40
0x40    "MOS"                   Magic marker
0x43    00                      Header version
0x44    01                      ADL mode flag (1=24-bit)
0x45    <code>                  Machine code starts here
```

### 2. eZ80 ADL Suffix Support (`pkg/z80asm/pattern_matcher.go`)

All instructions now support ADL mode suffixes:

| Suffix | Byte | Meaning |
|--------|------|---------|
| `.SIS` | 0x40 | Short instruction, Short operands |
| `.LIS` | 0x49 | Long instruction, Short operands |
| `.SIL` | 0x52 | Short instruction, Long operands |
| `.LIL` | 0x5B | Long instruction, Long operands |

Example:
```asm
RST.LIL $10    ; Outputs: 5B D7 (prefix + RST $10)
LD.LIL HL, nn  ; Outputs: 5B 21 nn nn nn (prefix + LD HL,nn with 24-bit)
```

### 3. Test Coverage

- All Agon-specific unit tests pass
- ADL suffix parsing tests pass
- Binary header format verified via hexdump

---

## Current Issue

**MOS reports "Invalid executable"** when running the generated binary.

### Likely Cause

The JP instruction in the header uses Z80-style 16-bit addressing (`C3 45 00`), but MOS runs in ADL mode where JP uses 24-bit addressing (`C3 45 00 00`).

### Proposed Fix

Update header to use 24-bit JP:
```go
// Current (Z80 mode - 3 bytes):
header = append(header, 0xC3, 0x45, 0x00)

// Fixed (ADL mode - 4 bytes):
header = append(header, 0xC3, 0x46, 0x00, 0x00)  // JP 0x000046
```

This shifts all subsequent offsets by 1 byte.

---

## How to Build and Test

### Build Assembler
```bash
cd minzc
make           # Builds all tools including mza
```

### Assemble for Agon
```bash
./mza -o hello.bin --target=agon hello.a80
```

### Test on Emulator
```bash
cd ~/dev/fab-agon-emulator/release-linux-amd64
mkdir -p sdcard
cp hello.bin sdcard/
./agon-cli-emulator --sdcard ./sdcard
# At MOS prompt: run hello.bin
```

### Example Agon Program
```asm
; hello_agon.a80
    ORG 0

start:
    LD A, 'H'
    RST.LIL $10     ; Send char to VDP
    LD A, 'i'
    RST.LIL $10
    LD A, '!'
    RST.LIL $10
    RET             ; Return to MOS
```

---

## Still TODO

### Critical (for valid executables)
- [ ] Fix JP instruction to use 24-bit addressing in header
- [ ] Verify header offsets after JP fix
- [ ] Test on real Agon emulator

### Important (from Agon spec)
- [ ] Startup prologue (save/restore MB, registers)
- [ ] 3-byte return addresses on stack (affects local variable offsets)
- [ ] IY reserved checking in codegen
- [ ] Code should use ORG 0x040000 or relative addressing

### Nice to Have
- [ ] VDP command helper functions
- [ ] MOS API wrapper functions
- [ ] Custom program name in header (currently "program.bin")

---

## Reference Documents

- `docs/agon-mos-binary-format.md` - Complete MOS binary specification
- Agon MOS source: https://github.com/breakintoprogram/agon-mos
- fab-agon-emulator: `~/dev/fab-agon-emulator/`

---

## Commits

1. `764f3d5` - feat: Add proper Agon MOS binary header and ADL suffix support
2. `bba61e3` - fix: Add mzv to Makefile build targets
3. `7faabcd` - fix: Remove unused bufio import from mzr
4. `77b1e9f` - docs: Update documentation for loop reroll optimization
5. `ed7dff1` - feat: Multi-parameter pattern detection in loop reroll (1-7 params)
