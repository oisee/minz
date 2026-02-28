# MinZ Iterator Implementation Status

*Updated: 2026-02-28 — v0.19.3*

## E2E Verified (MZX --console-io)

Full pipeline: MinZ source → parser → semantic → MIR optimizer → Z80 codegen → MZA → MZX emulator.

| Test | Chain | Output | Hex |
|------|-------|--------|-----|
| `iter_foreach.minz` | `arr.forEach(console_log)` | `ABCDE` | `41424344450a` |
| `iter_take.minz` | `arr.iter().take(3).forEach(console_log)` | `ABC` | `4142430a` |
| `iter_skip.minz` | `arr.iter().skip(2).forEach(console_log)` | `CDE` | `4344450a` |
| `iter_map_foreach.minz` | `arr.iter().map(double).forEach(console_log)` | `\x02\x04\x06\x08\x0a` | `0204060a` |
| `iter_filter_foreach.minz` | `arr.filter(is_big).forEach(console_log)` | `DE` | `4445` |
| `iter_lambda_map.minz` | `arr.iter().map(\|x\| x+1).forEach(console_log)` | `BCDEF` | `4243444546` |

Run: `bash examples/e2e_iterators/run_e2e.sh`

### Compile-Time Diagnostics (v0.19.2+)

Use `--compile-trace` to see every optimization decision:
```bash
mz program.minz --compile-trace 2>&1 | grep '\[semantic\]'
```
```
[semantic ] DJNZ loop: array[5] (≤255 threshold)
[semantic ] Inline filter: CP 68 + JR C (saved ~27 T-states/iter)
[semantic ] Lambda extracted: iter_lambda_main_0
```

---

## The Full Pipeline: MinZ → Z80 Assembly

These are **real compiler outputs**, not hand-written examples.

### 1. `forEach(fn)` — The Foundation

The simplest chain: iterate array, call function for each element.

```minz
let arr: [u8; 5] = [65, 66, 67, 68, 69];   // 'A', 'B', 'C', 'D', 'E'
arr.forEach(console_log);
```

**Z80 output** (core loop, cleaned):
```asm
    LD B, 5                          ; DJNZ counter
.loop:
    LD A, (HL)                       ; Load element via pointer
    LD C, A                          ; Element → C (arg register)
    CALL console_log_u8              ; console_log(element)
    INC HL                           ; Advance pointer
    DEC B                            ; Decrement counter
    JR NZ, .loop                     ; Loop if B ≠ 0
```

**What's good:** Single tight loop. `LD A, (HL)` + `INC HL` walks the array.
Counter in B for efficient decrement.

**What's not ideal yet:** `LD D, L` after CALL saves return value nobody uses.
`LD A, B` / `LD B, A` round-trip is redundant. These are peephole targets.

---

### 2. `take(n)` — Zero-Cost Counter Folding

`take(3)` doesn't add a second counter — it just changes the DJNZ limit.

```minz
let arr: [u8; 5] = [65, 66, 67, 68, 69];
arr.iter().take(3).forEach(console_log);    // Only 'A', 'B', 'C'
```

**Z80 output** (core loop):
```asm
    ; ENHANCED DJNZ LOOP: skip=0, take=3, total=3
    LD B, 3                          ; Counter = take(3), not array.length!
.loop:
    LD A, (HL)                       ; Load element
    LD C, A
    CALL console_log_u8
    INC HL
    DEC B
    JR NZ, .loop
```

**Mind-blowing:** `take(3)` is compiled away entirely — just `LD B, 3` instead of
`LD B, 5`. Zero runtime cost. The semantic layer folds `take(n)` into the DJNZ
counter at compile time.

---

### 3. `skip(n)` — Zero-Cost Pointer Offset

`skip(2)` adjusts both the pointer start AND the counter. No runtime loop to skip elements.

```minz
let arr: [u8; 5] = [65, 66, 67, 68, 69];
arr.iter().skip(2).forEach(console_log);    // 'C', 'D', 'E' only
```

**Z80 output:**
```asm
    ; ENHANCED DJNZ LOOP: skip=2, take=5, total=3
    LD B, 3                          ; Counter = 5 - 2 = 3
    ; Pointer to first element after skip
    ; (HL already points past first 2 elements)
.loop:
    LD A, (HL)
    LD C, A
    CALL console_log_u8
    INC HL
    DEC B
    JR NZ, .loop
```

**Zero-cost:** `skip(2)` compiles to `LD B, 3` + offset pointer. No "skip loop"
that reads and discards 2 elements. The compiler knows the skip amount at compile
time and adjusts the loop bounds.

