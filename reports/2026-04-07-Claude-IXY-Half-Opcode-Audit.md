# IX/IY Half-Register Opcode Family Audit

**Date:** 2026-04-07
**Scope:** Diagnose `write_fat12: BIT 0, IYL` and identify all affected opcode families.
**Prior fix:** `a0f1094c` — fixed `LD (HL), IXL` in OpStore path (3 sites).

---

## Z80 Encoding Rules for IX/IY Halves

The DD/FD prefix that enables IXH/IXL/IYH/IYL **also** transforms other parts of the instruction encoding. This creates forbidden combinations:

| Opcode family | IXH/IXL/IYH/IYL allowed? | Why |
|---------------|--------------------------|-----|
| `LD r, r'` | Partial — see below | DD prefix substitutes H→IXH, L→IXL |
| `LD (HL), r` | **NO** | DD makes (HL)→(IX+d), double conflict |
| `LD r, (HL)` | **NO** | Same reason |
| `ADD/SUB/AND/OR/XOR/CP r` | **YES** | DD CB prefix works for 8-bit ALU |
| `INC/DEC r` | **YES** | DD prefix works |
| `BIT n, r` | **NO** | CB prefix encoding, no DD/FD CB r form |
| `SET n, r` | **NO** | Same as BIT |
| `RES n, r` | **NO** | Same as RES |
| `RLC/RRC/RL/RR/SLA/SRA/SRL r` | **NO** | CB prefix, same issue |

**Safe lowering rule:** When emitting `BIT/SET/RES/RLC/RRC/RL/RR/SLA/SRA/SRL` or `LD (HL)/LD r,(HL)`, the register operand must be one of `{A, B, C, D, E, H, L}`. If the allocated register is IXH/IXL/IYH/IYL, route through A first.

---

## Root Cause: `write_fat12: BIT 0, IYL`

Source: `write_fat12` in `stdlib/fs/fat12.minz` checks `clst & 1` (odd/even cluster for FAT12 bit packing).

PBQP allocates `clst` to IY. The comparison path detects a single-bit test pattern and emits `BIT 0, IYL` via:

1. `selectBitReg("IY", 0)` → `lowByte("IY")` → `"IYL"` (line 1974-1977)
2. `isSimpleReg("IYL")` → `true` (only checks alphabetic chars, line 882-891)
3. `emitBitRegOp("BIT", 0, "IYL")` → `BIT 0, IYL` emitted (line 1985-1995)

The comment at line 1992 says "IXH/IXL/IYH/IYL are first-class 8-bit regs here" — **this is wrong** for BIT/SET/RES.

---

## All Affected Sites in z80codegen.go

### 1. `emitBitRegOp` — lines 1985-1995

The central function for BIT/SET/RES register ops. Missing IXY guard.

```go
func (g *z80cg) emitBitRegOp(op string, bit int, reg string) bool {
    if reg == "" || reg == "A" || isSpill(reg) || reg == "F" {
        return false
    }
    if !isSimpleReg(reg) {
        return false
    }
    // WRONG: "IXH/IXL/IYH/IYL are first-class 8-bit regs here"
    g.emitf("    %s %d, %s", op, bit, reg)
    return true
}
```

**Fix:** Add `isIXYReg(reg)` to the rejection list. Return false → caller falls through to the AND/OR mask path.

### 2. `selectBitReg` — lines 1974-1983

Returns `lowByte(src)` / `highByte(src)` which can be IXL/IYL/IXH/IYH for IX/IY pairs. Every caller must check the result.

**Callers:**

| Line | Context | Guarded? |
|------|---------|----------|
| 2707 | OpBitSet/OpBitReset | **NO** — passes to emitBitRegOp without IX check |
| 3628 | genBinOp 8-bit OR single-bit mask → SET | **NO** — same |
| 3637 | genBinOp 8-bit AND single-bit mask → RES | **NO** — same |
| 3855 | genBinOp16 16-bit OR single-bit mask → SET | **NO** — same |
| 3866 | genBinOp16 16-bit AND single-bit mask → RES | **NO** — same |
| 5662 | OpCmp bit-test pattern → BIT | **NO** — uses isSimpleReg which accepts IXL |

### 3. Direct BIT emit — line 5664

```go
src = selectBitReg(src, pat.byteOffset)
if isSimpleReg(src) && !isSpill(src) {
    g.emitf("    BIT %d, %s", pat.bit, src)
```

**Fix:** Add `&& !isIXYReg(src)` to the guard.

---

## Minimal Repros

### Repro 1: BIT on IY half (write_fat12 trigger)

```nanz
fun test_bit_iyhalf(x: u16) -> u8 {
    if x & 1 == 0 {
        return 10
    }
    return 20
}
```

