# Report 060 — MIR2 Pattern Matching & Peephole: Unified Proposal

**Date**: 2026-03-12
**Status**: Proposal (not yet implemented)
**Author**: Claude Opus 4.6 (analysis session with Alice)
**Depends on**: Report 058 (multiply strength reduction)
**References**: antique-toy/ch04-maths (Dark/X-Trade), MIR1 optimizer infrastructure

---

## Motivation

MIR2 has **2 asm peephole patterns** (double EX DE,HL, single-JP→EQU) and **1 IR-level
combine** (Sub+Cmp fusion in condret.go). Meanwhile MIR1 has **90+ patterns** across
three layers (48 regex asm, 8 specialized handlers, 32 MIR-level, plus loop rerolling).

MIR2 is the active pipeline. MIR1 is frozen. The gap is clear: MIR2 needs its own
pattern matching infrastructure, informed by MIR1's experience but designed for
MIR2's block-argument SSA form.

Three concrete problems motivate this now:

1. **`(x * 256) + y` → `LD H, x / LD L, y`** — IR combine eliminates dead `LD L, 0`
2. **`LD A, B / LD L, A` → `LD L, B`** — asm peephole eliminates copy-through-A
3. **Repeated putchar → @print** — loop rerolling needs adaptation for MIR2

---

## Current State

### What MIR2 has today

| Layer | File | Patterns | Infrastructure |
|-------|------|----------|----------------|
| IR constant fold | `constprop.go` | ~15 algebraic | Per-instruction, constants only |
| IR combine | `condret.go` | 1 (Sub+Cmp) | Hand-coded, block-local |
| IR simplify | `constprop.go:407` | 1 (PtrAdd+0) | Post-fold cleanup |
| IR DSE | `dse.go` | 1 iterative | Use-count based |
| ASM peephole | `z80codegen.go:156` | 2 string-based | Line array, comment-aware |

### What MIR1 has (frozen, for reference)

| Layer | File | Patterns | Infrastructure |
|-------|------|----------|----------------|
| MIR peephole | `mir_peephole.go` | 32 patterns | 6 passes, fixpoint iteration |
| ASM regex | `assembly_peephole.go` | 48 regex + 8 handlers | 5-iteration regex, then handlers |
| Loop reroll | `loop_reroll.go` | Template matcher | PatternTemplate + captures |
| Superopt | `superopt_peephole.go` | External JSON DB | Rule database |

---

## Proposal: Two-Layer Architecture

### Layer 1: MIR2 IR Combine Pass (`pkg/mir2/combine.go`)

A new pass that pattern-matches multi-instruction sequences within a block
and replaces them with fused instructions or synthetic opcodes.

**Design:**

```go
// combine.go — MIR2 instruction combining pass
//
// Runs after constprop, before regalloc. Iterates blocks looking for
// adjacent instruction patterns and fusing them.
//
// Each pattern is a function: func(blk *Block, i int) bool
// Returns true if a rewrite happened (triggers re-scan).

func CombinePass(f *Func) bool {
    changed := false
    for _, blk := range f.Blocks {
        // Scan pairs/triples of adjacent instructions
        for i := 0; i < len(blk.Insts)-1; i++ {
            for _, rule := range combineRules {
                if rule(blk, i) {
                    changed = true
                    break // rescan from same position
                }
            }
        }
    }
    return changed
}

var combineRules = []func(*Block, int) bool{
    combineMulAddComposePair,   // (x * 256) + y → compose_pair(x, y)
    combineMulAddGeneral,       // (x * K) + y where K is power-of-2 ≥ 256
    combineLoadFieldDirect,     // load(field(ptr, N)) → load_offset(ptr, N)
    combineConsecutiveStores,   // store(p, v1); store(p+1, v2) → store16(p, v1:v2)
}
```

**Pattern 1: Mul-Add Compose Pair**

The motivating case. Detects:
```
%r1 = mul %x, Const(256)    ; Ty = u16
%r2 = add %r1, %y           ; Ty = u16, %y is u8 (zero-extended)
```

