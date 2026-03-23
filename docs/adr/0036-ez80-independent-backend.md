# ADR-0036: eZ80 as an Independent Backend

**Date:** 2026-03-20
**Status:** Proposed
**Related:** ADR-0033 (LIR pipeline), ADR-0034 (unified constraint framework)

## Context

The Agon Light 2 runs a Zilog eZ80 at 18.432 MHz in ADL (Address Data Long) mode.
MinZ currently targets eZ80 by patching the Z80 codegen: `isEZ80Target` adds `.LIL`
suffixes to CALL/RST, sets ORG to `$040000`, and that's about it. All registers are
still treated as 16-bit, pointers are 16-bit, and the assembler emits 2-byte
immediates.

This is wrong. eZ80 in ADL mode is a **fundamentally different architecture**:

| Property | Z80 | eZ80 ADL |
|----------|-----|----------|
| Register width | 16-bit | 24-bit |
| Address space | 64 KB | 16 MB |
| `LD HL, nn` | 3 bytes (op + 2) | 4 bytes (op + 3) |
| `PUSH/POP` | 2 bytes on stack | 3 bytes on stack |
| `CALL/RET` | pushes 2-byte PC | pushes 3-byte PC |
| `JP nn` | 3 bytes | 4 bytes |
| Stack frame slot | 2 bytes | 3 bytes |
| Hardware multiply | No | `MLT rr` (B×C→BC, 6T) |
| LEA | No | `LEA rr, IX+d` (one opcode) |
| PEA | No | `PEA IX+d` (push effective address) |
| TST | No | `TST A, r` (AND without storing) |
| Block I/O | IN/OUT family | Extended INI2, OTIRX, etc. |

Bolting 24-bit support onto the Z80 codegen would require conditional logic in
every `LD`, every `PUSH`, every `CALL`, every stack offset calculation, every
virtual register address. This creates a combinatorial explosion of `if isEZ80`
branches that makes both backends harder to maintain and debug.

### Assembler Question

MZA (our assembler) already encodes eZ80 instructions (LEA, PEA, MLT, ADL suffixes)
but does **not** handle 24-bit immediates/addresses in the standard Z80 instruction
set. Extending MZA to support `LD HL, $040000` (3-byte immediate) requires changes
to the expression evaluator, operand parser, and encoding pipeline.

Three options:

1. **Extend MZA** — add `CPUModeEZ80ADL` that switches to 3-byte immediates/addresses.
   The infrastructure exists (`SetCPUMode`, `IsADLMode`), but the actual encoding
   paths don't emit 3-byte values yet. ~300 LOC.

2. **Use fasmg** — the assembler used by AgDev/CEdev. Mature eZ80 support, used by
   the TI-84 CE and Agon communities. MIT-licensed. External dependency.

3. **Use agon-ez80asm** — community assembler for Agon. Runs natively on Agon.
   Good for development on-device but not suitable as a cross-assembler dependency.

## Decision

### 1. Separate codegen package: `pkg/ez80`

The eZ80 ADL backend lives in `minzc/pkg/ez80/`, independent of `pkg/codegen/z80.go`.
It consumes MIR2 directly (same IR as Z80) but generates eZ80 ADL assembly.

```
MIR2 → pkg/codegen/z80.go    → Z80 assembly  → MZA → .bin/.sna/.tap
MIR2 → pkg/ez80/codegen.go   → eZ80 assembly → MZA (ADL mode) → .bin
```

The two codegens share **nothing** at the Go code level. They may share MIR2
optimization passes (constant folding, DSE, etc.) since those are target-independent.

Rationale:
- Z80 codegen is 5000+ LOC of battle-tested code. We must not destabilize it.
- eZ80 ADL makes different decisions everywhere: pointer size, stack layout,
  register width, instruction selection, calling convention.
- Undocumented instructions differ between Z80 and eZ80 — separate code avoids
  accidentally emitting invalid opcodes for either target.
