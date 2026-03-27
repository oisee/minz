# ADR-0041: BCD Packed Decimal Types for Z80/6502

**Status:** Proposed
**Date:** 2026-03-27
**Context:** COBOL frontend, Z80 DAA instruction, 6502 decimal mode

---

## Problem

COBOL uses PIC 9 types (packed decimal / BCD). Z80 and 6502 have native BCD hardware:
- **Z80:** `DAA` (4T) adjusts A after binary add/sub to produce BCD result. `RLD`/`RRD` (18T) rotate BCD digits between A and memory.
- **6502:** `SED` enables decimal mode — all ADC/SBC automatically produce BCD results. Zero overhead.

Currently MinZ has no BCD type. All arithmetic is binary. COBOL `PIC 99 VALUE 42` would need runtime conversion (expensive). With native BCD types, the compiler can emit `DAA` / `SED` directly.

## Decision

Add BCD types to the MinZ type system as first-class numeric types.

### Type Definitions

```
TyBCD8   — 1 byte,  2 BCD digits (00-99)
TyBCD16  — 2 bytes, 4 BCD digits (0000-9999)
TyBCD24  — 3 bytes, 6 BCD digits (000000-999999)
TyBCD32  — 4 bytes, 8 BCD digits (00000000-99999999)
```

Each byte stores two BCD digits (high nibble = tens, low nibble = units).
Value `42` stored as `0x42` (not `0x2A`).

### Storage Layout

```
BCD8:  [byte 0: tens|units]
       42 → 0x42

BCD16: [byte 0: thousands|hundreds] [byte 1: tens|units]  (big-endian BCD)
       OR: [byte 0: tens|units] [byte 1: thousands|hundreds]  (little-endian BCD)

       Decision: little-endian BCD (matches Z80 byte order)
       1234 → [0x34, 0x12]  (LE: low digits first)
```

### Z80 Codegen

#### BCD8 Addition
```z80
; A = bcd8_a, B = bcd8_b
ADD A, B        ; binary add
DAA             ; decimal adjust → BCD result (4T)
; Total: 2 instructions, 8T
```

#### BCD8 Subtraction
```z80
; A = bcd8_a, B = bcd8_b
SUB B           ; binary subtract
DAA             ; decimal adjust → BCD result (4T)
; Total: 2 instructions, 8T
```

#### BCD16 Addition (multi-byte)
```z80
; HL = &bcd16_a, DE = &bcd16_b
LD A, (HL)      ; low byte (tens|units)
LD C, A
LD A, (DE)
ADD A, C
DAA             ; adjust low BCD byte
LD (HL), A
INC HL
INC DE
LD A, (HL)      ; high byte (thousands|hundreds)
LD C, A
LD A, (DE)
ADC A, C        ; add with carry from low byte
DAA             ; adjust high BCD byte
LD (HL), A
```

#### BCD Digit Extraction (RLD/RRD)
```z80
; Extract low digit of (HL) into A[3:0]
XOR A           ; A = 0
RRD             ; A[3:0] = (HL)[3:0], (HL) rotated
AND 0x0F        ; mask to single digit
; Result: A = units digit (0-9)
```

### 6502 Codegen

```asm
; BCD addition — native decimal mode
SED             ; set decimal flag
LDA bcd_a
CLC
ADC bcd_b       ; BCD add (automatic!)
STA bcd_result
CLD             ; clear decimal flag

; BCD subtraction
SED
LDA bcd_a
SEC
SBC bcd_b       ; BCD subtract (automatic!)
STA bcd_result
CLD
```

### COBOL Mapping

```
PIC 9         → TyBCD8   (1 digit, stored in 1 byte, range 0-9)
PIC 99        → TyBCD8   (2 digits, 1 byte, range 00-99)
PIC 9(3)      → TyBCD16  (3 digits, 2 bytes, range 000-999)
PIC 9(4)      → TyBCD16  (4 digits, 2 bytes, range 0000-9999)
PIC 9(5)-9(6) → TyBCD24  (5-6 digits, 3 bytes)
PIC 9(7)-9(8) → TyBCD32  (7-8 digits, 4 bytes)
PIC S9(N)     → signed variant (sign in high nibble or separate byte)
PIC 9(N)V9(M) → fixed-point: integer + fractional BCD digits
```

### Nanz Syntax (proposed)

```nanz
var price: bcd16 = 1999       // 0x19, 0x99
var tax: bcd8 = 20            // 0x20
var total: bcd16 = price + tax // DAA after each add
```

### C Syntax (proposed)

```c
#include <stdfix.h>   // or <bcd.h>
typedef _BCD(2) bcd8;    // 2 BCD digits
typedef _BCD(4) bcd16;   // 4 BCD digits

bcd8 price = BCD(42);     // 0x42
bcd8 tax = BCD(10);       // 0x10
bcd8 total = price + tax;  // 0x52 (via DAA)
```

