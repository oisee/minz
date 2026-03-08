# MIR2 Development Roadmap

`pkg/mir2` — clean-room IR + register allocator + Z80 codegen,
developed in parallel with the production compiler.

---

## Progress

```
Phase 1  ████████████████████  100%  Core pipeline            ✅ DONE
Phase 2  ████████████████████  100%  Codegen quality          ✅ DONE
Phase 3  ░░░░░░░░░░░░░░░░░░░░    0%  Feature coverage         ← HERE
Phase 4  ░░░░░░░░░░░░░░░░░░░░    0%  minz→MIR2 lowering
Phase 5  ░░░░░░░░░░░░░░░░░░░░    0%  Optimisation (PBQP)

Overall  █████░░░░░░░░░░░░░░░   ~25% → production + PBQP
         │    │         │    │
         │    │         │    └─ 100% — PBQP memory layout live
         │    │         └─────  ~80% — real MinZ programs compile
         │    └───────────────  ~55% — feature-complete
         └────────────────────  ~25% — codegen quality done
```

Weight model (effort estimate): Phase 1=15%, 2=10%, 3=30%, 4=25%, 5=20%.
Overall = P1×1.0 + P2×0.75 + P3×0 + P4×0 + P5×0 = 22.5%.

---

## Phase 1 — Core pipeline ✅ DONE

- [x] SSA-like IR with block arguments
- [x] Interference graph + greedy register allocator
- [x] Z80CostTable with 14 register classes
- [x] Z80 codegen: 8/16-bit arithmetic, branches, loops, calls
- [x] Parallel-copy resolution (HL↔DE cycles, trampolines)
- [x] Peepholes: INC/DEC for ±1, CP imm8, NEG+ADD for rhs=A
- [x] Shadow register guard (InfCost — ADR-0011)
- [x] DSE pass: dead constant suppression
- [x] AND/OR/XOR/ADD/SUB immediate peepholes
- [x] `holdsPhys` — topology-aware intra-block copy propagation
- [x] E2E verified: fib, clamp, abs_diff, sum_range, mul8, min8, gcd, max3, popcount

---

## Phase 2 — Codegen quality (CURRENT, ~75%)

- [x] DSE pass — suppress `OpConst` emission when peephole made it dead
- [x] AND/OR/XOR/ADD/SUB imm8 — no register needed for rhs constant
- [x] `holdsPhys` — bidirectional alias map, topology-aware (INC DE invalidates D+E)
- [x] **TermBrIf2** — three-way branch using Z and C from one CP
      eliminates the redundant `CP B` in GCD; `ge` block gone entirely
- [x] More examples: `rol8`, `is_pow2`, `swap_nibbles` — shift+OR idiom, bit-trick
- [x] `genShift` constant-count: emits N copies of SLA/SRL for compile-time counts
- [ ] `min16` — deferred: needs 16-bit comparison in genCmp (Phase 3 SBC HL,DE)

---

## Phase 3 — Feature coverage (~30% of total effort)

Goal: cover enough of the language to run real MinZ programs through MIR2.

### 3a — Scalars and pointers
- [x] `OpAddrOf(sym)` → `LD HL, label` + global data emission (`DB`/`DW`)
- [x] VM: `resolveSymbol` handles `Module.Globals` (pre-allocated in heap)
- [x] `compileModule` helper: merged AllocResults for multi-function modules
- [x] E2E verified: `add_to_counter` — global read/add/store, multi-call persistence
- [ ] u8/u16 mixed-width arithmetic (zero-extend, sign-extend — `OpExt`/`OpSext` already exist)
- [ ] Verifier: enforce type width constraints at OpLoad/OpStore

### 3b — Structs
- [ ] `StructTy` in types.go — named fields, fixed layout
- [ ] `OpField(base, field_index)` → pointer + compile-time byte offset
- [ ] AoS codegen: `INC HL` × offset, or `LD BC, offset; ADD HL, BC`
- [ ] SoA lowering: `[N]Struct{f0,f1,f2}` → three separate `[N]Ti` arrays

### 3c — Arrays and iterators
- [ ] `ArrayTy` with `Layout` (AoS/SoA/SoA256) — types.go ✅ stub added
- [ ] `SliceTy` — fat pointer (base_ptr, len) for iterator-style access
- [ ] `OpPtrBump(ptr, stride_const)` → `INC HL` / `INC HL;INC HL` / `ADD HL,BC`
- [ ] SoA256 codegen: `INC H` column switch, `INC L` element advance, `LD L,i` random
- [ ] Verifier: SoA256 constraints (Len≤256, global-only, Align=256)
- [ ] Loop idiom: `OpForRange` → DJNZ with pointer bump

### 3d — Multi-function modules
- [ ] String literals + `@print` intrinsic
- [ ] Multi-function modules with explicit call convention
- [ ] Liveness across calls: spill/reload around OpCall
- [ ] Callee-save PUSH/POP scaffold

---

## Phase 4 — minz → MIR2 lowering (~25% of total effort)

Gating condition: Phase 3 complete + MIR2 passes all old-IR Z80 tests.

