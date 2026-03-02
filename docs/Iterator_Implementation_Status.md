# MinZ Iterator Implementation Status

*Updated: 2026-03-02 — v0.19.5 (grounded in actual compiler output)*

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

## What the Compiler ACTUALLY Produces

All assembly below is **real compiler output** from v0.19.5, not idealized.

### forEach — the core loop (actual)

```minz
let arr: [u8; 5] = [65, 66, 67, 68, 69];
arr.forEach(console_log);
```

**Actual Z80 output** (loop body, annotated with T-states):
```asm
    LD HL, ($F014)    ; 16T  load pointer from memory (virtual reg 10)
    LD A, (HL)        ;  7T  load element
    LD ($F016), A     ; 13T  store element to memory (virtual reg 11)
    PUSH HL           ; 11T  save pointer (CALL will clobber HL)
    LD A, ($F016)     ; 13T  reload element from memory (!!)
    CALL console_log  ; 17T  the actual work
    LD ($F018), HL    ; 16T  store return value (void — pure waste)
    POP HL            ; 10T  restore pointer
    LD ($F014), HL    ; 16T  store pointer to memory
    LD HL, ($F014)    ; 16T  reload pointer from memory (!!)
    INC HL            ;  6T  advance pointer
    LD ($F014), HL    ; 16T  store advanced pointer
    LD A, ($F012)     ; 13T  load counter from memory
    LD B, A           ;  4T
    DEC B             ;  4T
    LD A, B           ;  4T
    LD ($F012), A     ; 13T  store counter to memory
    JR NZ, loop       ; 12T
                      ; ----
                      ; ~207T per element
```

**Ideal hand-written Z80** for the same operation:
```asm
.loop:
    LD A, (HL)          ;  7T  load element
    CALL console_log    ; 17T
    INC HL              ;  6T  advance
    DJNZ .loop          ; 13T
                        ; ----
                        ; ~43T per element
```

**Reality: ~207T actual vs ~43T ideal = 4.8x overhead.**

The bottleneck is **memory-backed virtual registers** — every virtual reg lives at
`$F0xx` absolute addresses. Of the ~207T per iteration, ~132T (64%) is memory
load/store for virtual registers that could live in physical Z80 registers.

### map(double).filter(|x|x>5).forEach(console_log) — multi-stage chain (actual)

```minz
fun double(x: u8) -> u8 { return x * 2; }
let arr: [u8; 5] = [1, 2, 3, 4, 5];
arr.map(double).filter(|x| x > 5).forEach(console_log);
```

**Actual Z80 output** (loop body):
```asm
    LD HL, ($F014)    ; load pointer from memory
    LD A, (HL)        ; load element
    LD ($F016), A     ; store element to memory
    PUSH HL           ; save pointer
    ; --- inlined double() body (fusion optimizer) ---
    LD A, ($F016)     ; reload element
    ADD A, A          ; x * 2 (optimized from SLA A)
    LD H, 0
    LD L, A
    LD ($F006), HL    ; store doubled value
    ; --- end inlined double ---
    LD HL, ($F006)    ; reload doubled value
    LD ($F018), HL    ; store to another virtual reg
    ; --- inline filter: x > 5 ---
    LD A, 6
    LD ($F01A), A     ; store filter constant (waste)
    LD A, ($F006)     ; reload doubled value
    CP 6              ; compare (correct!)
    JR C, filter_continue  ; skip if < 6
    ; --- forEach: console_log ---
    LD A, ($F018)     ; reload doubled value (3rd time!)
    CALL console_log
    LD ($F01C), HL    ; store void return (waste)
filter_continue:
    POP HL            ; restore pointer
    LD ($F014), HL    ; store pointer
    LD HL, ($F014)    ; reload pointer (!!)
    INC HL
    LD ($F014), HL    ; store advanced pointer
    ; --- counter (memory-backed, not DJNZ) ---
    LD A, ($F012)
    LD B, A
    DEC B
    LD A, B
    LD ($F012), A
    JR NZ, loop
```

**What the fusion optimizer achieved:** `double()` is inlined — no CALL to double,
no TRUE SMC parameter patching. Saves ~27T per element.

**What it didn't achieve:** `console_log` has `OpAsm` (physical register dependency),
so it can't be inlined. PUSH/POP HL still needed. Counter still memory-backed.

**The `double_u8` function is still emitted as dead code** in the output (lines 89-114).
Fusion inlines the body but doesn't remove the now-unreferenced function definition.

### TRUE SMC pattern (actual vs doc)

The **actual** compiler emits correct SMC:
```asm
double_u8_param_x_immOP:
    LD A, 0        ; opcode 3E 00 — the 00 byte is patched at runtime
double_u8_param_x_imm0 EQU double_u8_param_x_immOP+1  ; points to the patchable byte
```

Old docs incorrectly showed `LD A, (x_imm0)` — opcode 3A (indirect load from address).
That's NOT SMC, that's a regular memory read (13T, 3 bytes). Real SMC is `LD A, N`
(opcode 3E, 7T, 2 bytes) where N is patched in-place.

### take(3) — counter folding (actual, works correctly)

```asm
    LD B, 3       ; Counter = take(3), not array.length(5)!
```

This part is genuinely zero-cost. `take(n)` compiles to a different constant in `LD B`.

### skip(2) — pointer offset (actual, works correctly)

```asm
    LD HL, 2          ; offset constant
    ADD HL, DE        ; advance pointer past skipped elements
```

