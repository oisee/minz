# Report #029 — MIR2 Architecture Foundations & Progress

**Date:** 2026-03-07
**Status:** Phase 1 done, Phase 2 at 75%, architecture for Phase 3–5 locked

---

## Where we are

```
Phase 1  ████████████████████  100%  Core pipeline            ✅ DONE
Phase 2  ███████████████░░░░░   75%  Codegen quality          ← HERE
Phase 3  ░░░░░░░░░░░░░░░░░░░░    0%  Feature coverage
Phase 4  ░░░░░░░░░░░░░░░░░░░░    0%  minz→MIR2 lowering
Phase 5  ░░░░░░░░░░░░░░░░░░░░    0%  Optimisation (PBQP)

Overall  ████░░░░░░░░░░░░░░░░   ~22% → production + PBQP
```

42 tests pass. 9 verified functions. The codegen produces correct, readable
Z80 assembly for all examples. Phase 2 has two items left.

---

## What landed since #028

### Topology-aware register alias tracking (`holdsPhys`)

Replaced `aHoldsPhys string` with `holdsPhys map[string]string` — a proper
bidirectional copy-propagation map covering all physical registers.

The key addition: Z80 register hierarchy tables `regBytes` / `regParent`.
Writing to `D` must invalidate `DE` (its parent pair). Writing to `HL` must
invalidate `H` and `L` (its byte components). Without this, `INC DE` after
`LD D, A` would leave a false alias `holdsPhys["A"]="D"` — silent corruption.

```go
var regBytes = map[string][]string{
    "BC": {"B","C"}, "DE": {"D","E"}, "HL": {"H","L"}, ...
}
var regParent = map[string]string{
    "B":"BC", "C":"BC", "D":"DE", "E":"DE", "H":"HL", "L":"HL", ...
}

func (g *z80cg) invalidate(r string) {
    g.killOne(r)
    if bytes, ok := regBytes[r]; ok { for _, b := range bytes { g.killOne(b) } }
    if parent, ok := regParent[r]; ok { g.killOne(parent) }
}
```

`invalidate` is now called at every register write: INC/DEC (8+16-bit),
SLA/SRL/SRA, NEG, CPL, ADD/SBC 16-bit, all ALU paths. Previously INC/DEC
and shifts left stale aliases.

`emitMov` now records 8-bit copies: `setCopy(dst, src)`. This enables
cross-instruction coalescing — after `LD C, A`, a later instruction that
needs A from C will skip the `LD A, C`.

### Array layout architecture (ADR-0012)

Locked the design for three memory layouts before Phase 3 starts — changing
this later would break memory layout ABI:

```go
type ArrayLayout uint8
const (
    LayoutAoS    ArrayLayout = iota  // C-style row-major (default)
    LayoutSoA                        // columnar: one array per struct field
    LayoutSoA256                     // page-aligned SoA: H=field, L=index
)
```

**The SoA256 insight:** if a struct's field arrays each occupy a consecutive
256-byte page starting at `$xx00`:

```
$C000: field_x[0..N]   ; page $C0
$C100: field_y[0..N]   ; page $C1
$C200: field_z[0..N]   ; page $C2
```

Then `H` = column selector, `L` = element index. Cross-column access costs
`INC H` (4T) instead of a full pointer reload. Random element access costs
`LD L, i` (7T) instead of a multiply. And `B` is freed for DJNZ — no pointer
pairs needed for three simultaneous columns.

```asm
; Three fields, one loop, no multiplies, B=DJNZ counter:
    LD H, $C0  ; base page
    LD L, 0    ; element 0
    LD B, N    ; count
.loop:
    LD A, (HL) ; x[L]
    INC H
    ADD A,(HL) ; + y[L]
    INC H
    ADD A,(HL) ; + z[L]
    LD H, $C0  ; restore base
    INC L      ; next element
    DJNZ .loop
```

Constraint: `Len ≤ 256`, global allocation only (stack can't be page-aligned).

### New types added

`types.go` now has:
- `ArrayTy` — enhanced with `Layout ArrayLayout` + `Align int`
- `StructTy` — named fields + `ByteOffset(i int)` helper (Phase 3 lowering target)
- `SliceTy` — fat pointer `(base_ptr, len)` for iterator-style access

---

## What PBQP is actually for (clarification)

PBQP for intra-function register allocation will never be needed on Z80 —
7 GP registers, 3–5 live ranges in current examples, greedy is already optimal.

PBQP applies in four genuinely useful places:

| Domain | Phase | What it optimises |
|--------|-------|-------------------|
| **Function contracts** | 5b | Which register carries each argument/return across the call graph — minimise save/restore overhead at every call site |
| **SoA256 page order** | 5c | Which page (H value) each field array gets — minimise `LD H,page` for frequently co-accessed fields |
| **EXX region assignment** | 5a | Which variables live in B'/C'/D'/E' during EXX blocks |
| **Bank switching** | 5d | Which code/data blocks share a bank on 128K targets |

All four are **interprocedural** or **cross-object** problems — they can't be
solved by looking at one function or one array at a time.

---

## Phase 2 remaining

Two items:

**1. `TermBrIf2` — multi-flag branch**

GCD currently emits a redundant `CP B` because the branch uses Z flag but
the prior `CP B` only set C. A `TermBrIf2` terminal can branch on two
conditions (Z and C) from one compare:

```asm
; gcd after TermBrIf2:
.gcd_ge:
    JP Z, .gcd_done    ; both flags from the same CP
    SUB B
    JP .gcd_loop
```

vs the current redundant version:
```asm
.gcd_ge:
    CP B               ; ← redundant, flags already set
    JP Z, .gcd_done
```

**2. More Phase 2 examples:** `rol8`, `is_pow2`, `min16`, `swap_nibbles`.
These exercise the 16-bit comparison path and rotation instructions.

---

## What's next (Phase 3 start)

After Phase 2:
1. `OpGlobal(sym)` → `LD HL, label` — the simplest Phase 3 opcode
2. `OpField(base, offset)` — struct field pointer
3. `OpPtrBump(ptr, stride_const)` — iterator advance (`INC HL` / `ADD HL,BC`)
4. First multi-function module test

Full plan: [MIR2 Roadmap](../docs/MIR2_Roadmap.md)
Layout ADR: [ADR-0012](../docs/adr/0012-mir2-array-layout-soa256.md)