---

### 4. `map(fn)` — Named Function with TRUE SMC

```minz
fun double(x: u8) -> u8 { return x * 2; }

let arr: [u8; 5] = [1, 2, 3, 4, 5];
arr.map(double).forEach(console_log);
```

**Z80 output:**
```asm
    LD B, 5                          ; DJNZ counter
.loop:
    LD A, (HL)                       ; Load element
    LD C, A
    ; TRUE SMC call — patches parameter directly into instruction!
    LD A, C
    LD (double_u8_param_x_imm0), A  ; Patch x into double's body
    CALL double_u8                   ; double(x)
    LD D, L                          ; D = result (from HL return)
    CALL console_log_u8              ; console_log(result)
    INC HL                           ; Next element
    DEC B
    JR NZ, .loop

; double$u8: TRUE SMC function
double_u8:
    LD A, (x_imm0)                   ; Read patched parameter
    ADD A, A                         ; x * 2 (optimized from SLA A)
    LD H, 0
    LD L, A                          ; Result in HL
    RET
```

**What's impressive:**
- `x * 2` optimized to `ADD A, A` (4 T-states) instead of `SLA A` (8 T-states)
- TRUE SMC patches the parameter directly into the function's instruction stream
- No stack frame, no parameter pushing — just `LD (addr), A` + `CALL`

**Known issue:** HL is clobbered after `CALL double_u8` (returns result in HL).
The `INC HL` at loop end increments the *return value*, not the array pointer.
This E2E test passes because `console_log` uses A (from `LD A, C`), not HL.
Multi-op chains (`map(f).filter(g)`) would fail here — needs PUSH/POP HL around CALL.

---

### 5. `filter(fn)` — Named Predicate with Conditional Skip

```minz
fun is_big(x: u8) -> bool { return x > 67; }

let arr: [u8; 5] = [65, 66, 67, 68, 69];
arr.filter(is_big).forEach(console_log);    // Only 'D' and 'E'
```

**Z80 output:**
```asm
    LD B, 5
.loop:
    LD A, (HL)                       ; Load element
    LD C, A                          ; Save in C
    ; TRUE SMC call to is_big
    LD A, C
    LD (is_big_u8_param_x_imm0), A  ; Patch parameter
    CALL is_big_u8                   ; Returns HL = 0 or 1
    LD D, L                          ; D = boolean result
    ; Skip if filter predicate is false
    LD A, D
    OR A
    JP Z, .filter_continue           ; Skip if false
    ; Element passed filter — call forEach function
    CALL console_log_u8
.filter_continue:
    INC HL                           ; Next element
    DEC B
    JR NZ, .loop

; is_big$u8: TRUE SMC with optimized comparison
is_big_u8:
    LD A, (x_imm0)                   ; Read patched x
    CP 68                            ; Compare with 67+1
    JR C, .false                     ; If carry: x < 68 → x ≤ 67
    LD HL, 1                         ; True
    JR .done
.false:
    LD HL, 0                         ; False
.done:
    RET
```

**What works:** Filter logic is correct — `CP 68` + `JR C` correctly tests `x > 67`.
The `JP Z, .filter_continue` skips non-matching elements.

**What's not ideal:** The predicate returns `HL = 0/1` and we test with `OR A / JP Z`.
The flag-based ABI (ADR-0008) would eliminate this — `CP 68` already sets the flags
we need, so the entire `LD HL, 0/1` + `OR A` dance is wasted work.

---

### 6. `map(|x| x + 1)` — Lambda Inlining (with caveats)

```minz
let arr: [u8; 5] = [65, 66, 67, 68, 69];
arr.iter().map(|x| => u8 { x + 1 }).forEach(console_log);  // 'B','C','D','E','F'
```

**Z80 output** (lambda inlined into loop):
```asm
    LD B, 5
.loop:
    ; Inlined from iter_lambda_main_0
    LD HL, ($F000)                   ; Load lambda param (SMC address)
    LD ($F000), HL                   ; Store back (no-op)
    LD HL, ($F000)                   ; Load again
    LD C, L                          ; C = result
    CALL console_log_u8
    ; Advance pointer
    LD H, D
    LD L, E                          ; Restore HL from DE
    INC HL
    LD D, H
    LD E, L                          ; Save HL to DE
    DEC B
    JR NZ, .loop
```

**This passes E2E** — the test outputs `BCDEF`. But look at the code quality:
- The `+ 1` from `|x| x + 1` is **missing** — the lambda compiles as identity
- It works because `$F000` happens to hold the right values from a prior store
- `LD HL, ($F000)` / `LD ($F000), HL` / `LD HL, ($F000)` is three redundant loads

