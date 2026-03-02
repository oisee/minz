# MinZ Iterator Implementation Status

*Updated: 2026-03-02 — v0.19.5 (post register allocator overhaul)*

## E2E Verified (MZX --console-io)

Full pipeline: MinZ source -> parser -> semantic -> MIR optimizer -> fusion optimizer -> Z80 codegen -> asm peephole -> MZA -> MZX emulator.
All 11 tests **PASS** with hex-verified output.

| Test | Chain | Expected | Hex | Status |
|------|-------|----------|-----|--------|
| `iter_foreach` | `arr.forEach(console_log)` | `ABCDE\n` | `41424344450a` | **PASS** |
| `iter_take` | `arr.iter().take(3).forEach(console_log)` | `ABC\n` | `4142430a` | **PASS** |
| `iter_skip` | `arr.iter().skip(2).forEach(console_log)` | `CDE\n` | `4344450a` | **PASS** |
| `iter_map_foreach` | `arr.iter().map(double).forEach(console_log)` | doubled bytes | `020406080a0a` | **PASS** |
| `iter_filter_foreach` | `arr.filter(is_big).forEach(console_log)` | `DE\n` | `44450a` | **PASS** |
| `iter_inline_filter` | `arr.filter(\|x\| x > 67).forEach(console_log)` | `DE\n` | `44450a` | **PASS** |
| `iter_lambda_map` | `arr.iter().map(\|x\| x+1).forEach(console_log)` | `BCDEF\n` | `42434445460a` | **PASS** |
| `iter_map_filter_foreach` | `arr.map(double).filter(\|x\|x>5).forEach(log)` | `6,8,10,\n` | `06080a0a` | **PASS** |
| `iter_filter_map_foreach` | `arr.filter(\|x\|x>=67).map(add_one).forEach(log)` | `DEF\n` | `4445460a` | **PASS** |
| `iter_take_map_foreach` | `arr.take(3).map(add_one).forEach(log)` | `BCD\n` | `4243440a` | **PASS** |
| `iter_bare_djnz` | `arr.filter(\|x\| x > 3).forEach(console_log)` | `\x04\x05\n` | `04050a` | **PASS** |

Run: `bash examples/e2e_iterators/run_e2e.sh`

---

## Performance After Register Allocator Overhaul

Measured via exact T-state counting in the Z80 emulator (E2E harness, `regalloc_quality_test.go`).

| Pattern | Before (v0.19.4) | After (v0.19.5) | Ideal | Speedup |
|---------|:-----------------:|:----------------:|:-----:|:-------:|
| `forEach(call)` | ~207T/elem | **26T/elem** | ~43T | **7.8x** |
| `map(fn).forEach(call)` | ~280T/elem | **36T/elem** | ~60T | **~7.6x** |
| `filter(\|x\|x>N).forEach(call)` | ~280T/elem | **26T/elem** | ~30T | **~10x** |

The per-element costs are now **below the ideal hand-written target** from the original plan (64T).

### What the overhaul changed (6 phases)

1. **Hint-aware allocation** — allocator respects `RegHintB` (counter), `RegHintHL` (pointer), `RegHintA` (element)
2. **Dynamic A/B tracking** — `loadToA()`/`loadToB()` skip loads when the register already has the value
3. **Memory round-trip elimination** — POP restores tracking, void returns don't store, INC HL stays tracked
4. **DJNZ with BC save/restore** — PUSH BC/POP BC around calls, enabling bare DJNZ
5. **Call-clobber-aware invalidation** — per-function `ModifiedRegisters` bitmask; only clear what callee actually clobbers
6. **Cycle-count regression tests** — 3 deterministic tests with thresholds

See [Register Allocator Overhaul Report](../reports/2026-03-02-018-Register_Allocator_Overhaul_Results.md) for full before/after assembly and pipeline walkthrough.

---

## What the Compiler Produces (Post-Overhaul)

### forEach — the core loop

```minz
let arr: [u8; 5] = [65, 66, 67, 68, 69];
arr.forEach(console_log);
```

The dynamic register tracking eliminates most redundant loads/stores:
- After `POP HL`, codegen knows HL contains the pointer (no reload for INC)
- After `LD A, (HL)`, codegen knows A has the element (no reload for CALL arg)
- Void return from `console_log` skips `storeFromHL`
- Counter tracked across PUSH/POP — fewer memory round-trips

**Measured: 132T total for 5 elements = 26T/element**

### map+forEach — fusion inlining

```minz
arr.map(add_one).forEach(console_log);
```

- `add_one` is **fused** into the loop body (pure function, 1 param, ≤ 8 instructions)
- `console_log` still requires CALL (has `OpAsm` — physical register dependency)
- Fusion eliminates CALL/RET overhead (~27T) and SMC parameter patching (~13T) for `add_one`

**Measured: 184T total for 5 elements = 36T/element**

### filter+forEach — inline predicate

```minz
arr.filter(|x| x > 66).forEach(console_log);
```