- The LIR backend (ADR-0033) can be adapted for eZ80 with a new `TargetDescriptor`
  — different register set, different costs, different patterns.

### 2. Extend MZA for 24-bit mode

MZA already has `CPUModeEZ80ADL`. We extend the encoding pipeline:

- `addressSize()` returns 3 in ADL mode (currently stubbed)
- `immediateSize()` returns 3 for `LD rr, nn` in ADL mode
- Expression evaluator accepts 24-bit values (0..16777215)
- `.ASSUME ADL=1` directive sets mode (compatible with community assemblers)

This keeps our toolchain self-contained. No external assembler dependency.

### 3. Target info structure

```go
// TargetInfo describes the target architecture for codegen decisions.
type TargetInfo struct {
    Name         string // "z80", "ez80_adl", "ez80_z80"
    PtrSize      int    // 2 (Z80) or 3 (eZ80 ADL)
    RegWidth     int    // 16 (Z80) or 24 (eZ80 ADL)
    StackSlot    int    // bytes per PUSH/POP: 2 or 3
    HasMLT       bool   // hardware 8×8 multiply (MLT rr)
    HasLEA       bool   // LEA rr, IX+d
    HasPEA       bool   // PEA IX+d
    HasTST       bool   // TST A, r (non-destructive AND)
    HasBlockIO2  bool   // extended block I/O (INI2, OTIRX, etc.)
    MaxImm       uint32 // max immediate value: 0xFFFF or 0xFFFFFF
    OrgAddress   uint32 // default ORG: $8000, $0100, $040000
}

var TargetZ80 = TargetInfo{
    Name: "z80", PtrSize: 2, RegWidth: 16, StackSlot: 2,
    MaxImm: 0xFFFF, OrgAddress: 0x8000,
}

var TargetEZ80ADL = TargetInfo{
    Name: "ez80_adl", PtrSize: 3, RegWidth: 24, StackSlot: 3,
    HasMLT: true, HasLEA: true, HasPEA: true, HasTST: true,
    HasBlockIO2: true, MaxImm: 0xFFFFFF, OrgAddress: 0x040000,
}
```

### 4. Pointer type in MIR2

`TyPtr` width becomes target-dependent:

- Z80: `TyPtr` = 16-bit (uses HL, DE, BC — 2-byte loads/stores)
- eZ80 ADL: `TyPtr` = 24-bit (uses HL, DE, BC — 3-byte loads/stores)

MIR2 `TyU24`/`TyI24` already exist but currently map to `ClassDWord` (shadow
register pair on Z80). On eZ80 ADL, `u24` maps to a single register pair — native
width, no splitting needed.

The MIR2 layer remains target-neutral. Codegen interprets `TyPtr` width based on
`TargetInfo.PtrSize`.

### 5. ADL mode calling convention

```
eZ80 ADL Calling Convention (Agon)
──────────────────────────────────
Arguments:  A (u8), HL (u24/ptr), DE (u24), BC (u24)
            Stack for overflow (3 bytes per slot)
Returns:    A (u8), HL (u24/ptr)
Callee-save: IX, IY
Clobbers:   AF, BC, DE, HL (caller-save)
Stack:      3 bytes per PUSH/POP, SP is 24-bit
MOS calls:  RST.LIL $08/$10/$18 with A=function code
```

This is similar to the Z80 convention but with 24-bit register widths and 3-byte
stack slots. PFCCO (Per-Function Calling Convention Optimization) applies — the
optimizer can choose register assignments per call site.

### 6. eZ80-specific optimizations

#### MLT — hardware 8×8 multiply

Z80: `__mul8` software routine (~80T, ~30 bytes)
eZ80: `MLT BC` — 6 T-states, 2 bytes

```lisp
;; ISLE rule for eZ80
(rule (lower_ez80 (mul u8 ?x ?y))
      (mlt_bc (mov_b ?x) (mov_c ?y)))  ; B=x, C=y, MLT BC → BC = B*C
```

