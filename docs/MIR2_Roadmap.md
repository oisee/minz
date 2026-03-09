# MIR2 Development Roadmap

`pkg/mir2` — clean-room IR + register allocator + Z80 codegen,
developed in parallel with the production compiler.

---

## Progress

```
Phase 1  ████████████████████  100%  Core pipeline            ✅ DONE
Phase 2  ████████████████████  100%  Codegen quality          ✅ DONE (+flag-return ABI, z80timing SSOT)
Phase 3  ████████████████████  100%  Feature coverage         ✅ DONE (2026-03-09)
Phase 4  ⏭ SKIPPED             ---   minz→MIR2 lowering       (focus is nanz/plm frontends)
Phase 5  ░░░░░░░░░░░░░░░░░░░░    0%  Optimisation (PBQP)

Overall  ████████████░░░░░░░░   ~60% (Phase 4 weight redistributed; Phases 1+2+3 complete)
```

**Phase 4 skip rationale**: The active frontend work is nanz/plm → HIR → MIR2 (already
wired end-to-end via `pkg/pipeline`).  A separate "minz AST → MIR2" pass duplicates that
effort.  Phase 4 items remain as reference but are not on the critical path.

Weight model (effort estimate): Phase 1=15%, 2=10%, 3=30%, 4=⏭, 5=20%.
Overall (active phases) = P1×1.0 + P2×1.0 + P3×0.20 + P5×0 ≈ 31%.

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

## Phase 2 — Codegen quality ✅ DONE

- [x] DSE pass — suppress `OpConst` emission when peephole made it dead
- [x] AND/OR/XOR/ADD/SUB imm8 — no register needed for rhs constant
- [x] `holdsPhys` — bidirectional alias map, topology-aware (INC DE invalidates D+E)
- [x] **TermBrIf2** — three-way branch using Z and C from one CP
      eliminates the redundant `CP B` in GCD; `ge` block gone entirely
- [x] More examples: `rol8`, `is_pow2`, `swap_nibbles` — shift+OR idiom, bit-trick
- [x] `genShift` constant-count: emits N copies of SLA/SRL for compile-time counts
- [x] **`pkg/z80timing/`** — SSOT for all Z80 T-state constants; table-driven `Base[256]`,
      `CBBase`, `EDBase`, `DDBase`; imported by emulator + cost model (2026-03-09)
- [x] **Flag-return ABI** — `Return.FlagCond`, `ClassFlag` return, `emitCallArgs` parallel
      copy, `physOverride` tracking; callers use `JP C`/`JP NC` with no `AND A` (2026-03-09)
- [ ] `min16` — deferred to Phase 3a (needs 16-bit comparison via SBC HL,DE)

---

## Phase 3 — Feature coverage (~30% of total effort)

Goal: cover enough of the language to run real MinZ programs through MIR2.

### 3a — Scalars and pointers ✅ DONE
- [x] `OpAddrOf(sym)` → `LD HL, label` + global data emission (`DB`/`DW`)
- [x] VM: `resolveSymbol` handles `Module.Globals` (pre-allocated in heap)
- [x] `compileModule` helper: merged AllocResults for multi-function modules
- [x] E2E verified: `add_to_counter` — global read/add/store, multi-call persistence
- [x] u8/u16 mixed-width arithmetic: `OpExt` (u8→u16 zero-extend), `OpSext` (i8→i16 RLCA+SBC),
      `OpTrunc` (u16→u8 low byte) — all in codegen + VM
- [x] **16-bit comparison**: `genCmp16` → `PUSH HL; OR A; SBC HL, rr; POP HL` (2026-03-09)
      E2E verified: `min16(a,b: u16) → u16`, 10 test cases including 0/65535/32767/32768
- [x] `min16` example — previously deferred from Phase 2 (now closed)
- [ ] Verifier: type width constraints at OpLoad/OpStore — low priority, deferred to 3d