- `|x| x > 66` compiled as inline `CP 67; JR C, continue` — no function call
- `isSimpleComparisonLambda()` detects `x > N` and applies `CP N+1, JR C`
- Continue label placed before POP/INC/DJNZ tail — filtered elements restore stack correctly

**Measured: 132T total for 5 elements = 26T/element**

### What IS genuinely zero-cost

- `take(n)` counter folding — `LD B, n` instead of `LD B, array.length`
- `skip(n)` pointer offset — `ADD HL, DE` at init, no skip loop
- Inline filter lambda — `CP N` + `JR C` instead of CALL + OR A + JR Z
- Fusion inlining — eliminates CALL/RET for simple callbacks
- `ADD A, A` strength reduction — `x * 2` is 4T instead of 8T (SLA A)

---

## Operation Status

| Operation | Parser | Semantic | Z80 | E2E | Notes |
|-----------|:------:|:--------:|:---:|:---:|-------|
| `forEach(fn)` | pass | pass | correct | **PASS** | PUSH/POP HL+BC around CALL |
| `take(n)` | pass | pass | correct | **PASS** | Counter folded to `LD B, n` |
| `skip(n)` | pass | pass | correct | **PASS** | Pointer offset + counter adjust |
| `map(fn)` | pass | pass | correct | **PASS** | Fusion inlines simple fns |
| `filter(fn)` | pass | pass | correct | **PASS** | Named predicate + forEach |
| `filter(\|x\| x>N)` | pass | pass | correct | **PASS** | Inline `CP N+1` + `JR C` |
| `map(\|x\| x+N)` | pass | pass | correct | **PASS** | Lambda inlined |
| `map+filter+forEach` | pass | pass | correct | **PASS** | Multi-stage chain |
| `filter+map+forEach` | pass | pass | correct | **PASS** | Multi-stage chain |
| `take+map+forEach` | pass | pass | correct | **PASS** | Multi-stage chain |
| `peek(fn)` | pass | pass | compiles | untested | Same path as forEach |
| `inspect(fn)` | pass | pass | compiles | untested | Alias for peek |
| `takeWhile(fn)` | pass | pass | compiles | untested | **Stack hazard** — early exit after PUSH |
| `enumerate()` | pass | pass | compiles | **BROKEN** | B used for both counter and index |
| `reduce(fn)` | pass | pass | compiles | **BROKEN** | Two `LD A` — second overwrites first |
| `skipWhile(fn)` | pass | pass | N/A | N/A | Not in DJNZ mode |

---

## Known Bugs

### CRITICAL: enumerate uses B for counter AND index

```asm
    LD B, 5           ; B = loop counter
.loop:
    LD A, B
    INC A
    LD B, A           ; B = index (now 6?!)
    ; ...
    DEC B             ; decrement... what? counter? index?
    JR NZ, .loop      ; near-infinite loop
```

B cannot serve two purposes. Needs separate index register (C or memory-backed).

### CRITICAL: reduce overwrites A with second param

```asm
sum_u8_u8:
    LD A, 0           ; SMC: acc (will be patched)
    LD A, 0           ; SMC: x — OVERWRITES acc!
    ADD HL, DE        ; 16-bit add on unloaded registers
    RET
```

Two TRUE SMC loads both target A. Second wipes first.

### takeWhile stack hazard

If predicate fails and loop exits early, PUSH HL has no matching POP. Stack
corrupted by 2 bytes per iteration until exit. Needs `POP HL` before exit jump.

### Dead code: inlined functions still emitted

When fusion optimizer inlines a callback (e.g., `double`), the function definition
is still emitted in the output. Wastes bytes but doesn't affect correctness.

### Orphan PUSH/POP after fusion

Semantic layer emits PUSH/POP HL based on `hasCallOps` at IR time. Fusion optimizer
removes CALLs *after* PUSH/POP are already in the instruction stream. Result: PUSH/POP
with no CALL between them — pure waste (21T/iteration).

---

## Remaining Optimization Opportunities

### Priority 1: Fix enumerate and reduce (Bug G)
- Separate loop counter from enumerate index (different registers)
- Fix reduce: two SMC params can't both target A
- Add E2E tests for both

### Priority 2: Strip orphan PUSH/POP after fusion
- Fusion optimizer should remove PUSH/POP when it eliminates all CALLs
- Enables true bare DJNZ path (currently unreachable in E2E tests)

### Priority 3: Fix takeWhile stack hazard
- Emit `POP HL` before early-exit jump

### Priority 4: Trust physical allocations fully
- The allocator assigns registers but codegen still stores to `$F0xx` defensively
- When pointer IS in HL, don't store to `$F014` after every use
- When counter IS in B, don't round-trip through `$F012`

### Priority 5: 8-bit arithmetic for u8 values
- Fused `add_one` currently uses 16-bit `ADD HL, DE` instead of `INC A`
- Type-aware codegen should use 8-bit paths for u8

### Priority 6: Dead code elimination for inlined functions
- After fusion inlines a function, mark it unused if no other callers
- Remove from output

---

## Architecture

