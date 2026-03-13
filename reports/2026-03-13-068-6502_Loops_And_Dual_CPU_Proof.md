# Report 068: 6502 Round-Trip Oracle — Dual-CPU Correctness Proof

**Date:** 2026-03-13
**Status:** 20/20 round-trip tests pass (5 paths agree), 32/32 standalone 6502 tests pass
**Builds on:** Report 067 (6502 E2E harness)

---

## Summary

The MOS 6502 backend is now wired as **path 6** in the cross-backend round-trip
oracle.  All 4 Nanz functions (abs_diff, fib, clamp, max3) produce **identical
results** across 5 independent compilation paths:

| Path | Pipeline | Status |
|------|----------|--------|
| 1: VM | MIR2 → mir2.VM.Call() | Pass |
| 2: C | MIR2 → mir2c → cc → native | Pass |
| 3: QBE | MIR2 → mir2qbe → qbe → cc → native | Pass |
| 4: QBE↔MIR2 | MIR2 → mir2qbe → qbe2mir2 → mir2c → cc → native | Pass |
| 6: 6502 | MIR2 → M6502Codegen → tinyAsm → sim6502 | Pass |

Path 5 (Z80) is show-only (assembly logged, not executed in this test).

This work uncovered and fixed **10 distinct codegen bugs** across two sessions,
all rooted in the 6502's 3-register constraint + implicit A clobber.

---

## What's New (Session 2)

### Round-Trip Oracle Integration

- Created `sim6502run` shared package (compile + execute 6502 from MIR2)
- Added `path6_6502` to `mir2c/roundtrip_test.go`
- Contract-aware bootstrap: loads args into correct registers per allocation
- All 20 test cases (4 functions × 5 cases each) pass on all 5 paths

### Test Results

```
✓ all 5 paths agree: abs_diff([10 3]) = 7
✓ all 5 paths agree: abs_diff([3 10]) = 7
✓ all 5 paths agree: abs_diff([5 5]) = 0
✓ all 5 paths agree: abs_diff([200 100]) = 100
✓ all 5 paths agree: abs_diff([0 255]) = 255
✓ all 5 paths agree: fib([0]) = 0
✓ all 5 paths agree: fib([1]) = 1
✓ all 5 paths agree: fib([5]) = 5
✓ all 5 paths agree: fib([8]) = 21
✓ all 5 paths agree: fib([10]) = 55
✓ all 5 paths agree: clamp([5 0 10]) = 5
✓ all 5 paths agree: clamp([0 5 10]) = 5
✓ all 5 paths agree: clamp([15 5 10]) = 10
✓ all 5 paths agree: clamp([5 5 5]) = 5
✓ all 5 paths agree: clamp([128 10 200]) = 128
✓ all 5 paths agree: max3([1 2 3]) = 3
✓ all 5 paths agree: max3([3 2 1]) = 3
✓ all 5 paths agree: max3([2 3 1]) = 3
✓ all 5 paths agree: max3([7 7 7]) = 7
✓ all 5 paths agree: max3([100 200 150]) = 200
```

---

## Root Cause Analysis: 10 Bugs in Two Sessions

All 10 bugs share a common root: the MOS 6502 has only 3 general-purpose
registers (A, X, Y) and ALU instructions implicitly use A as the accumulator.
The register allocator assigns locations globally per function, but codegen
routines clobber registers during instruction emission without the allocator's
knowledge.

### Taxonomy

| # | Category | Root Cause Class |
|---|----------|-----------------|
| 1-2, 6-7 | **Value clobber** | Instruction/terminator destroys a live register |
| 3 | **Address collision** | Spill slots overlap allocator's ZP range |
| 4 | **Missing instruction** | genMove lacked LDX/LDY cases |
| 5 | **Re-processing** | Two-pass spill didn't track completed entries |
| 8 | **Missing save scope** | preSaveBlockArgs didn't cover contract params |
| 9 | **Copy ordering** | Sequential copies clobbered A via LDA+STA routing |
| 10 | **Bootstrap mismatch** | Args loaded positionally, not per allocation |