Compile via PBQP path with enough register pressure to push `x` into IY. In `write_fat12` this happens naturally because 9+ vregs are live.

### Repro 2: SET/RES on IX half

```nanz
fun test_set_ixhalf(x: u16) -> u16 {
    return x | 4   // SET 2, lowByte(x) if x in IX
}
```

### Actual repro (guaranteed):

```bash
cd minzc
go run ./cmd/minzc /home/alice/dev/minz/stdlib/fs/fat12.minz -o /tmp/fat12.a80 2>&1 | grep Z80-VALIDATE
# Output: [Z80-VALIDATE] write_fat12: 2 invalid instruction(s)
```

---

## Safe Lowering Rule

**For BIT/SET/RES n, r:**

```
if reg ∈ {A, B, C, D, E, H, L}:
    emit "BIT/SET/RES n, reg"        ← valid
elif reg ∈ {IXH, IXL, IYH, IYL}:
    return false                      ← fall through to AND/OR mask path
elif reg is pair and isIXY(pair):
    return false                      ← lowByte/highByte would be IXH/IXL
else:
    return false                      ← spill, unknown
```

The AND/OR mask fallback already exists in every caller — `emitBitRegOp` returning false triggers:
- `BIT n, r` → `LD A, r; AND (1<<n); ...` (test via AND mask)
- `SET n, r` → `LD A, r; OR (1<<n); LD r, A` (set via OR mask)
- `RES n, r` → `LD A, r; AND ~(1<<n); LD r, A` (reset via AND mask)

**Single-point fix:** Add `|| isIXYReg(reg)` to line 1986 in `emitBitRegOp`. All 6 callers automatically fall through.

---

## Shift/Rotate Opcodes (Same Class, Not Yet Triggered)

`RLC/RRC/RL/RR/SLA/SRA/SRL r` use CB prefix and have the same encoding constraint as BIT/SET/RES — IXH/IXL/IYH/IYL are not valid operands.

These are emitted by `genShift` in z80codegen.go. Current code uses SRL/SRA/SLA/RR on the allocated register. If PBQP assigns a shift operand to IXH/IXL, invalid instructions would be emitted.

Not yet triggered in test corpus but structurally the same bug class.

---

## Summary

| Issue | Status | Fix |
|-------|--------|-----|
| `LD (HL), IXL` | **FIXED** (a0f1094c) | isPairReg guard excludes IX/IY |
| `BIT/SET/RES n, IXH/IXL/IYH/IYL` | **OPEN** — 6 sites | Add `isIXYReg(reg)` to `emitBitRegOp` line 1986 |
| `BIT n, IXL` direct emit | **OPEN** — 1 site (line 5664) | Add `!isIXYReg(src)` guard |
| `SRL/SRA/SLA/RL/RR r` on IX halves | **LATENT** — not yet triggered | Needs audit of `genShift` |

---

## Fix Applied

### Changed lines

**`minzc/pkg/mir2/z80codegen.go`:**

1. `emitBitRegOp` (line 1985): added `isIXYReg(reg)` rejection — single-point fix for all 6 BIT/SET/RES callers.
2. Direct BIT emit (line 5667): added `&& !isIXYReg(src)` guard.

**`minzc/pkg/mir2/z80codegen_test.go`:**

Updated 6 tests that previously asserted invalid `BIT/SET/RES n, IXH/IXL/IYH/IYL` as correct behavior:
- `TestBitRegPattern_DirectBitReset_U8_IXL` — now asserts AND mask fallback
- `TestBitRegPattern_RES_U8_IXL` — now asserts LD A,IXL; AND mask
- `TestBitCmpPattern_DirectBitGet_U16_IXRegPair` — now asserts no BIT n,IXH
- `TestBitRegPattern_SET_U16_IXHigh` — now asserts no SET n,IXH
- `TestBitRegPattern_RES_U16_IYLow` — now asserts no RES n,IYL
- `TestBitCmpPattern_ShiftedBitRead_U16_IXRegPair` — now asserts no BIT n,IXH

### Validation

```bash
# All Z80-VALIDATE errors gone for fat12.minz:
go run ./cmd/minzc stdlib/fs/fat12.minz -o /tmp/fat12.a80 2>&1 | grep Z80-VALIDATE
# (no output)

# FatFS differential: no more invalid BIT/SET/RES:
go test ./pkg/c89/ -run "Differential_Z80" -count=1 -timeout 120s
# PASS

# MIR2 test suite (1 pre-existing failure unrelated):
go test ./pkg/mir2/ -count=1 -timeout 120s
# Only TestStrLenZ80 fails (pre-existing, string DB format expectation)
```