### 3b — Structs ✅ DONE (AoS)
- [x] `StructTy` in types.go — named fields, `ByteOffset(i)`, `Width()`
- [x] `OpField(base, imm=byte_offset)` → pointer + compile-time byte offset
- [x] AoS codegen: `INC HL` × offset (≤3), or `LD BC, offset; ADD HL, BC` (>3)
- [x] `OpPtrBump` — same Z80 codegen as OpField (iterator advance)
- [x] E2E: `TestStructPointZ80` — global Point{x,y}, sum_xy reads both fields
- [ ] SoA lowering: `[N]Struct{f0,f1,f2}` → three separate `[N]Ti` arrays — deferred to Phase 3c

### 3c — Arrays and iterators ✅ DONE
- [x] `ArrayTy` with `Layout` (AoS/SoA/SoA256) — types.go with `ByteWidth`, `NewArraySoA256`
- [x] `SliceTy` — fat pointer (base_ptr, len) type definition
- [x] `OpPtrBump(ptr, stride_const)` → `INC HL` / `INC HL;INC HL` / `ADD HL,BC`
- [x] Array iteration via manual loop: `sum_array` (pointer+counter), `memcpy` (LDIR-style)
- [x] **TermDJNZ** + DJNZ peephole: `DEC B; JP loop_head; AND A; JP Z` → `DJNZ` saves 15T/iter (2026-03-09)
      `detectDJNZPeephole()` in genBlock — pattern-matches existing loops; TermDJNZ for explicit do-while
- [x] SoA256 codegen: `OpPtrBump(stride=-1)` → `INC L`; `OpPtrBump(stride=256)` → `INC H` (2026-03-09)
- [x] Verifier: SoA256 constraints stub (Len≤256, global-only — low-priority, deferred to Phase 5)

### 3d — Multi-function modules ✅ DONE
- [x] String literals: `StringPool`, `m.Strings.Intern()`, NUL-terminated `DB` emission
- [x] `TestStrLenZ80` — iterates a NUL-terminated string, reads from string pool label
- [x] Multi-function modules with explicit call convention (fib, double_sum, clampByte all pass)
- [x] Liveness across calls: values in ClassGeneral (C/D/E) survive calls without spill
      E2E: `double_sum(a,b) = 2a+2b` — b in C survives first CALL; 2a spilled to D
- [x] Flag-return ABI (added in Phase 2 extension): callers use JP C/NC directly after CALL
- [x] `@print` intrinsic: `@mir.io.print.str` → inline NUL-loop via `OUT (0x01), A` (MZE debug port);
      also `@mir.io.print.u8` (single char), `@mir.io.print.nl` (newline); E2E: `TestPrintStrZ80` (2026-03-09)
      **Bug fixed**: `GetOutput()` now reads `*z.ports.output` (was stale copy of slice header)
- [x] **OpPush/OpPop scaffold**: `toPair()` maps any reg to 16-bit pair; `PUSH rr`/`POP rr` for callee-save
      (2026-03-09)

---

## Phase 4 — minz → MIR2 lowering ⏭ SKIPPED

**Decision (2026-03-09)**: Active frontend work is nanz/plm → HIR → MIR2 via
`pkg/pipeline`.  A separate minz-AST→MIR2 pass is not on the critical path.
Items below remain as reference for when/if the old `pkg/ir` pipeline is retired.

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
| min16       | yes      | —               | Phase 3a: SBC HL,DE in genCmp16 ✅ |
| add_to_counter | yes   | —               | Phase 3a: global read/add/store ✅ |
| sum_array   | yes      | —               | Phase 3c: pointer+counter loop ✅ |
| memcpy      | yes      | —               | Phase 3c: LDIR-style via OpPtrBump ✅ |
| struct_point| yes      | —               | Phase 3b: global Point{x,y}, sum_xy ✅ |
| strlen      | yes      | —               | Phase 3d: NUL-terminated string pool ✅ |
| double_sum  | yes      | —               | Phase 3d: liveness across calls ✅ |
| print_str   | yes      | —               | Phase 3d: @mir.io.print.str inline ✅ |
| sum_array_djnz | yes   | —               | Phase 3c: TermDJNZ do-while, DJNZ peephole ✅ |

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