Rewrites to synthetic instruction:
```
%r2 = compose_pair %x, %y   ; Ty = u16, Cls = ClassPair
```

Codegen for `OpComposePair`:
```z80
LD H, <x>      ; high byte = x
LD L, <y>      ; low byte = y
```

This is **not just `* 256`** — it's the general "make a 16-bit value from two 8-bit
halves" pattern. Screen address calculation, sprite coordinates, packed BCD,
protocol framing — all use this.

Generalization for `* K` where K is a power-of-2 ≥ 256:
```
%r1 = mul %x, Const(K)
%r2 = add %r1, %y
→ %tmp = compose_pair %x, %y; %r2 = shl %tmp, log2(K)-8
```

**Pattern 2: Consecutive putchar → PrintString (from MIR1 loop_reroll)**

MIR1's `loop_reroll.go` detects repeated `Call("putchar", Const(ch))` and
replaces them with `PrintString(label)`. This needs adaptation for MIR2:

MIR2 representation:
```
%r1 = const 72          ; 'H'
%_  = call "putchar" (%r1)
%r2 = const 101         ; 'e'
%_  = call "putchar" (%r2)
...
```

Pattern: sequence of `OpConst` + `OpCall("putchar")` pairs, all in the same block.
Rewrite: emit string data, replace sequence with single `OpCall("__print_string", addr)`.

Key differences from MIR1:
- MIR2 uses virtual registers (not stack-based IR), so const+call are paired by register
- MIR2 has block params, so the pattern must not cross block boundaries
- MIR1 had `OpPrintString` synthetic opcode; MIR2 should use `OpCall` to keep the IR clean

**Pattern 3: Load-after-Field (already partially done)**

Existing in z80codegen.go:1356 as a codegen-level pattern. Move to IR level:
```
%ptr = field %base, Offset(N)
%val = load %ptr
→ %val = load_offset %base, N
```

This enables the codegen to use `LD A, (sym+N)` directly without materializing
the intermediate pointer.

**Why IR level, not codegen level:**

| Concern | Codegen-level | IR-level |
|---------|---------------|----------|
| DSE cleanup | Manual | Automatic (dead `mul` removed by DSE) |
| Regalloc pressure | Intermediate regs allocated | Fewer virtual regs |
| Backend portability | Z80-only | Works for C, QBE, future backends |
| Testability | Need full compile | Unit test on IR |

### Layer 2: MIR2 ASM Peephole Pass (`asmPeepholePass` in z80codegen.go)

Extend the existing string-based pass with patterns ported from MIR1's
`assembly_peephole.go`. Start with highest-impact patterns.

**Infrastructure upgrade:**

Currently `asmPeepholePass` is a flat function calling two helpers. Refactor to
a pattern table for extensibility:

```go
type asmPattern struct {
    name    string
    apply   func(lines []string) []string
}

var asmPatterns = []asmPattern{
    {"elimDoubleExDeHl", elimDoubleExDeHl},         // existing
    {"elimSingleJpEqu", elimSingleJpEqu},           // existing
    {"elimDeadLd", elimDeadLd},                     // NEW
    {"elimCopyThroughA", elimCopyThroughA},          // NEW
    {"elimLdSelf", elimLdSelf},                     // NEW: LD A,A → nop
    {"foldConsecutiveLdZero", foldConsecutiveLdZero}, // from MIR1 pattern 17
    {"elimDoubleNeg", elimDoubleNeg},                // from MIR1
}

func asmPeepholePass(src string) string {
    lines := strings.Split(src, "\n")
    for _, p := range asmPatterns {
        lines = p.apply(lines)
    }
    return strings.Join(lines, "\n")
}
```

**Pattern A: Dead LD Elimination (`elimDeadLd`)**

```
LD L, 0        ; ← remove (L overwritten before read)
...            ; (blank lines, comments only — no instruction reading L)
LD L, A        ; ← L overwritten here
```

Rule: `LD dst, X` followed by `LD dst, Y` with no intervening read of `dst`
→ remove first `LD dst, X`.

Safety: must check that no instruction between them reads `dst`. Simple string
scan for register name in operand position. Comments and blank lines are safe.