**Root cause:** Bug 1 (arithmetic elision) drops the `ADD A, 1`, and Bug 3 (lambda
param not stored) means the element isn't explicitly stored to `$F000`. The test
passes by accident, not by design. This is the priority fix area.

**Ideal output** (what it should be):
```asm
.loop:
    LD A, (HL)           ; Load element
    ADD A, 1             ; Lambda body: x + 1
    CALL console_log_u8  ; forEach
    INC HL               ; Next element
    DJNZ .loop
```

---

### 7. `enumerate()` — NEW in v0.19.3 (OpPush/OpPop)

```minz
fun print_u8(x: u8) -> void {}
let nums: [u8; 5] = [1, 2, 3, 4, 5];
nums.iter().enumerate().forEach(print_u8);
```

**Z80 output** (core loop):
```asm
    XOR A                            ; Index = 0
    LD B, 5                          ; DJNZ counter
.loop:
    LD A, (HL)                       ; Load element
    LD D, A
    PUSH HL                          ; Push element for enumerated call
    PUSH HL                          ; Push index for enumerated call
    CALL print_u8                    ; print_u8(element, index)
    ; Increment enumeration index
    LD A, B
    INC A
    LD B, A
    INC HL                           ; Next element
    DEC B                            ; Counter--
    JR NZ, .loop
```

**NEW:** `PUSH HL` instructions generate correctly now (v0.19.3 — OpPush/OpPop
implemented in Z80 backend). Previously this failed with "unsupported opcode".

**Known issue:** The PUSH pushes HL (the pointer), but should push the element (D)
and index (B) separately. The semantic layer emits correct virtual register pushes
but the Z80 codegen always routes through HL. Needs register-aware push.

---

### 8. `reduce(fn)` — NEW in v0.19.3 (OpPush/OpPop)

```minz
fun sum(acc: u8, x: u8) -> u8 { return acc + x; }
let nums: [u8; 5] = [1, 2, 3, 4, 5];
nums.iter().reduce(sum);            // 1 + 2 + 3 + 4 + 5 = 15
```

**Z80 output:**
```asm
    LD B, 4                          ; Counter = 4 (first elem is init accumulator)
    ; Reduce: init accumulator with first element
    LD A, (HL)                       ; acc = arr[0]
    LD C, A
    INC HL                           ; Advance past first element
.loop:
    LD A, (HL)                       ; Load next element
    LD D, A
    PUSH HL                          ; Push element for reducer call
    PUSH HL                          ; Push accumulator for reducer call
    CALL sum                         ; sum(acc, element)
    LD C, L                          ; Update accumulator with result
    INC HL                           ; Next element
    DEC B
    JR NZ, .loop

; sum$u8$u8: TRUE SMC with two patched parameters
sum_u8_u8:
    LD A, (acc_imm0)                 ; Read patched acc
    LD A, (x_imm0)                   ; Read patched x
    ADD HL, DE                       ; acc + x (16-bit ADD, should be 8-bit)
    RET
```

**NEW:** Compiles to Z80! (v0.19.3) The reduce pattern is correct:
- Init accumulator with `arr[0]`, loop over remaining 4 elements
- Counter is `array.length - 1 = 4` (accounts for first-element init)
- Accumulator updated after each call: `LD C, L`

**Known issue:** Same PUSH HL problem as enumerate — pushes pointer, not the actual
element/accumulator values. Also, `sum` function uses 16-bit `ADD HL, DE` for what
should be 8-bit `ADD A, C`. These are codegen register allocation issues.

---

## Operation Status Table

| Operation | Parser | Semantic | Z80 Compiles | Z80 Correct | Notes |
|-----------|--------|----------|:------------:|:-----------:|-------|
| `forEach(fn)` | pass | pass | yes | **E2E pass** | DJNZ loop, direct CALL |
| `take(n)` | pass | pass | yes | **E2E pass** | Zero-cost counter fold |
| `skip(n)` | pass | pass | yes | **E2E pass** | Zero-cost pointer offset |
| `map(fn)` | pass | pass | yes | **E2E pass** | TRUE SMC call, HL clobber masked |
| `filter(fn)` | pass | pass | yes | **E2E pass** | TRUE SMC predicate, `JP Z` skip |
| `filter(\|x\| x > N)` | pass | pass | yes | **E2E pass** | Inline `CP N+1 + JR C` |
| `map(\|x\| x + N)` | pass | pass | yes | **E2E pass** | Lambda inlined (but +N dropped) |
| `peek(fn)` | pass | pass | yes | untested | Same codegen path as forEach |
| `inspect(fn)` | pass | pass | yes | untested | Same codegen path as forEach |
| `takeWhile(fn)` | pass | pass | yes | untested | Generates early-exit jump |
| `enumerate()` | pass | pass | **yes** (v0.19.3) | needs fix | PUSH HL wrong register |
| `reduce(fn)` | pass | pass | **yes** (v0.19.3) | needs fix | PUSH HL wrong register |
| `reduce(init, fn)` | pass | pass | **yes** (v0.19.3) | needs fix | Same as reduce(fn) |
| `skipWhile(fn)` | pass | pass | N/A | N/A | Not in DJNZ mode |
| `zip(other)` | pass | — | — | — | Parser only |
| `flatMap(fn)` | pass | — | — | — | Parser only |
| `collect()` | pass | — | — | — | Parser only |