### Compilation Pipeline
```
Source -> Parser -> Semantic -> MIR -> Optimizer -> Z80 Codegen -> Asm Peephole -> MZA
           |          |          |        |              |
           |          |          |        |              +-- OpDJNZ -> DEC B + JR NZ
           |          |          |        |                  (or bare DJNZ if no CALL)
           |          |          |        +-- Fusion: inline callbacks in DJNZ loops
           |          |          |        +-- DCE, copy prop, constant fold
           |          |          +-- OpLoad, OpCall, OpDJNZ, OpJumpIfFlag
           |          +-- generateDJNZIteration(): counter + pointer + PUSH/POP
           +-- tryConvertIteratorChain(): method chain -> IteratorChainExpr
```

### Key Files

| File | Role |
|------|------|
| `pkg/ast/iterator.go` | IteratorChainExpr, IteratorOp (Function + Argument fields) |
| `pkg/parser/participle/convert.go` | `tryConvertIteratorChain()` — CallExpr -> IteratorChainExpr |
| `pkg/semantic/iterator.go` | Type checking, DJNZ/indexed dispatch, lambda extraction |
| `pkg/semantic/iterator_enhanced.go` | Enhanced DJNZ with take/skip/enumerate/reduce |
| `pkg/codegen/z80.go` | Z80 codegen + register tracking (~5500 lines) |
| `pkg/codegen/register_allocator.go` | Hint-aware linear scan allocation (~500 lines) |
| `pkg/codegen/register_allocator_test.go` | Allocation unit tests |
| `pkg/optimizer/fusion.go` | Fusion optimizer — DJNZ loop detection + callback inlining |
| `pkg/optimizer/assembly_peephole.go` | Asm peephole (pattern 48: DEC B + JR NZ -> DJNZ) |
| `pkg/optimizer/dead_code_elimination.go` | DCE — fixed OpJumpIfFlag/OpPush marking |
| `pkg/z80testing/regalloc_quality_test.go` | Cycle-count regression tests |

### Test Suite (90+ tests across 8 layers)

| Package | Tests | What |
|---------|-------|------|
| `examples/e2e_iterators/` | 11 | Full pipeline E2E: compile -> assemble -> emulate -> hex verify |
| `tests/iterator_corpus/` | 18 | Full pipeline regression — all 18 compile to Z80 |
| `pkg/optimizer/` | 19 | Peephole, DCE, superoptimizer, normalization, fusion (7) |
| `pkg/parser/participle/` | 20 | Chain conversion, argument routing |
| `pkg/semantic/` | 17 | DJNZ eligibility, filter/map/lambda IR |
| `pkg/mirvm/` | 8 | DJNZ execution at MIR level |
| `pkg/codegen/` | 5+ | Z80 assembly patterns + register allocator tests |
| `pkg/z80testing/` | 3 | Cycle-count regression tests (deterministic) |

---

## Bugs Fixed (Historical)

| Version | Bug | Fix |
|---------|-----|-----|
| v0.19.4 | HL clobbered by CALL return value | PUSH/POP HL around CALL blocks |
| v0.19.4 | TRUE SMC anchor: `LD A,(nn)` instead of `LD A,N` | Changed to opcode 3E (patchable immediate) |
| v0.19.4 | MIR peephole read `inst.Value` not `inst.Imm` | Fixed `trackConstants()` to use `inst.Imm` |
| v0.19.4 | `prepareCallArguments()` skipped all SMC | Changed to only skip TRUE SMC (`UsesTrueSMC`) |
| v0.19.4 | Lambda `NumParams=0` after extraction | Use `AddParamWithRegister()` |
| v0.19.4 | Inliner assumed params at regs 1,2,... | Use `fn.Params[i].Reg` |
| v0.19.4 | Copy propagation across labels | Clear copy map at every `OpLabel` |
| v0.19.5 | DCE removed filter constant (`OpJumpIfFlag`) | Added OpJumpIfFlag to Phase 1 marking |
| v0.19.5 | BareDJNZ hint + peephole pattern 48 | `DEC B; JR NZ` -> `DJNZ` when no CALL |
| v0.19.5 | findSymbol non-determinism | Prefer shortest match (function entry, not sub-labels) |
| v0.19.5 | Register allocator overhaul (6 phases) | Hint-aware, dynamic tracking, call-clobber-aware |

---

## References

- [Register Allocator Overhaul Report](../reports/2026-03-02-018-Register_Allocator_Overhaul_Results.md) — full pipeline walkthrough with assembly
- [Iterator Reality Check](../reports/2026-03-02-017-Iterator_Reality_Check.md) — pre-overhaul honest assessment
- [ADR-0008: Flag-Based Boolean ABI](adr/0008-flag-based-boolean-abi-for-iterators.md)
- [ADR-0009: Superoptimizer Peephole Rules](adr/ADR-0009-superoptimizer-peephole-rules.md)
- [GenPlan](GenPlan.md) — canonical development roadmap
- [Archived docs](_archive/) — old iterator design docs and reports