**Pattern B: Copy-Through-A Elimination (`elimCopyThroughA`)**

```
LD A, B        ; ← remove (A is just transit)
LD L, A        ; → LD L, B
```

Rule: `LD A, src` followed by `LD dst, A` where src ∈ {B,C,D,E,H,L}
and dst ∈ {B,C,D,E,H,L} → replace pair with `LD dst, src`.

Safety: `LD dst, src` must be a valid Z80 instruction (it is for all
register-to-register combinations except `LD (HL), (HL)`).

Exception: if A is used after the second LD, both instructions must stay.
Conservative approach: only fire when the very next non-blank line after
the pair does NOT read A.

**Pattern C: LD self elimination (`elimLdSelf`)**

```
LD A, A        ; ← remove (no-op, 4T wasted)
```

7 patterns: LD A,A / LD B,B / LD C,C / LD D,D / LD E,E / LD H,H / LD L,L.

**Pattern D: Consecutive LD r, 0 → XOR A + LD r, A (from MIR1)**

```
LD B, 0        ; 7T, 2B
LD C, 0        ; 7T, 2B
→
XOR A          ; 4T, 1B
LD B, A        ; 4T, 1B
LD C, A        ; 4T, 1B
```

Saves 4T and 1 byte per additional zeroed register.

---

## Priority Ranking

### Phase 1 — Immediate wins (both layers, ~100 lines total)

| Pattern | Layer | Impact | Effort |
|---------|-------|--------|--------|
| Dead LD elimination | ASM peephole | Removes `LD L,0` in `*256+x` | ~30 lines |
| Copy-through-A | ASM peephole | `LD A,B / LD L,A` → `LD L,B` | ~25 lines |
| LD self | ASM peephole | Removes 7 no-op variants | ~15 lines |
| Compose pair | IR combine | `mul 256 + add` → `LD H,x / LD L,y` | ~40 lines |

After Phase 1, `pixel_addr(row, col)` compiles to: `LD H, A / LD L, B / RET` (3 insts, 18T).

### Phase 2 — MIR1 port (~200 lines)

Port the highest-value MIR1 asm patterns:

| MIR1 Pattern | ID | Impact |
|--------------|----|--------|
| `LD A, 0` → `XOR A` | #1 | -3T, -1B per occurrence |
| `ADD r, 1` → `INC r` | #3 | -4T per occurrence |
| `SUB r, 1` → `DEC r` | #4 | -4T per occurrence |
| Consecutive LD zero | handler | -4T, -1B per extra reg |
| Double NEG | #36 | -12T, -2B |
| Jump to next label | handler | -10T, -3B |
| `DEC B / JR NZ` → `DJNZ` | handler | -6T, -2B |

### Phase 3 — Reroll for MIR2 (~150 lines)

Adapt `loop_reroll.go` template matcher for MIR2's register-based IR:
- Detect repeated `OpConst(char) + OpCall("putchar")` sequences
- Collect characters into string literal
- Replace with single `OpCall("__print_string", addr)`
- Emit string data in `.data` section

### Phase 4 — General IR combine framework (~200 lines)

Build out `combine.go` with:
- Load-after-Field fusion
- Consecutive stores → 16-bit store
- Mul(x, Const) + Add(result, y) generalization beyond 256
- Sub+Neg → NegSub (for abs_diff variant patterns)

---

## Architectural Decisions

### Q: Why not just regex like MIR1?

MIR1's `assembly_peephole.go` uses 48 compiled regexes applied 5 times.
This works but is:
- **Fragile**: whitespace, comments, label format changes break patterns
- **Slow**: 48 × 5 = 240 regex passes over the full output
- **Hard to test**: must compile full programs to exercise patterns

For MIR2, prefer:
- **IR-level combines** for semantic patterns (mul+add, field+load)
- **String-based line matching** (no regex) for asm patterns — faster, simpler

### Q: Why not a rule database like superopt?

`superopt_peephole.go` loads patterns from JSON. Powerful but:
- Requires maintaining external data files
- Pattern discovery is a separate (expensive) process
- Overkill for the ~20 patterns we need now