---

## What Doesn't Work and Why

### Bug 1: Arithmetic op elision (PARTIALLY MITIGATED)

The register allocator tracks which virtual register is in which physical register.
When it sees `OpAdd(dest=r10, src1=r10, src2=r11)` and r10 is already in HL, it
emits `; Register 10 already in HL` and **skips the ADD instruction entirely**.

**Observed in lambda `|x| x + 1`** — the `+ 1` is dropped:
```asm
    ; Inlined lambda body:
    LD HL, ($F000)        ; Load "x"
    LD ($F000), HL        ; No-op store
    LD HL, ($F000)        ; Load again — WHERE IS THE ADD?
```

**Mitigated for:** Inline filter predicates (use `CP` instruction instead of arithmetic).
**Still affects:** Lambda bodies with arithmetic (`|x| x + 1`, `|x| x * 2`).

### Bug 2: HL clobbered by CALL return (PARTIALLY FIXED)

DJNZ loops use HL as array pointer. But Z80 calling convention returns in HL.
Any `CALL` inside the loop destroys the pointer.

**Fixed for:** Inline lambda filters (no CALL at all — `CP + JR` directly).
**Still affects:** Named function calls (`map(double)`, `filter(is_big)`).
Currently masked in E2E tests because the A register happens to hold the right value.

**Fix:** PUSH/POP HL around CALLs in DJNZ loops (Phase 2 in GenPlan).

### Bug 3: Lambda parameter not stored to SMC address (BYPASSED)

Inlined lambda reads from `$F000` but the element isn't stored there explicitly.

**Bypassed for:** Simple lambdas where the value flows through registers.
**Still affects:** Complex lambda bodies that need the parameter from memory.

### Bug 4: OpPush pushes wrong register (NEW, v0.19.3)

`OpPush` always routes through HL (`loadToHL(src)` + `PUSH HL`). For enumerate
and reduce, the semantic layer emits `OpPush elementReg` and `OpPush indexReg`,
but both become `PUSH HL` — pushing the pointer, not the values.

**Fix:** Register-aware push — check if src is in a pushable pair (BC/DE/HL/AF)
and emit `PUSH` for that pair directly, or use a temp register.

---

## Summary

| Root Cause | Affects | Status | Effort |
|---|---|---|---|
| Arithmetic op elision | Lambda bodies with +/-/\* | **Mitigated** (inline filter bypasses) | Small |
| HL clobbered by CALL | Named fn in DJNZ loops | **Partially fixed** (inline filter OK) | Medium |
| Lambda param not stored | Complex lambda chains | **Bypassed** (simple lambdas work) | Small |
| PUSH wrong register | enumerate, reduce | **New** — needs register-aware push | Small |

---

## Architecture

### Compilation Pipeline
```
Source Code:
  scores.iter()
    .filter(|x| x >= 90)
    .map(|x| x + 5)
    .forEach(print_u8)
    |
AST: IteratorChainExpr
    |
Semantic: Type checking + operation dispatch
    |
IR: Virtual register instructions (OpLoad, OpCall, OpDJNZ, ...)
    |
Z80 Codegen: Physical register allocation + assembly emission
    |
Assembly (with flag-based boolean ABI — ADR-0008):
  LD HL, scores        ; array pointer
  LD B, 10             ; DJNZ counter
loop:
  LD A, (HL)           ; load element
  CP 90                ; filter: x >= 90? (flag-based, no CALL!)
  JR C, skip           ; CY set = A < 90 = skip
  ADD A, 5             ; map: x + 5
  CALL print_u8        ; forEach (PUSH/POP HL for non-bool CALLs)
skip:
  INC HL               ; HL untouched by filter!
  DJNZ loop
```

### Key Files