---

### Session 1 Bugs (Standalone E2E)

#### Bug 1: Register clobber in block-arg passing

**Symptom:** `gcd(100,75)` returned 50 instead of 25.

**Root cause:** `emitBlockArgs` trusted `loc(reg)` to reflect actual register
contents, but ALU ops (ADC/SBC) had already clobbered A.  The allocator
assigns `aP→A` globally, but `genSub` uses A as accumulator, destroying aP.

**RCA class:** Value clobber — ALU implicit A use.

**Fix:** `preSaveBlockArgs` — at block entry, save block params in A/X/Y to
ZP backup slots ($F0+).  `loc()` checks backups first; `rawLoc()` returns
allocator's original assignment (for destinations that MUST be in the real
register for the next block's preSaveBlockArgs).

#### Bug 2: BrIf2 missing block-arg trampolines

**Symptom:** `fib(n)` returned `n` instead of the Fibonacci number.

**Root cause:** `genThreeWayBranch` emitted `BEQ .done` / `BCC .body` /
`JMP .done` but never called `emitBlockArgs` for `EqArgs`/`LtArgs`/`GtArgs`.
The `done` block expected `result` in A, but got whatever was in A after
the comparison (iP, not aP).

**RCA class:** Value clobber — terminator didn't set up block args.

**Fix:** Emit trampoline blocks for any BrIf2 target with block args:
```asm
    BEQ .eq_trampoline
    BCC .body
    JMP .gt_trampoline
.eq_trampoline:
    LDA zp2          ; emitBlockArgs: aP → result(A)
    JMP .done
```

#### Bug 3: Spill slot / allocator ZP collision

**Symptom:** `fib(1)` returned 0 instead of 1.

**Root cause:** `emitBlockArgs` cycle-resolution spilled to $01, $02, $03...
which are the same addresses as the allocator's `zp2`=$02, `zp3`=$03.
Writing to $02 during spill destroyed `zp2` (aP).

**RCA class:** Address collision — two subsystems claiming same ZP range.

**Fix:** Moved spill scratch to $D0+ range, well separated from allocator
ZP ($02+), backup slots ($F0+), and ALU scratch ($00).

#### Bug 4: genMove X/Y from ZP broken

**Symptom:** `STA X` — invalid 6502 instruction.

**Root cause:** `genMove(X, $F0)` fell through to the default `LDA+STA` path,
producing `LDA $F0; STA X`.  But `STA X` isn't valid — X is a register.

**RCA class:** Missing instruction — genMove didn't handle all register combos.

**Fix:** Added `LDX`/`LDY`/`STX`/`STY` cases in genMove's default branch.

#### Bug 5: Spill pass re-processing

**Symptom:** Subtle corruption when 4+ copies needed cycle resolution.

**Root cause:** Two-pass spill (registers first, then ZP) re-processed
entries already saved in pass 0 because the source had been rewritten to
a scratch slot name (not A/X/Y), making it look like a ZP source.

**RCA class:** Re-processing — no tracking of completed entries.

**Fix:** Added `spilled[]` bool array to track processed entries.

---

### Session 2 Bugs (Round-Trip Oracle)

#### Bug 6: Allocator assigned flags register "P"

**Symptom:** `STA P` — assembler error, "P" not a valid address.

**Root cause:** `M6502PhysLocs` includes `{Kind: LocFlag, Name: "P"}` for
ClassFlag comparison results.  The allocator correctly returns InfCost for
ClassGeneral→LocFlag, but ClassFlag values (OpCmp results) DO get allocated
to "P".  The codegen's `genCmpBool` then tries to `STA P` to materialize
the boolean, which is not a valid 6502 instruction.

**RCA class:** Missing instruction — codegen can't store to flags register.
Deeper issue: cost table exposes locations the codegen can't handle.

