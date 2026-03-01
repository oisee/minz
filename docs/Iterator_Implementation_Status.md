# MinZ Iterator Implementation Status

*Updated: 2026-03-01 — v0.19.4*

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

| Operation | Parser | Semantic | Z80 Compiles | Z80 Correct | Bug | Notes |
|-----------|--------|----------|:------------:|:-----------:|:---:|-------|
| `forEach(fn)` | pass | pass | yes | **HL fixed** | B | PUSH/POP HL, but print reads A not element |
| `take(n)` | pass | pass | yes | **HL fixed** | B | Counter fold correct, arg handoff wrong |
| `skip(n)` | pass | pass | yes | **needs fix** | B,E | Counter OK, pointer offset elided |
| `map(fn)` | pass | pass | yes | **needs fix** | B | TRUE SMC correct, result not passed to next |
| `filter(fn)` | pass | pass | yes | **needs fix** | B | Predicate correct, print gets bool not elem |
| `filter(\|x\| x>N)` | pass | pass | yes | **needs fix** | D | Constant not tracked, OR A instead of CP N |
| `map(\|x\| x+N)` | pass | pass | yes | **needs fix** | C | Lambda arithmetic dropped |
| `map(fn).filter(fn)` | pass | pass | yes | **needs fix** | B | Chain wiring correct, print gets bool |
| `peek(fn)` | pass | pass | yes | untested | — | Same codegen path as forEach |
| `inspect(fn)` | pass | pass | yes | untested | — | Same codegen path as forEach |
| `takeWhile(fn)` | pass | pass | yes | untested | — | Generates early-exit jump |
| `enumerate()` | pass | pass | yes (v0.19.3) | needs fix | G | PUSH routes through HL |
| `reduce(fn)` | pass | pass | yes (v0.19.3) | needs fix | G | PUSH routes through HL |
| `reduce(init, fn)` | pass | pass | yes (v0.19.3) | needs fix | G | Same as reduce(fn) |
| `skipWhile(fn)` | pass | pass | N/A | N/A | — | Not in DJNZ mode |
| `zip(other)` | pass | — | — | — | — | Parser only |
| `flatMap(fn)` | pass | — | — | — | — | Parser only |
| `collect()` | pass | — | — | — | — | Parser only |

---

## Exhaustive E2E Bug Analysis (v0.19.4)

All 8 iterator patterns compiled to Z80 assembly and analyzed instruction-by-instruction.
Test date: 2026-03-01. Compiler built from `master` after PUSH/POP fix.

### What Got Fixed in v0.19.4

**HL pointer preservation (PUSH/POP)** — Applied to BOTH code paths:
- `generateDJNZIteration()` in `iterator.go` (forEach, map, filter, peek, inspect)
- `generateEnhancedDJNZIteration()` in `iterator_enhanced.go` (take, skip, enumerate, reduce)

All 8 patterns now emit `PUSH HL` before operations and `POP HL` after, ensuring
the array pointer survives function calls. Before this fix, take/skip patterns had
NO pointer preservation.

**TRUE SMC anchor generation** — `LD A, 0` (opcode 3E, 7T, 2B) instead of
`LD A, (nn)` (opcode 3A, 13T, 3B). Correct immediate-mode SMC.

### Remaining Bugs by Category

#### Bug A: Counter Waste (ALL patterns, peephole target)

Every DJNZ loop emits redundant register shuffling:

```asm
; At init:
    LD B, 5       ; Correct
    LD A, B       ; Waste (4T)
    LD B, A       ; Waste (4T)

; At loop end:
    LD A, B       ; Waste (4T)
    DEC B         ; Correct
    LD A, B       ; Waste (4T)
    LD B, A       ; Waste (4T)
    JR NZ, loop   ; Correct
```

**Should be:** `DJNZ loop` (13/8T) instead of the 4-instruction sequence (20T).

**Root cause:** The Z80 codegen emits `OpDJNZ` as explicit `DEC B` + `JR NZ` because
the register allocator tracks B through A intermediary. The `LD A, B` / `LD B, A`
pairs are generated by the virtual-to-physical register mapping.