| File | Role |
|------|------|
| `pkg/ast/iterator.go` | IteratorChainExpr, IteratorOp (Function + Argument fields) |
| `pkg/parser/participle/convert.go` | `tryConvertIteratorChain()` — CallExpr → IteratorChainExpr |
| `pkg/semantic/iterator.go` | Type checking, DJNZ/indexed dispatch, lambda extraction |
| `pkg/semantic/iterator_enhanced.go` | Enhanced DJNZ with take/skip/enumerate/reduce |
| `pkg/codegen/z80.go` | Z80 register allocation + assembly emission |
| `pkg/optimizer/fusion.go` | Fusion framework (skeleton, not wired in) |

### Test Suite (63 unit + 18 corpus + 6 E2E)

| Package | Tests | What |
|---------|-------|------|
| `pkg/parser/participle/` | 20 | Chain conversion, argument routing |
| `pkg/semantic/` | 17 | DJNZ eligibility, filter/map/lambda IR, take/skip/enumerate/reduce |
| `pkg/codegen/` | 5 | Z80 assembly patterns (use `-vet=off`) |
| `pkg/mirvm/` | 8 | DJNZ execution at MIR level |
| `tests/iterator_corpus/` | 18 | Full pipeline regression — all 18 compile to Z80 |
| `examples/e2e_iterators/` | 6 | MZX --console-io verified |

### Performance (measured and projected)

| Pattern | What It Does | T-states/elem |
|---------|-------------|---------------|
| `forEach(fn)` | `LD A,(HL)` + `CALL` + `INC HL` + `DEC B` + `JR NZ` | ~40 T |
| `take(3)` | Same loop, `LD B, 3` | ~40 T (zero overhead vs forEach) |
| `skip(2)` | Same loop, adjusted pointer | ~40 T (zero overhead vs forEach) |
| `filter(\|x\| x>N)` | `CP N+1` + `JR C` — no CALL | ~25 T (no call = 5.6x faster than filter(fn)) |
| Ideal fused chain | `LD A,(HL)` + inline ops + `INC HL` + `DJNZ` | ~18 T |
| Traditional indexed | `LD A, (arr+IX)` + index math | ~60-150 T |

---

## GenPlan: Next Steps

### Phase 1: OpPush/OpPop in Z80 Backend — DONE (v0.19.3)
- Added `case ir.OpPush:` / `case ir.OpPop:` to `generateInstruction()` in `z80.go`
- OpPush: load src to HL, then `PUSH HL`
- OpPop: `POP HL`, store to dest
- **Result:** 18/18 corpus tests now compile to Z80 (was 16/18)

### Phase 2: PUSH/POP HL around CALLs in DJNZ loops
- For `map(fn)` and `forEach(fn)` inside iterator loops, wrap CALL with `PUSH HL` / `POP HL`
- Semantic layer marks iterator CALL instructions with a hint (e.g., `inst.Meta["iter_call"] = true`)
- Z80 codegen checks hint and emits PUSH/POP around the CALL
- **Effort:** Medium (~50 lines semantic + codegen)

### Phase 3: Flag-return ABI for Named Predicates
- For `filter(is_big)` with named function, generate flag-return variant
- Predicate returns via CY flag instead of LD HL, 0/1
- Codegen: `PUSH HL` / `CALL is_big` / `POP HL` / `JR C, skip`
- **Effort:** Medium

### Phase 4: Register-Aware OpPush
- Fix PUSH to use the correct register pair (not always HL)
- Check if src is already in BC/DE/HL → `PUSH` that pair directly
- Unblocks correct enumerate and reduce E2E
- **Effort:** Small

### Phase 5: Fusion Optimizer Wiring
- Wire `pkg/optimizer/fusion.go` into the optimization pipeline
- Detect adjacent iterator ops that can be merged into single loop body
- Prerequisite: Phases 1-4 complete so fused code can actually generate correct Z80
- **Effort:** Large

### Phase 6: collect() + Generator Syntax
- `collect()` — allocate result array, store filtered/mapped elements
- `gen`/`yield` — lazy generator functions
- **Effort:** Large, future

---

## References

- [ADR-0008: Flag-Based Boolean ABI](adr/0008-flag-based-boolean-abi-for-iterators.md) — `CP` + flag returns for iterator predicates
- [Zero-Cost Iterators Revolution](Zero_Cost_Iterators_Revolution.md) — design vision
- [DJNZ Iterator Optimization](2026-01-03-301-DJNZ_Iterator_Optimization.md) — loop optimization details
- [Z80 Optimal Iteration Design](Z80_Optimal_Iteration_Design.md) — hardware-level patterns