**Fix:** Created `M6502CodegenCostTable` wrapper that filters out LocFlag,
LocStack, and absolute memory (Offset >= 0x100) from the allocator's location
list.  Forces comparison results into real registers where boolean
materialization (LDA #0/LDA #1) works.

#### Bug 7: Allocator spill slot name "mem" not parseable

**Symptom:** `STA mem` — assembler error, "mem" not a valid address.

**Root cause:** When the allocator spills to memory (all compatible locations
taken), it creates `PhysLoc{Kind: LocMem, Name: "mem", Offset: 0xF000+n}`.
The codegen's `loc()` returned `loc.Name` ("mem") instead of the hex address.

**RCA class:** Missing translation — codegen used symbolic name instead of
numeric address for memory locations.

**Fix:** Added `physLocTo6502()` that returns `$XX` hex format for LocMem
(using `loc.Offset`) and register names for LocReg.  Applied to both `loc()`
and `rawLoc()`.

#### Bug 8: Function contract params not saved on entry block

**Symptom:** `abs_diff(10,3)` returned 0 — params clobbered by OpCmp.

**Root cause:** `preSaveBlockArgs` saved block params (`b.Params`) at block
entry, but Nanz-compiled functions define parameters via `F.Contract.Params`
on the entry block — NOT as `Block.Params`.  The entry block's `b.Params`
was empty, so no values were saved.  OpCmp's `genCmpBool` (LDA #0/#1)
clobbered A (param `a`), and `genCondBranch` test (LDX for BNE) clobbered X
(param `b`).

The distinction: hand-built test MIR2 (m6502e2e_test.go) uses `BlockParam()`
which adds to `b.Params`.  Nanz-compiled MIR2 (via hir.LowerModule) uses
`bld.Param()` which adds to `F.Contract.Params`.

**RCA class:** Missing save scope — preSaveBlockArgs didn't know about
contract params.

**Fix:** Added entry-block detection: if `b == cg.curFunc.Blocks[0]`, also
iterate `cg.curFunc.Contract.Params` and save each to a ZP backup slot.

#### Bug 9: TermBrIf missing block-arg trampolines

**Symptom:** `fib(n)` returned 0 for n >= 1 — exit block got wrong value.

**Root cause:** `genCondBranch` (TermBrIf handler) emitted `BNE .then` /
`JMP .else` but completely ignored `ThenArgs` and `ElseArgs`.  The loop exit
block expected `a` (the Fibonacci result) as a block arg, but received
whatever happened to be in A after the comparison (the boolean 0/1).

This is the TermBrIf counterpart of Bug 2 (which was TermBrIf2).

**RCA class:** Value clobber — terminator didn't set up block args.

**Fix:** Added trampoline blocks to `genCondBranch`, same pattern as
`genThreeWayBranch`.

#### Bug 10: Bootstrap arg/register mismatch + copy ordering

**Symptom:** `clamp(5,0,10)` returned 10 instead of 5; `fib(8)` returned 8.

**Root cause (bootstrap):** `genBoot` loaded args positionally
(args[0]→A, args[1]→X, args[2]→Y), but the allocator assigns params to
arbitrary registers.  For `clamp`, the allocator assigned lo→Y, hi→X, but
the bootstrap put lo(=0)→X, hi(=10)→Y — swapped.

**Root cause (copy ordering):** `emitBlockArgs` emitted copies sequentially
in declaration order.  When a ZP destination (e.g., `n → $02`) followed a
register destination (e.g., `a → A`), the ZP copy used `LDA scratch; STA $02`
which clobbered A after it had been correctly set.

**RCA class:** Bootstrap mismatch + copy ordering conflict.

**Fix (bootstrap):** Rewrote `sim6502run.Run` to read `F.Contract.Params`
and load each arg into its allocated register/ZP location based on the
actual `AllocResult`.

**Fix (copy ordering):** Sort copies so ZP destinations come first (they
need LDA+STA which clobbers A), then Y, X, and finally A.  Applied to both
cycle and non-cycle paths in `emitBlockArgs`.

---

## Common Root Cause: The A-Clobber Problem

All 10 bugs stem from a single architectural constraint:

> **On the 6502, the accumulator (A) is implicitly used by every ALU and
> memory instruction, but the register allocator treats it as a general-
> purpose location that persists across instructions.**

This creates a fundamental impedance mismatch between the allocator's model
(A holds a value for the entire live range) and the codegen's reality (A is
destroyed by every arithmetic, comparison, and memory-routing operation).