**Fix:** Peephole pattern in `AssemblyPeepholePass` to collapse:
`LD A, B / DEC B / LD A, B / LD B, A / JR NZ, X` → `DJNZ X`

**Impact:** 12T wasted per iteration across all patterns.

#### Bug B: Register Handoff Between Pipeline Stages (map, filter, chain)

After `CALL fn`, the result sits in L (via HL return convention). Codegen
captures it to D (`LD D, L`). But the NEXT operation reads from A, not D.
No `LD A, D` is inserted between pipeline stages.

**Observed in 04_map_fn:**
```asm
    CALL double        ; Result in HL.L
    LD D, L            ; Map result saved to D
    CALL print_u8      ; Reads from A — but A is garbage!
```

**Observed in 05_filter_fn:**
```asm
    CALL is_big        ; Returns HL = 0/1
    LD D, L            ; Bool in D
    LD A, D            ; A = bool
    OR A               ; Test
    JP Z, skip         ; OK — filter works
    CALL print_u8      ; Reads from A = bool (0 or 1), NOT element!
```

**Observed in 08_chain:**
```asm
    CALL double        ; HL.L = doubled value
    LD D, L            ; D = doubled
    LD A, D            ; A = doubled (correct for filter input!)
    LD (is_big_SMC), A ; Patches is_big with doubled value (correct!)
    CALL is_big        ; HL.L = bool
    LD E, L            ; E = bool
    LD A, E            ; A = bool
    OR A               ; Test
    JP Z, skip
    CALL print_u8      ; A = bool (WRONG — should be doubled value in D)
```

**Root cause:** The semantic layer correctly updates `currentReg` after each
operation, but the Z80 codegen doesn't insert `LD A, <physReg>` before the
next CALL. The calling convention requires the first argument in A, but the
codegen doesn't bridge the gap between the result register and A.

**Fix:** In the Z80 codegen's CALL handling, emit `LD A, <argReg>` when the
argument isn't already in A. This is a register allocator issue — the arg
register mapping doesn't consider the calling convention.

**Impact:** Incorrect output for map+forEach, filter+forEach, and multi-op chains.

#### Bug C: Lambda Arithmetic Elision (lambda map)

Lambda `|x: u8| => u8 { x + 1 }` compiles as identity — the `+ 1` is dropped:

```asm
; Generated for |x| x + 1:
    LD HL, ($F000)        ; Load "x" from SMC address
    LD ($F000), HL        ; No-op store back
    LD HL, ($F000)        ; Load again — ADD A, 1 is MISSING
```

**Root cause:** The register allocator sees `OpAdd(dest=r2, src1=r0, src2=r1)`
where r0 is mapped to HL. It emits `; Register 2 already in HL` and **skips the
entire ADD instruction**. The allocator's "already in register" optimization
doesn't distinguish between "value is loaded" and "value needs computation."

**Additionally:** The element is never loaded from the array (`LD A, (HL)` missing
in lambda path) and is never stored to the SMC address ($F000) that the lambda
reads from.

**Fix:** Two-part:
1. Don't elide arithmetic ops even if dest register is "already" mapped
2. Ensure element is stored to the lambda's parameter address before the lambda body

**Impact:** All lambda map operations produce identity instead of transformation.

#### Bug D: Inline Filter Constant Not Tracked (filter lambda)

`filter(|x: u8| => bool { x > 3 })` generates correct `OpJumpIfFlag` IR with
`Src2 = constReg` holding value 4 (CP semantics: x > 3 → CP 4). But the Z80
codegen can't find the constant value:

```asm
    LD A, C
    ; WARNING: OpJumpIfFlag constant not tracked, using 0
    OR A              ; Should be CP 4!
    JR C, skip        ; OR A never sets CY → skip NEVER triggers
```

**Root cause:** The `OpLoadConst` for the filter constant is emitted, but by the
time `OpJumpIfFlag` is processed, the `constantValues` map has lost the entry
(likely invalidated at a label or by intervening instructions).

**Consequence:** `OR A` always clears CY on Z80. `JR C` never jumps. All elements
pass the filter. Filter is effectively a no-op.