- [ ] Semantic lowering pass: MinZ AST → MIR2 (replaces `pkg/ir` pipeline)
- [ ] Iterator chain → DJNZ loop in MIR2 (replaces special-case in `iterator.go`)
- [ ] EXX region detection (ADR-0013 planned) — B'/C'/D'/E' for loop scalars
- [ ] `@layout(soa)` / `@layout(soa256)` annotation in MinZ syntax → `ArrayLayout`
- [ ] Deprecate old `pkg/ir` + `pkg/codegen/z80.go` pipeline

---

## Phase 5 — Optimisation and PBQP (~20% of total effort)

### 5a — Standard optimisations
- [ ] Dead-code elimination (unreachable blocks)
- [ ] Constant propagation across blocks
- [ ] Coalescing: eliminate moves by pre-colouring interfering vregs
- [ ] EXX region allocation (ADR-0013): assign B'/C'/D'/E' to loop scalars

### 5b — PBQP: Function contract optimisation (interprocedural)

Optimise calling conventions across the call graph — which register carries
each argument and return value.

- **Nodes**: functions
- **Variables per node**: register class for each param/return
- **Edge**: call site — cost matrix of save/restore overhead for each
  (caller-choice, callee-choice) pair
- **Objective**: minimise total register traffic (saves + restores + copies)
  across all call sites, weighted by call frequency

Payoff: eliminates `LD A, B; CALL foo; LD B, A` sequences where callee
could simply accept argument in B and return in B.

Gating: needs call graph (Phase 3d) + real programs (Phase 4) to measure
actual call frequencies.

### 5c — PBQP: SoA256 page assignment (memory layout)

Optimise which page each field column occupies, minimising `LD H, page`
overhead for frequently co-accessed fields (see ADR-0012).

- **Nodes**: field column arrays within one `LayoutSoA256` struct
- **Variables**: page offset (0, 1, 2, ...)
- **Edge cost**: `INC H` (4T) if pages consecutive, `LD H, imm` (10T) if not
- **Edge weight**: co-access frequency from profiling or static analysis
- **Objective**: minimise total `LD H, imm` instructions

Gating: needs SoA256 codegen (Phase 3c) + access frequency data (Phase 4).

### 5d — PBQP: Bank switching (128K targets)

For ZX Spectrum 128K / MegaROM targets: assign code/data blocks to ROM/RAM
banks to minimise bank-switch overhead (`RST 0x08` or `OUT ($7FFD)` sequences).

- **Nodes**: code blocks and data segments
- **Variables**: bank number (0–7 for Spectrum 128)
- **Edge**: inter-block call/reference — cost 0 if same bank, N T-states if not
- **Objective**: minimise total bank switches weighted by execution frequency

Gating: Phase 4 complete + 128K target backend.

---

## PBQP: When and where

| Application | Phase | Gating condition |
|-------------|-------|-----------------|
| Intra-function allocation | — | **Never needed** — 7 GP regs, greedy is optimal |
| Function contracts (5b) | 5 | call graph + real programs |
| SoA256 page assignment (5c) | 5 | SoA256 codegen + access data |
| EXX region assignment | 5a | EXX region detection (ADR-0013) |
| Bank switching (5d) | 5 | 128K backend |

Z80 has 7 GP registers. With 3–5 simultaneous live ranges (current examples),
greedy allocation is already optimal. PBQP for intra-function allocation would
add ~500 LOC for zero measurable gain until functions have ≥10 live ranges.
The real PBQP wins are **interprocedural** (contracts, pages, banks).

---

## Example coverage tracker

| Example     | Verified | T-states approx | Notes |
|-------------|----------|-----------------|-------|
| fibonacci   | yes      | ~300 (n=8)      | recursive |
| clamp       | yes      | ~40             | 3-arg, 2 branches |
| abs_diff    | yes      | ~20             | NEG trick |
| sum_range   | yes      | ~140 (n=10)     | loop + accumulate |
| mul8        | yes      | ~220 (6×7)      | shift-add loop |
| min8        | yes      | ~25             | baseline |
| gcd         | yes      | —               | TermBrIf2: one CP, three-way branch, ge block gone |
| max3        | yes      | —               | 3-arg comparison chain |
| popcount    | yes      | —               | AND+shift loop |
| rol8        | yes      | —               | (x<<1)\|(x>>7), genShift N-copy path |
| is_pow2     | yes      | —               | x&(x-1)==0 bit-trick, CP 0 peephole |
| swap_nibbles| yes      | —               | (x<<4)\|(x>>4), 4×SLA + 4×SRL |
| min16       | —        | —               | Phase 3 (needs SBC HL,DE in genCmp) |
| memcpy      | —        | —               | Phase 3c (OpPtrBump) |
| struct_init | —        | —               | Phase 3b |
| iter_filter | —        | —               | Phase 3c |

---

## When to write minz → MIR2?

When Phase 3 is complete and MIR2 produces correct Z80 for:
- at least 20 distinct example programs
- struct field access (AoS and SoA)
- multi-function modules with explicit calling convention

Until then, the old pipeline (`pkg/ir` + `pkg/codegen/z80.go`) stays production.
MIR2 is the sandbox where allocation and layout strategies are proven first.

---

## Key ADRs

| ADR | Decision |
|-----|----------|
| ADR-0011 | Shadow register guard (LocShadow = InfCost for standard classes) |
| ADR-0012 | Array layout strategy: AoS / SoA / SoA256 (H=field, L=index) |
| ADR-0013 | EXX region detection — planned for Phase 4 |