### The Solution Pattern: Save-at-Entry + Ordered-Copies

1. **preSaveBlockArgs** — At block entry, save all register values that could
   be clobbered by the block's instructions or terminator.  Saves go to ZP
   backup slots ($F0+).  `loc()` returns backup locations; `rawLoc()` returns
   the real register assignment.

2. **Trampolines** — Every terminator with block args (TermJmp, TermBrIf,
   TermBrIf2, TermCondRet) emits intermediate blocks that call
   `emitBlockArgs` before jumping to the target.

3. **Ordered copies** — `emitBlockArgs` sorts copies so ZP destinations
   (which route through A) execute before register destinations.  A is
   always set last.

4. **Cycle resolution** — When sources and destinations overlap, all sources
   are spilled to $D0+ scratch slots first, then loaded in ZP-first order.

---

## 6502 ZP Memory Map (Codegen)

```
$00      : ALU scratch (zpScratch) — CMP/ADC/SBC operand spill
$02-$0F  : Allocator ZP slots (zp2, zp3, ...) — virtual registers
$D0-$DF  : emitBlockArgs cycle-resolution spill
$F0-$FF  : Block param backup slots (preSaveBlockArgs)
$F000+   : Allocator spill overflow (absolute addressing)
```

---

## Architecture: What Makes This Work

```
                    ┌─── Path 1: MIR2 VM ──────── reference result
                    │
                    ├─── Path 2: mir2c → cc ───── native binary
                    │
  Nanz → HIR → MIR2 ├─── Path 3: mir2qbe → qbe ── native binary
                    │
                    ├─── Path 4: QBE↔MIR2→C ──── double-translation
                    │
                    ├─── Path 5: Z80Codegen ───── assembly (show only)
                    │
                    └─── Path 6: M6502Codegen ─── sim6502 emulator
```

Every function is compiled to ALL available paths and the results must agree.
A bug in any backend is caught immediately by disagreement with the VM
reference oracle.

---

## Gap Analysis vs Z80

| Feature | Z80 | 6502 | Gap |
|---------|-----|------|-----|
| u8 arithmetic | ✓ | ✓ | — |
| u16 arithmetic | ✓ | ✗ | Need ADC/SBC with carry chain |
| Branching (if/else) | ✓ | ✓ | — |
| Loops (while/for) | ✓ | ✓ | — |
| Function calls | ✓ | ✓ (JSR/RTS) | — |
| Memory access (load/store) | ✓ | ✗ | Need LDA/STA abs, (zp),Y |
| Structs | ✓ | ✗ | Need pointer offset modes |
| Arrays | ✓ | ✗ | Need indexed addressing |
| Peephole optimization | 67 patterns | 0 | Future work |
| Register allocation | PBQP + coalescing | PBQP (filtered locs) | Adequate for u8 |
| Console I/O | ✓ | ✓ (4 platforms) | — |
| SMC | ✓ (production) | ✗ (proposed in paper) | Natural fit for ZP |
| **Round-trip oracle** | **show only** | **execute + assert** | — |

---

## Next Steps

1. ~~Wire 6502 into round-trip oracle~~ **DONE**
2. **Write paper section** on dual-CPU correctness evidence
3. **Add u16 support** (ADC/SBC with carry, 16-bit ZP pairs)
4. **Add memory access** (LDA/STA absolute, indexed modes)
5. **Peephole pass** — eliminate redundant LDA/STA sequences