Also genuinely zero-cost. Pointer adjusted at loop init, counter adjusted to 3.

---

## Operation Status

| Operation | Parser | Semantic | Z80 | E2E | Notes |
|-----------|:------:|:--------:|:---:|:---:|-------|
| `forEach(fn)` | pass | pass | correct | **PASS** | PUSH/POP HL around CALL |
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

Two TRUE SMC loads both target A. Second wipes first. Also uses 16-bit ADD HL,DE
for u8 math — wrong register pair, values never loaded into HL/DE.

### takeWhile stack hazard

If predicate fails and loop exits early, PUSH HL has no matching POP. Stack
corrupted by 2 bytes per iteration until exit. Needs `POP HL` before exit jump.

### Performance: memory-backed registers

The register allocator puts ALL virtual registers through absolute memory ($F0xx).
This is the dominant overhead — 64% of loop T-states are memory load/store.

Redundant patterns that appear in EVERY loop:
```asm
    LD ($F016), A     ; store element
    ; ... later ...
    LD A, ($F016)     ; reload same element (no intervening write!)

    POP HL            ; restore pointer
    LD ($F014), HL    ; store to memory
    LD HL, ($F014)    ; reload from memory (!!)
```

### Dead code: inlined functions still emitted

When fusion optimizer inlines a callback (e.g., `double`), the function definition
is still emitted in the output. Wastes bytes but doesn't affect correctness.

### Orphan PUSH/POP after fusion

Semantic layer emits PUSH/POP HL based on `hasCallOps` at IR time. Fusion optimizer
removes CALLs *after* PUSH/POP are already in the instruction stream. Result: PUSH/POP
with no CALL between them — pure waste (21T/iteration).

---

## Performance Reality

| Pattern | Actual T/elem | Ideal T/elem | Overhead |
|---------|:------------:|:------------:|:--------:|
| `forEach(fn)` | ~207 | ~43 | 4.8x |
| `map(fn).filter.forEach` | ~280 | ~60 | 4.7x |
| `filter(\|x\|x>N).forEach` | ~207 | ~30 | 6.9x |
| `take(3).forEach` | ~207 | ~43 | 4.8x (but only 3 iters) |

The overhead is almost entirely memory-backed virtual registers. The iterator
pipeline itself (DJNZ pattern, fusion, inline filter) is architecturally sound.
The register allocator is the bottleneck.

### What IS genuinely zero-cost

- `take(n)` counter folding — `LD B, n` instead of `LD B, array.length`
- `skip(n)` pointer offset — `ADD HL, DE` at init, no skip loop
- Inline filter lambda — `CP N` + `JR C` instead of CALL + OR A + JR Z (~27T saved)
- Fusion inlining — eliminates CALL/RET for simple callbacks (~17T saved)
- `ADD A, A` strength reduction — `x * 2` is 4T instead of 8T (SLA A)

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
| `pkg/codegen/z80.go` | Z80 register allocation + assembly emission (~5500 lines) |
| `pkg/optimizer/fusion.go` | Fusion optimizer — DJNZ loop detection + callback inlining |
| `pkg/optimizer/fusion_test.go` | 7 unit tests for fusion optimizer |
| `pkg/optimizer/assembly_peephole.go` | Asm peephole (pattern 48: DEC B + JR NZ -> DJNZ) |
| `pkg/optimizer/dead_code_elimination.go` | DCE — fixed OpJumpIfFlag/OpPush marking |

### Test Suite (87+ tests across 7 layers)

| Package | Tests | What |
|---------|-------|------|
| `examples/e2e_iterators/` | 11 | Full pipeline E2E: compile -> assemble -> emulate -> hex verify |
| `tests/iterator_corpus/` | 18 | Full pipeline regression — all 18 compile to Z80 |
| `pkg/optimizer/` | 19 | Peephole, DCE, superoptimizer, normalization, fusion (7) |
| `pkg/parser/participle/` | 20 | Chain conversion, argument routing |
| `pkg/semantic/` | 17 | DJNZ eligibility, filter/map/lambda IR |
| `pkg/mirvm/` | 8 | DJNZ execution at MIR level |
| `pkg/codegen/` | 5 | Z80 assembly patterns (use `-vet=off`) |

---

## What Needs to Happen Next

### Priority 1: Fix enumerate and reduce (Bug G)
- Separate loop counter from enumerate index (different registers)
- Fix reduce: two SMC params can't both target A
- Add E2E tests for both

### Priority 2: Strip orphan PUSH/POP after fusion
- Fusion optimizer should remove PUSH/POP when it eliminates all CALLs
- Enables true bare DJNZ path (currently unreachable in E2E tests)

### Priority 3: Fix takeWhile stack hazard
- Emit `POP HL` before early-exit jump

### Priority 4: Reduce memory traffic (register allocator)
- The 4.8x overhead is almost entirely memory-backed virtual registers
- Even keeping HL as pointer and B as counter in physical registers would halve the overhead
- This is a register allocator improvement, not an iterator-specific fix

### Priority 5: Dead code elimination for inlined functions
- After fusion inlines a function, mark it unused if no other callers
- Remove from output

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

---

## References

- [ADR-0008: Flag-Based Boolean ABI](adr/0008-flag-based-boolean-abi-for-iterators.md)
- [ADR-0009: Superoptimizer Peephole Rules](adr/ADR-0009-superoptimizer-peephole-rules.md)
- [GenPlan](GenPlan.md) — canonical development roadmap
- [Archived docs](_archive/) — old iterator design docs and reports