At 18.432 MHz, MLT takes 0.33μs. The Z80 software multiply at 3.5 MHz takes ~23μs.
That's a **70× speedup** for u8 multiplication.

#### LEA — load effective address

Z80: struct field access = `LD L, (IX+d)` + `LD H, (IX+d+1)` (2 instructions)
eZ80: `LEA HL, IX+d` (1 instruction, loads 24-bit address)

```lisp
(rule (lower_ez80 (addr_of_field ?base ?offset))
      (lea_hl_ix ?offset))
```

#### PEA — push effective address

Z80: push struct pointer = `LEA HL, IX+d` + `PUSH HL` (or manual computation)
eZ80: `PEA IX+d` (1 instruction, pushes 24-bit IX+d onto stack)

Useful for passing struct field pointers as function arguments.

#### TST — non-destructive test

Z80: `AND r` destroys A; to test without destroying, need `PUSH AF` / `AND r` / `POP AF`
eZ80: `TST A, r` — sets flags from A AND r without modifying A

```lisp
(rule (lower_ez80 (test_bit_pattern ?a ?mask))
      (tst_a ?mask))  ; flags = A AND mask, A unchanged
```

### 7. Cross-mode calling (future)

ADL mode code can call Z80-mode code and vice versa using suffixes:

| Suffix | From → To | Stack push | Instruction fetch |
|--------|-----------|------------|-------------------|
| `.SIS` | Z80 → Z80 | 2 bytes | 16-bit |
| `.LIL` | ADL → ADL | 3 bytes | 24-bit |
| `.SIL` | Z80 → ADL | 2 bytes push, 24-bit fetch | mixed |
| `.LIS` | ADL → Z80 | 3 bytes push, 16-bit fetch | mixed |

Phase 1 targets pure ADL mode only (all functions in ADL). Cross-mode calling is
deferred to Phase 2, which would enable linking Z80 legacy libraries.

### 8. Phased implementation

| Phase | What | LOC (est.) | Depends on |
|-------|------|------------|------------|
| **1a** | `TargetInfo` struct + `pkg/ez80/` skeleton | ~100 | None |
| **1b** | MZA 24-bit immediate/address encoding | ~300 | None |
| **1c** | eZ80 ADL codegen: basic functions, u8/u24 | ~800 | 1a, 1b |
| **1d** | MLT for u8×u8 | ~50 | 1c |
| **1e** | LEA/PEA for struct access | ~100 | 1c |
| **2a** | Full u24 pointer support (TyPtr = 24-bit) | ~200 | 1c |
| **2b** | MOS API integration (RST.LIL dispatch) | ~100 | 2a |
| **2c** | VDP graphics API | ~100 | 2b |
| **3a** | Cross-mode calling (.SIS/.LIL/.SIL/.LIS) | ~300 | 2a |
| **3b** | LIR backend eZ80 TargetDescriptor | ~400 | ADR-0033 |
| | **Total Phase 1** | **~1350** | |
| | **Total Phase 1+2** | **~1750** | |
| | **Total all** | **~2450** | |

### 9. Testing strategy

- **MIR2 VM (risc32):** u24/pointer tests already work on the generic VM.
  Add eZ80-specific assertion tests alongside C89 corpus.
- **MZE extension:** The Z80 emulator can be extended to emulate eZ80 ADL mode.
  Alternatively, use AgDev's emulator or run on real hardware via MZRUN.
- **Real hardware:** Agon Light 2 via serial upload. The existing MZRUN/DZRP
  protocol works over UART. Test on actual eZ80 at 18.432 MHz.
- **Cross-validation:** QBE oracle (mir2qbe) validates semantics independently
  of the target backend.

### 10. What happens to existing eZ80 code