## Alternatives Considered

### A. Library-only BCD (no type system change)
Runtime functions: `bcd_add(a, b)`, `bcd_sub(a, b)`, `bcd_to_bin(x)`, `bin_to_bcd(x)`.
**Rejected:** CALL overhead per operation. DAA is 4T, CALL is 17T+10T. Type system enables inline DAA.

### B. BCD as annotation (attribute)
```c
[[bcd]] uint8_t price = 0x42;
```
**Rejected:** Compiler can't distinguish BCD+BCD from BCD+binary. Needs real type for type checking.

### C. Fixed-point only (no BCD)
Use binary fixed-point (Q8.8) instead of BCD.
**Rejected:** COBOL requires BCD — financial arithmetic must be decimal-exact. Binary fixed-point has rounding errors (0.1 can't be represented).

## Implementation Plan

### Phase 1: Type System (~30 LOC)
- Add `TyBCD8`, `TyBCD16` to `mir2/types.go`
- Width: BCD8=8, BCD16=16 (same storage as u8/u16)
- `IsBCD()` method on Ty

### Phase 2: Z80 Codegen (~40 LOC)
- `genAdd` / `genSub`: if both operands are BCD, emit DAA after ADD/SUB
- `genBCDConst`: emit BCD constant (42 → 0x42, not 0x2A)

### Phase 3: Binary↔BCD Conversion (~30 LOC)
- `bin_to_bcd(u8) → bcd8`: binary to packed BCD
- `bcd_to_bin(bcd8) → u8`: packed BCD to binary
- Used at system boundaries (DISPLAY, I/O)

### Phase 4: COBOL Frontend (separate ADR)
- PIC clause parser
- COBOL → HIR with BCD types

### Phase 5: 6502 Codegen (~20 LOC)
- SED before BCD arithmetic block
- CLD after
- Bracket pattern: SED...CLD around BCD expressions

## Open Questions

1. **Endianness:** Big-endian BCD (natural digit order) or little-endian BCD (matches Z80)?
   - Z80 is LE → LE BCD avoids byte reversal
   - COBOL displays big-endian → need swap for DISPLAY
   - **Proposal:** LE storage, convert on display

2. **Signed BCD:** How to represent sign?
   - Option A: High nibble of MSB = 0xC (positive) / 0xD (negative) — IBM mainframe convention
   - Option B: Separate sign byte — simpler
   - **Proposal:** Separate sign byte for Z80 (simpler codegen)

3. **Fixed-point BCD (PIC 9V99):** Implicit decimal point?
   - Store as integer BCD, track scale at compile time
   - `PIC 99V99` = BCD16 with scale=2 (divide by 100 for display)
   - **Proposal:** Scale as compile-time metadata, not runtime

4. **VIR solver:** Can Z3 model DAA?
   - DAA is complex (depends on H flag, C flag, upper/lower nibble ranges)
   - **Proposal:** Treat BCD ops as opaque (like mul8 inline) — Z3 allocates registers, DAA emitted as fixed postfix

5. **Mixed BCD + binary:** What happens for `bcd8 + u8`?
   - **Proposal:** Compile error. Explicit `bcd_to_bin()` / `bin_to_bcd()` required.

## References

- Z80 User Manual: DAA instruction (p.173)
- Z80 User Manual: RLD/RRD instructions (p.219-221)
- MOS 6502: Decimal mode (SED/CLD)
- COBOL PIC clause: ISO/IEC 1989:2014 §8.5.2
- IBM mainframe packed decimal: PACK/UNPK instructions

---

## GPT-5.4 Review (2026-03-27)

1. **Big-endian BCD recommended** — COBOL convention, IBM standard, human-readable. LE is Z80-native but non-standard.
2. **IBM sign nibble 0xC/0xD** — compact, industry standard. 0xC=positive, 0xD=negative, 0xF=unsigned.
3. **Compile-time scale** — matches COBOL PIC clause semantics. No runtime scale tracking needed.
4. **Mixed BCD+binary = compile error** — explicit `bcd_to_bin()`/`bin_to_bcd()` required.
5. **Multi-byte carry propagation** — ADC+DAA per byte, test edge cases (99+01=00 carry).
6. **Reference implementations:** GnuCOBOL, IBM XL C/C++ packed decimal.

**Revised proposal:** Big-endian BCD (COBOL-native) despite Z80 LE. Conversion cost: 1 byte-swap for 2-byte BCD16 at load/store. Worth it for COBOL compatibility.

*Pending: VIR backend team review (Z3 modeling, register constraints).*