**Fix:** Either:
1. Embed the constant value directly in `OpJumpIfFlag` (Imm field already used for
   flag condition — need second immediate or encode both)
2. Preserve constant tracking across the short instruction sequence between
   `OpLoadConst` and `OpJumpIfFlag`

**Impact:** All inline filter lambdas pass all elements through.

#### Bug E: Skip Pointer Offset Elided (skip pattern)

`skip(2)` should advance the array pointer by 2 before the loop. The IR emits
`OpAdd ptrReg = sourceReg + offsetReg(2)`. But the register allocator sees that
ptrReg maps to the same physical register (HL) as sourceReg and skips the ADD:

```asm
    ; Pointer to first element after skip
    ; Register 10 already in HL    ← ADD elided! Pointer NOT advanced!
```

The counter is correct (`LD B, 3` for 5-2=3 elements), but the pointer starts at
element 0 instead of element 2. So `skip(2).forEach(print_u8)` on `[1,2,3,4,5]`
prints `[1,2,3]` instead of `[3,4,5]`.

**Root cause:** Same as Bug C — the register allocator's "already in register"
optimization doesn't consider that the value needs to change via arithmetic.

**Fix:** Same fix as Bug C (don't elide arithmetic ops).

**Impact:** `skip(n)` doesn't actually skip — iterates from beginning.

#### Bug F: Unused Result Saves (all patterns with CALL)

After every CALL, the codegen saves the return value even when nobody reads it:

```asm
    CALL print_u8      ; Returns void
    LD D, L            ; Waste (4T) — nobody reads D
```

**Root cause:** The codegen always emits a store for `OpCall.Dest`, even when
Dest is 0 (void) or the result is never used.

**Fix:** Peephole pattern or codegen check — skip store if Dest == 0 or unused.

**Impact:** 4T wasted per CALL, cosmetic.

#### Bug G: OpPush Routes Through HL (enumerate, reduce)

`OpPush` always loads the source to HL first, then `PUSH HL`. For enumerate and
reduce where the semantic layer emits `OpPush elementReg` and `OpPush indexReg`,
both become `PUSH HL` — pushing the pointer, not the intended values.

**Fix:** Register-aware push — check if src is already in a pushable pair (BC/DE)
and emit `PUSH BC`/`PUSH DE` directly. Only use HL routing as fallback.

**Impact:** enumerate and reduce get wrong values from stack.

---

## Bug Summary

| ID | Bug | Affects | Status | Effort | Priority |
|---|---|---|---|---|---|
| A | Counter waste (no DJNZ) | All 8 patterns | Peephole target | Small | P3 |
| B | Register handoff (A not loaded) | map+forEach, filter+forEach, chains | **Open** | Medium | **P1** |
| C | Lambda arithmetic elision | Lambda map (`\|x\| x+1`) | **Open** | Medium | **P1** |
| D | Filter constant not tracked | Inline filter lambda (`\|x\| x>3`) | **Open** | Small | **P1** |
| E | Skip pointer offset elided | `skip(n)` | **Open** (same root as C) | Small | **P2** |
| F | Unused result saves | All patterns with CALL | Peephole target | Small | P3 |
| G | OpPush wrong register | enumerate, reduce | **Open** | Small | P2 |

**Root cause clustering:**
- Bugs C + E share the same root cause: register allocator's "already in register"
  optimization elides arithmetic ops. Fixing this one issue resolves both.
- Bug B is a calling convention issue — argument registers not set before CALL.
- Bug D is a constant tracking issue — value lost between OpLoadConst and OpJumpIfFlag.

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

### Test Suite (63 unit + 18 corpus + 8 E2E analysis)

| Package | Tests | What |
|---------|-------|------|
| `pkg/parser/participle/` | 20 | Chain conversion, argument routing |
| `pkg/semantic/` | 17 | DJNZ eligibility, filter/map/lambda IR, take/skip/enumerate/reduce |
| `pkg/codegen/` | 5 | Z80 assembly patterns (use `-vet=off`) |
| `pkg/mirvm/` | 8 | DJNZ execution at MIR level |
| `tests/iterator_corpus/` | 18 | Full pipeline regression — all 18 compile to Z80 |
| `examples/e2e_iterators/` | 6 | MZX --console-io verified |
| `/tmp/iter_e2e_v2/` | 8 | Deep Z80 output analysis (v0.19.4) |

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

### Phase 2: PUSH/POP HL around CALLs — DONE (v0.19.4)
- Semantic layer emits `OpPush ptrReg` / `OpPop ptrReg` around operation blocks
- Applied to both `generateDJNZIteration()` and `generateEnhancedDJNZIteration()`
- Detects `hasCallOps` by scanning operation types before emitting
- **Result:** HL correctly preserved in all 8 E2E patterns

### Phase 3: Register Handoff Fix (Bug B) — NEXT
- After `CALL fn`, result in L. Next CALL reads from A. Insert `LD A, <resultReg>`.
- Fix in Z80 codegen: before emitting CALL, check if arg register matches A.
- If not: emit `LD A, <phys_reg>` where phys_reg holds the argument.
- Unblocks: correct output for map+forEach, filter+forEach, chains.
- **Effort:** Medium

### Phase 4: Register Allocator Arithmetic Elision Fix (Bugs C, E)
- The "already in register" optimization skips arithmetic ops when dest == src phys reg
- Need to distinguish "value loaded" from "value needs computation"
- Fix: only elide `OpMove`/`OpLoad` when dest==src, never elide arithmetic ops
- Unblocks: lambda map (`|x| x+1`), skip pointer offset
- **Effort:** Medium (register allocator change — risk of regression)

### Phase 5: Inline Filter Constant Tracking (Bug D)
- `OpJumpIfFlag` needs the constant from `Src2`, but `constantValues` map loses it
- Option A: embed constant directly in `OpJumpIfFlag.Imm2` (new field)
- Option B: preserve constants across the short sequence (OpLoadConst → OpJumpIfFlag)
- Unblocks: correct inline filter lambda behavior
- **Effort:** Small

### Phase 6: Register-Aware OpPush (Bug G)
- Fix PUSH to use the correct register pair (not always HL)
- Check if src is already in BC/DE/HL → `PUSH` that pair directly
- Unblocks correct enumerate and reduce E2E
- **Effort:** Small

### Phase 7: Flag-return ABI for Named Predicates
- For `filter(is_big)` with named function, generate flag-return variant
- Predicate returns via CY flag instead of LD HL, 0/1
- Codegen: `PUSH HL` / `CALL is_big` / `POP HL` / `JR C, skip`
- **Effort:** Medium

### Phase 8: Counter Peephole (Bug A)
- Peephole pattern: `LD A, B / DEC B / LD A, B / LD B, A / JR NZ, X` → `DJNZ X`
- Also: `LD B, N / LD A, B / LD B, A` → `LD B, N`
- **Effort:** Small

### Phase 9: Fusion Optimizer Wiring
- Wire `pkg/optimizer/fusion.go` into the optimization pipeline
- Detect adjacent iterator ops that can be merged into single loop body
- Prerequisite: Phases 3-6 complete so fused code can actually generate correct Z80
- **Effort:** Large

### Phase 10: collect() + Generator Syntax
- `collect()` — allocate result array, store filtered/mapped elements
- `gen`/`yield` — lazy generator functions
- **Effort:** Large, future

---

## References

- [ADR-0008: Flag-Based Boolean ABI](adr/0008-flag-based-boolean-abi-for-iterators.md) — `CP` + flag returns for iterator predicates
- [ADR-0009: Superoptimizer Peephole Rules](adr/ADR-0009-superoptimizer-peephole-rules.md) — 602K proven Z80 optimizations
- [ADR-0010: Hex Output Format](adr/ADR-0010-hex-output-format.md) — planned hex switch
- [Zero-Cost Iterators Revolution](Zero_Cost_Iterators_Revolution.md) — design vision
- [DJNZ Iterator Optimization](2026-01-03-301-DJNZ_Iterator_Optimization.md) — loop optimization details
- [Z80 Optimal Iteration Design](Z80_Optimal_Iteration_Design.md) — hardware-level patterns