The current `isEZ80Target` / `defaultADLMode` / `getCallSuffix()` code in
`z80.go` is **removed** once `pkg/ez80/` reaches parity. The Agon examples
migrate from `-b z80 --target=agon` to `-b ez80`. During transition, both paths
coexist.

The existing `ez80_instructions.go` in MZA stays — it's already in the assembler
and serves both backends.

## Consequences

### Positive

- **Clean separation:** Z80 codegen remains stable. eZ80 development can't break
  existing Z80 programs.
- **Full ADL exploitation:** 24-bit pointers, MLT, LEA, PEA, TST — real eZ80
  code, not Z80-with-suffixes.
- **Independent optimization:** eZ80 at 18.432 MHz has different cost tradeoffs
  than Z80 at 3.5 MHz. Code size matters less; throughput matters more. The eZ80
  backend can optimize for speed without compromising Z80 code density.
- **LIR-ready:** Phase 3b gives the LIR backend an eZ80 TargetDescriptor with
  PBQP costs tuned for 24-bit registers and hardware multiply.
- **Community alignment:** Using MZA in ADL mode produces assembly compatible with
  agon-ez80asm syntax, making it easy for Agon developers to read and modify our
  output.

### Negative

- **Two codegens to maintain:** Bug fixes and features may need to be applied to
  both Z80 and eZ80 backends. Mitigated by sharing MIR2 passes and potentially
  extracting common patterns to a shared helper package.
- **MZA complexity:** 24-bit encoding adds complexity to the assembler. Mitigated
  by clean `CPUMode` branching already in place.
- **No emulator yet:** MZE doesn't emulate eZ80. Phase 1 testing relies on
  real hardware or external emulators (Fab Agon Emulator).

### Neutral

- Existing Z80 programs unchanged — no migration needed.
- Agon examples work today via the Z80 path; they'll work better via the eZ80 path.
- MIR2 is already target-neutral; no changes needed to the IR itself.

## Alternatives Considered

### 1. Keep eZ80 as Z80 codegen flags

The current approach. Every function in `z80.go` grows `if g.isEZ80Target` branches.
At 5000+ LOC, this becomes unmaintainable. Already showing strain with the
`$F000` → `$00F000` virtual register comment that's been deferred.

**Rejected:** Architectural complexity grows quadratically with feature count.

### 2. Use fasmg as external assembler

Mature eZ80 support, used by AgDev. But adds an external binary dependency,
complicates the build, and loses our self-contained toolchain guarantee.

**Rejected:** Self-contained toolchain is a core MinZ value. Extending MZA is
modest effort (~300 LOC) and keeps everything in one `go build`.

### 3. Full eZ80 emulator in MZE

Build a complete eZ80 emulator with ADL mode, 24-bit addressing, MLT, LEA, etc.
Would enable the same FUSE-style verification we have for Z80.

**Deferred to Phase 2:** Real hardware testing is sufficient for Phase 1. An
eZ80 emulator is valuable but not blocking.

### 4. Transpile eZ80→Z80 (run eZ80 code on Z80)

Generate eZ80 ADL code, then transpile to equivalent Z80 sequences for testing.
Theoretically possible (eZ80 ⊃ Z80) but the 24-bit→16-bit pointer truncation
makes this semantically wrong.

**Rejected:** The whole point is 24-bit addressing. Can't test that with 16-bit.

## References

- [eZ80 CPU User Manual (UM0077)](https://www.zilog.com/docs/um0077.pdf)
- [AgDev — LLVM/fasmg toolchain for Agon](https://github.com/pcawte/AgDev)
- [agon-ez80asm — native assembler](https://github.com/AgonPlatform/agon-ez80asm)
- [Agon MOS firmware](https://github.com/AgonPlatform/agon-mos)
- [Agon Platform documentation](https://agonconsole8.github.io/agon-docs/)
- [spasm-ng — Z80/eZ80 assembler](https://github.com/alberthdev/spasm-ng)