Use hand-coded patterns. If pattern count exceeds ~50, revisit.

### Q: Where in the pipeline does CombinePass run?

```
constprop → fold → simplify → COMBINE → condret → dse → alloc → codegen → asmPeephole
                                  ↑ NEW                                        ↑ EXTENDED
```

After constant propagation (so `Const(256)` is available) and before DSE
(so dead intermediate `mul` results get cleaned up automatically).

### Q: Should OpComposePair be a real opcode?

**Yes.** Adding it to `ops.go` is cleaner than emitting it inline:
- DSE, regalloc, dumper all handle it via existing `IsBinary` predicate
- Codegen emits `LD H, <high> / LD L, <low>` — trivial
- C backend emits `(high << 8) | low` — also trivial
- QBE backend emits `or(shl(ext(high), 8), ext(low))`

Only ~10 lines to wire up across ops.go, z80codegen.go, and builder.go.

---

## pixel_addr Walkthrough: Before vs After

### Current output (after `* 256` fix, but no peephole):
```z80
pixel_addr:
    LD H, A        ; H = row           ← from genMul16 *256 fix
    LD L, 0        ; L = 0             ← dead store (killed by add)
    LD A, B        ; A = col           ← copy-through-A
    LD L, A        ; L = col           ← overwrites L=0
    RET
```
4 instructions, 26T.

### After Phase 1 (IR combine + asm peephole):

**IR combine** detects `mul(row, 256) + zext(col)` → `compose_pair(row, col)`:
```z80
pixel_addr:
    LD H, A        ; H = row
    LD L, B        ; L = col
    RET
```
2 instructions, 15T. **42% faster, 50% smaller.**

If IR combine doesn't fire (e.g. non-constant multiplier), asm peephole catches it:
1. `elimDeadLd`: removes `LD L, 0` (overwritten by `LD L, A`)
2. `elimCopyThroughA`: `LD A, B / LD L, A` → `LD L, B`

Result: same optimal code via either path.

---

## Verification

- [ ] `go test ./pkg/mir2/...` — combine pass unit tests
- [ ] `go test ./...` — full suite green
- [ ] Compile `pixel_addr` example → inspect .a80 for `LD H, A / LD L, B / RET`
- [ ] Compile `fill_rect` → verify `* 256` path uses compose_pair
- [ ] Compile "Hello World" with putchar chain → verify string reroll fires
- [ ] No regressions in `compile_all_examples.sh` (≥71/73)

---

## Key Files To Modify

| File | Change |
|------|--------|
| `pkg/mir2/ops.go` | Add `OpComposePair` opcode |
| `pkg/mir2/combine.go` | **NEW** — IR combine pass |
| `pkg/mir2/combine_test.go` | **NEW** — unit tests |
| `pkg/mir2/z80codegen.go:156` | Extend `asmPeepholePass()` with pattern table |
| `pkg/mir2/z80codegen.go` | Add `OpComposePair` codegen case |
| `pkg/mir2/builder.go` | Add `ComposePair(hi, lo Reg) Reg` helper |
| `pkg/mir2/asmpeephole_test.go` | Add tests for new asm patterns |

**Not modified:** `constprop.go`, `dse.go`, `alloc.go` — they handle new opcodes
automatically via existing `IsBinary`/`isPure` predicates.

---

## Summary

Two layers, four phases, ~650 lines total:

1. **IR combine** (`combine.go`): semantic patterns — mul+add→compose, field+load→direct, putchar chain→string
2. **ASM peephole** (`asmPeepholePass`): mechanical patterns — dead LD, copy-through-A, LD self, consecutive zeros

MIR1 proved these patterns work. MIR2 gets a cleaner implementation because:
- Block-argument SSA (no phi mess)
- String-based asm matching (no fragile regex)
- IR combines cleaned up automatically by existing DSE

The `pixel_addr` case goes from 4 instructions (26T) to 2 instructions (15T).
The `fill_rect` inner loop drops by ~50T per iteration.
Print strings shrink from N×(const+call) to 1 call.
