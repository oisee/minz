# Agon Light 2 / eZ80 Support Implementation Plan

## Overview

Add comprehensive support for the **Agon Light 2** (and Agon Light) microcomputer powered by the **Zilog eZ80** processor. This includes:
- New target configuration for Agon platform
- eZ80-specific instructions and ADL (Address Data Long) mode
- 24-bit type support (u24/i24)
- MOS (Machine Operating System) API bindings
- Fixed-point math types (f8.8, f16.8, f8.16)

## Architecture Decision Records

- **[ADR-001](docs/ADR-001-eZ80-ABI-and-Mode-Support.md)** - eZ80 ABI and CPU Mode Support
- **[ADR-002](docs/ADR-002-eZ80-Assembler-Instructions.md)** - eZ80 Assembler Instructions

## Reference Implementation

- **[agon-ez80asm](https://github.com/AgonPlatform/agon-ez80asm)** - MIT License eZ80 assembler
- Local reference: `minzc/pkg/z80asm/ez80_instructions.md`

---

## Phase 1: Target Configuration - DONE

### Task 1.1: Add TargetAgonLight2 to targets.go - DONE
**File:** `minzc/pkg/z80asm/targets.go`

Added:
- `TargetAgonLight2 Target = "agon"`
- Memory layout (512KB RAM)
- Output format (`.bin`)
- 50+ MOS API symbols

### Task 1.2: Add MOS Common Symbols - DONE
**File:** `minzc/pkg/z80asm/targets.go`

Added MOS API entry points and function numbers.

### Task 1.3: Create generateAgonBin Output Generator - DONE
**File:** `minzc/pkg/z80asm/targets.go`

Created MOS-compatible binary output generator.

---

## Phase 2: eZ80 Instruction Support - DONE

### Task 2.1: Add eZ80 CPU Mode Flag - DONE
**File:** `minzc/pkg/z80asm/assembler.go`

Added:
- `CPUMode` type with `CPUModeZ80`, `CPUModeEZ80Z80`, `CPUModeEZ80ADL`
- `ADLSuffixSIS/LIS/SIL/LIL` constants
- `SetCPUMode()`, `IsEZ80()`, `IsADLMode()`, `GetAddressSize()` methods

### Task 2.2: Add ADL Suffix Handling - DONE
**File:** `minzc/pkg/z80asm/assembler.go`

Added:
- `ParseADLSuffix()` - extracts `.SIS/.LIS/.SIL/.LIL` or `.S/.L` suffixes
- `GetCrossModeSuffix()` - returns correct suffix for cross-mode calls
- Smart `.S`/`.L` resolution based on current ADL mode

### Task 2.3: Add LEA/PEA/MLT Instructions - DONE
**File:** `minzc/pkg/z80asm/ez80_instructions.go`

Added:
- LEA BC/DE/HL/IX/IY, IX+d and IY+d variants
- PEA IX+d, IY+d
- MLT BC/DE/HL/SP

### Task 2.4: Add eZ80 Block I/O Instructions - DONE
**File:** `minzc/pkg/z80asm/ez80_instructions.go`

Added:
- IND2, IND2R, INDM, INDMR, INDRX, INI2, INI2R, INIM, INIMR, INIRX
- OTD2R, OTDM, OTDMR, OTDRX, OTI2R, OTIM, OTIMR, OTIRX, OUTD2, OUTI2
- IN0 r,(n), OUT0 (n),r
- TST A,r and TST A,n
- SLP, STMIX, RSMIX

---

## Phase 3: ABI and Codegen - IN PROGRESS

### Task 3.1: Add @abi("ez80_adl") Support - DONE
**File:** `minzc/pkg/semantic/analyzer.go`

Added ABI values:
- `ez80_adl` - 24-bit ADL mode stack ABI
- `ez80_z80` - 16-bit Z80 mode on eZ80
- `mos` - MOS API calling convention

### Task 3.2: Add @mode Annotation - DONE
**File:** `minzc/pkg/ast/ast.go`, `minzc/pkg/semantic/analyzer.go`, `minzc/pkg/ir/ir.go`

Added:
- `CPUModeType` enum in ast.go (Default, Z80, ADL)
- `CPUMode` field in ir.Function
- `processModeAttributes()` function in analyzer
- Propagation from AST to IR

```minz
@mode("adl")   // Function runs in ADL mode
@mode("z80")   // Function runs in Z80 mode
```

### Task 3.3: Cross-Mode Call Generation - DONE
**File:** `minzc/pkg/codegen/z80.go`

Added:
- `isEZ80Target` and `defaultADLMode` fields in Z80Generator
- `getCPUMode()`, `isADLMode()`, `getCallSuffix()` helper methods
- `emitCall()`, `emitCallAddress()`, `emitRST()` - emit instructions with ADL suffixes
- `SetEZ80Mode()` - configure eZ80 target mode
- Auto-detect eZ80 mode from "agon" platform
- Updated extern function calls to use cross-mode suffixes

### Task 3.4: 24-bit Type Codegen
**File:** `minzc/pkg/codegen/z80.go`
**Priority:** MEDIUM

- u24/i24 arithmetic
- 24-bit loads/stores
- LEA for efficient address calculation

---

## Phase 4: MOS Stdlib - DONE

### Task 4.1: Create stdlib/agon/mos.minz - DONE
~200 lines with MOS API bindings.

### Task 4.2: Create stdlib/agon/vdp.minz - DONE
~300 lines with VDP graphics API.

### Task 4.3: Create stdlib/agon/audio.minz - TODO
Audio subsystem bindings.

---

## Phase 5: Fixed-Point Math - TODO

### Task 5.1: Implement f8.8 Operations
**File:** `minzc/pkg/codegen/z80/fixedpoint.go` (new)
**Priority:** HIGH

### Task 5.2: Implement f16.8 and f8.16 Operations
**Priority:** MEDIUM

### Task 5.3: Add Fixed-Point Type Inference
**Priority:** MEDIUM

---

## Phase 6: Testing & Examples - PARTIAL

### Task 6.1: Create examples/agon/hello_world.minz - DONE
### Task 6.2: Create examples/agon/graphics_demo.minz - DONE
### Task 6.3: Create examples/agon/file_io.minz - TODO
### Task 6.4: Add Agon Target Tests - DONE

5 tests in `minzc/pkg/z80asm/agon_test.go`, all passing.

---

## Progress Summary

| Phase | Status | Completion |
|-------|--------|------------|
| 1. Target Config | DONE | 100% |
| 2. eZ80 Instructions | DONE | 100% |
| 3. ABI & Codegen | IN PROGRESS | 75% |
| 4. MOS Stdlib | PARTIAL | 80% |
| 5. Fixed-Point | TODO | 0% |
| 6. Testing | PARTIAL | 70% |

**Overall: ~70% complete**

## MinZ 2026 Vision - Phase 1 Syntax Cleanup - DONE

Implemented the new unified syntax from `docs/MinZ_2026_Vision.md`:

### New extern function syntax:
```minz
extern fun mos_putchar(c: u8) at 0x10;           // Simple extern
extern fun mos_call(fn: u8) at 0x08 mode "adl";  // With CPU mode
extern fun custom(x: u16) at 0xC000 abi "register"; // With ABI
```

### Files modified:
- `pkg/parser/participle/decl.go` - Added AtAddress, Mode, Abi fields
- `pkg/parser/participle/convert.go` - Converts to AST fields
- `pkg/parser/participle/decl_test.go` - Added 8 test cases

### Test results:
- All parser tests pass
- All 3 platform tests pass (ZX Spectrum, CP/M, Agon)
- All 11 feature tests pass

## Next Steps

1. ~~Add `@abi("ez80_adl")` and `@mode("adl/z80")` support~~ DONE
2. ~~Implement Task 3.3 (cross-mode call generation in codegen)~~ DONE
3. ~~MinZ 2026 Vision Phase 1 (syntax cleanup)~~ DONE
4. ~~MinZ 2026 Vision Phase 2 (core metafunctions)~~ DONE
   - @sizeof(T), @target(), @static_assert, @comptime_error
5. Implement register mapping for extern: `extern fun f(x in HL) at 0x10;`
6. Implement Task 3.4 (24-bit type codegen) - u24/i24 arithmetic, LEA usage
7. Test with real Agon hardware or emulator
8. Add audio stdlib (stdlib/agon/audio.minz)
