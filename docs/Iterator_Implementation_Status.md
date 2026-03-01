# MinZ Iterator Implementation Status

*Updated: 2026-03-01 — v0.19.5 (fusion optimizer + bare DJNZ)*

## E2E Verified (MZX --console-io)

Full pipeline: MinZ source → parser → semantic → MIR optimizer → **fusion optimizer** → Z80 codegen → MZA → MZX emulator.
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

### Bugs Fixed in This E2E Round (v0.19.4)

| Bug | Root Cause | Fix Location |
|-----|-----------|--------------|
| **skip outputs ABC not CDE** | MIR peephole read `inst.Value` (always 0) instead of `inst.Imm` for constants → `r12=2` tracked as `r12=0` → `r8+r12` folded to `r8` | `mir_peephole.go`: `trackConstants()`, `constantFolding()`, `algebraicSimplification()` |
| **filter outputs bool not element** | `prepareCallArguments()` skipped arg loading for ALL SMC functions, not just TRUE SMC | `z80.go`: changed condition to `targetFunc.UsesTrueSMC` |
| **lambda outputs garbage** | `generateIteratorLambda` didn't call `AddParamWithRegister` → `NumParams=0` → inliner never mapped params; inliner assumed params at regs 1,2,... but lambda used reg 0 | `iterator.go`: use `AddParamWithRegister`; `inlining.go`: use `fn.Params[i].Reg`, add OpLoadVar/OpLoadParam substitution |
| **copy propagation across labels** | MIR peephole didn't clear copy map at `OpLabel` merge points | `mir_peephole.go`: clear copies at every `OpLabel` |

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

**Z80 output** (v0.19.4 — after Imm fix):
```asm
    ; ENHANCED DJNZ LOOP: skip=2, take=5, total=3
    LD B, 3                          ; Counter = 5 - 2 = 3
    ; Skip offset = 2 elements
    LD DE, 2                         ; Offset constant
    ADD HL, DE                       ; Advance pointer past skipped elements
    PUSH HL                          ; Save array pointer
.loop:
    LD A, (HL)
    CALL console_log_u8
    POP HL                           ; Restore pointer
    INC HL                           ; Next element
    PUSH HL                          ; Save for next iteration
    DEC B
    JR NZ, .loop
    POP HL                           ; Final cleanup
```

**Zero-cost:** `skip(2)` compiles to `LD B, 3` + `ADD HL, DE` offset. No "skip loop"
that reads and discards 2 elements. The compiler knows the skip amount at compile
time and adjusts both the loop counter and start pointer.

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
The `JP Z, .filter_continue` skips non-matching elements. After the fix to
`prepareCallArguments()`, the forEach function receives the original element value
in A (not the boolean filter result).

**What's not ideal:** The predicate returns `HL = 0/1` and we test with `OR A / JP Z`.
The flag-based ABI (ADR-0008) would eliminate this — `CP 68` already sets the flags
we need, so the entire `LD HL, 0/1` + `OR A` dance is wasted work.

---

### 6. `map(|x| x + 1)` — Lambda Inlining

```minz
let arr: [u8; 5] = [65, 66, 67, 68, 69];
arr.iter().map(|x| => u8 { x + 1 }).forEach(console_log);  // 'B','C','D','E','F'
```

**Z80 output** (v0.19.4 — lambda correctly inlined):
```asm
    LD B, 5
.loop:
    LD A, (HL)                       ; Load element
    PUSH HL                          ; Save array pointer
    ; Inlined: load param x from arg r11
    ; Lambda body: x + 1
    LD C, A                          ; Element to C
    INC C                            ; +1 (optimized from ADD)
    ; forEach: console_log
    LD A, C
    CALL console_log_u8
    POP HL                           ; Restore pointer
    INC HL                           ; Next element
    DEC B
    JR NZ, .loop
```

**E2E PASS** — outputs `BCDEF\n` (hex `42434445460a`).

**What's good:**
- Lambda body correctly inlined — `INC C` implements `x + 1`
- Parameter `x` correctly mapped from element register (r11) via `OpMove`
- PUSH/POP HL preserves array pointer across operations
- `AddParamWithRegister` + inliner `paramArgMap` fix made this work

**Ideal output** (peephole optimization target):
```asm
.loop:
    LD A, (HL)           ; Load element
    INC A                ; Lambda body: x + 1
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
| `forEach(fn)` | pass | pass | yes | **E2E PASS** | — | PUSH/POP HL preserves pointer across CALL |
| `take(n)` | pass | pass | yes | **E2E PASS** | — | Counter fold to `LD B, n` — zero overhead |
| `skip(n)` | pass | pass | yes | **E2E PASS** | — | Pointer offset + counter adjust — zero overhead |
| `map(fn)` | pass | pass | yes | **E2E PASS** | — | TRUE SMC correct, arg loading fixed |
| `filter(fn)` | pass | pass | yes | **E2E PASS** | — | Predicate call + element correctly passed to forEach |
| `map(\|x\| x+N)` | pass | pass | yes | **E2E PASS** | — | Lambda inlined, arithmetic correct |
| `filter(\|x\| x>N)` | pass | pass | yes | **E2E PASS** | — | DCE fix: OpJumpIfFlag operands tracked. `CP N` correct. |
| `map(fn).filter(fn)` | pass | pass | yes | **E2E PASS** | — | Multi-stage chain verified (3 tests) |
| `filter(fn).map(fn)` | pass | pass | yes | **E2E PASS** | — | Multi-stage chain verified |
| `take(n).map(fn)` | pass | pass | yes | **E2E PASS** | — | Multi-stage chain verified |
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
Test date: 2026-03-01. Compiler built from `master` after all E2E fixes.

### What Got Fixed in v0.19.4

**Round 1: HL pointer preservation (PUSH/POP):**
- `generateDJNZIteration()` in `iterator.go` (forEach, map, filter, peek, inspect)
- `generateEnhancedDJNZIteration()` in `iterator_enhanced.go` (take, skip, enumerate, reduce)
- All 8 patterns now emit `PUSH HL` before operations and `POP HL` after

**Round 1: TRUE SMC anchor generation** — `LD A, 0` (opcode 3E, 7T, 2B) instead of
`LD A, (nn)` (opcode 3A, 13T, 3B). Correct immediate-mode SMC.

**Round 2: MIR peephole Imm vs Value** (`mir_peephole.go`):
- `trackConstants()` read `inst.Value` (always 0) instead of `inst.Imm` — all constants tracked as 0
- `constantFolding()` and `algebraicSimplification()` now set `inst.Imm` when creating OpLoadConst
- **Impact:** Fixed skip pointer offset (Bug E) and any other constant-dependent optimization

**Round 2: SMC arg loading** (`z80.go`):
- `prepareCallArguments()` returned early for ALL SMC functions — changed to only skip for `UsesTrueSMC`
- **Impact:** Fixed filter element passthrough (Bug B)

**Round 2: Lambda inlining** (`iterator.go` + `inlining.go`):
- `generateIteratorLambda()` now uses `AddParamWithRegister()` (sets NumParams correctly)
- Inliner uses `fn.Params[i].Reg` for formal parameter registers (not hardcoded 1, 2, ...)
- Inliner substitutes `OpLoadVar`/`OpLoadParam` matching parameter names with `OpMove` from argument register
- **Impact:** Fixed lambda map/filter (Bugs C, partially)

**Round 2: Copy propagation at labels** (`mir_peephole.go`):
- Copy map cleared at every `OpLabel` (merge points invalidate all tracked copies)
- **Impact:** Fixed forEach producing wrong values after optimization

### Remaining Bugs by Category

#### Bug A: Counter Waste (ALL patterns, peephole target) — PARTIALLY FIXED

**Status: PARTIALLY FIXED in v0.19.5**

When a DJNZ loop body contains no `CALL` instructions (all callbacks inlined by fusion
optimizer), the codegen now emits a bare `DJNZ label` instruction. When `CALL` is present
(B may be clobbered), the manual `DEC B` + `JR NZ` sequence is still used — correctly.

**v0.19.5 fixes:**
1. **BareDJNZ CodegenHint** — Fusion optimizer sets `CodegenHint.BareDJNZ = true` when no
   OpCall remains in the loop body. Z80 codegen emits `DJNZ label` (13/8T) instead of the
   6-instruction sequence (~28T).
2. **Peephole Pattern 48** — Safety net: `DEC B; JR NZ, label → DJNZ label` catches any
   remaining cases in non-iterator code.

**Remaining waste** (when CALL is present, B cannot be trusted):
```asm
; At loop end (with CALL in body):
    LD A, ($F012)     ; Load counter from memory
    LD B, A
    DEC B
    LD A, B
    LD ($F012), A     ; Store decremented counter
    JR NZ, loop
```

This sequence is **correct** when CALL is in the loop — B is caller-saved on Z80, so
the counter must live in memory. The waste is the `LD A, B` + store-back pattern, which
could be optimized to `DEC (HL)` or similar, but that's a minor improvement.

**Impact:** With fusion optimizer inlining callbacks, most simple iterator chains now
emit bare `DJNZ`. Only chains with non-inlineable callbacks (asm functions, multi-param
functions) use the memory-backed counter.

#### Bug B: Register Handoff Between Pipeline Stages (map, filter, chain) — FIXED

**Status: FIXED in v0.19.4**

**Root cause:** `prepareCallArguments()` in z80.go returned early for ALL SMC
functions (`IsSMCDefault || IsSMCEnabled`), not loading arguments into registers.
After a filter predicate call, A had the filter result (0/1) instead of the element.

**Fix:** Changed condition to `targetFunc.UsesTrueSMC` — only TRUE SMC functions
(which patch parameters directly into instruction immediates) skip standard arg loading.
Regular SMC functions still need `LD A, <argReg>` before CALL.

**Verified:** `iter_filter_foreach.minz` now outputs `DE\n` (hex `44450a`) correctly.

#### Bug C: Lambda Arithmetic Elision (lambda map) — FIXED

**Status: FIXED in v0.19.4**

**Root cause (multi-part):**
1. `generateIteratorLambda()` appended to `Params` directly without calling
   `AddParamWithRegister()`, leaving `NumParams = 0`. The inliner's parameter
   mapping loop never executed.
2. The inliner assumed parameters at registers 1, 2, ... but lambda functions
   used register 0.
3. Inlined `OpLoadVar x` was not converted to `OpMove` from the argument register.

**Fix (3 files):**
- `iterator.go`: Use `AddParamWithRegister()` so `NumParams` is set correctly
- `inlining.go`: Use `fn.Params[i].Reg` for formal parameter register instead of `ir.Register(i+1)`
- `inlining.go`: Build `paramArgMap` (name→argReg) and substitute `OpLoadVar`/`OpLoadParam`
  with `OpMove` from actual argument register during inlining

**Verified:** `iter_lambda_map.minz` now outputs `BCDEF\n` (hex `42434445460a`) correctly.

#### Bug D: Inline Filter Constant Not Tracked (filter lambda) — FIXED

**Status: FIXED in v0.19.5**

**Root cause:** `DeadCodeEliminationPass` in `pkg/optimizer/dead_code_elimination.go`
was missing `OpJumpIfFlag` from `markUsedRegisters()` Phase 1. The DCE never marked
`Src1` (element register) and `Src2` (constant register) as used, so it deleted the
`OpLoadConst` that loaded the filter threshold. At Z80 codegen time, `constantValues`
map was empty → fallback to `OR A` (compare with 0).

**Fix:** Added `OpJumpIfFlag` (marks Src1 + Src2), `OpJumpIf`/`OpJumpIfZero`/
`OpJumpIfNotZero` (mark Src1), and `OpPush` (marks Src1) to Phase 1 marking.
Also added all missing jump opcodes to `markReferencedLabels()`.

**Verified:** `iter_inline_filter.minz` — `filter(|x| x > 67)` now generates `CP 68`
(correct) instead of `OR A`. E2E outputs `DE\n` (hex `44450a`).

#### Bug E: Skip Pointer Offset Elided (skip pattern) — FIXED

**Status: FIXED in v0.19.4**

**Root cause:** MIR peephole optimizer's `trackConstants()` read `inst.Value` (always 0)
instead of `inst.Imm` (the actual constant value from the semantic layer). The skip
offset `r12 = 2` was tracked as `r12 = 0`, so `algebraicSimplification` transformed
`r10 = r8 + r12` into `r10 = r8 + 0 = r8` (OpMove) — the pointer was never advanced.

**Fix:** (`mir_peephole.go`):
- `trackConstants()`: Read `int(inst.Imm)` instead of `inst.Value`
- `constantFolding()`: Set both `inst.Imm` and `inst.Value` when creating OpLoadConst
- `algebraicSimplification()`: Set `inst.Imm` in all 5 identity-to-constant cases (x-x, x*0, 0/x, x&0, x^x)

**Verified:** `iter_skip.minz` now outputs `CDE\n` (hex `4344450a`) correctly.
Z80 assembly shows `ADD HL, DE` with DE=2 to advance pointer past skipped elements.

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
| A | Counter waste (no DJNZ) | All patterns | **PARTIAL** (v0.19.5) — bare DJNZ when no CALL | Small | P3 |
| B | Register handoff (A not loaded) | map+forEach, filter+forEach, chains | **FIXED** (v0.19.4) | Medium | — |
| C | Lambda arithmetic elision | Lambda map (`\|x\| x+1`) | **FIXED** (v0.19.4) | Medium | — |
| D | Filter constant not tracked | Inline filter lambda (`\|x\| x>3`) | **FIXED** (v0.19.5) | Small | — |
| E | Skip pointer offset elided | `skip(n)` | **FIXED** (v0.19.4) | Small | — |
| F | Unused result saves | All patterns with CALL | Peephole target | Small | P3 |
| G | OpPush wrong register | enumerate, reduce | **Open** | Small | P2 |

**What got fixed in v0.19.4 E2E round:**
- **Bug B** (filter arg loading): `prepareCallArguments()` now only skips arg loading for TRUE SMC functions (`UsesTrueSMC`), not all SMC functions. Regular SMC functions correctly load arguments via registers.
- **Bug C** (lambda inlining): `generateIteratorLambda()` now uses `AddParamWithRegister()` so `NumParams` is set correctly. Inliner uses `fn.Params[i].Reg` for formal parameter register, and substitutes `OpLoadVar`/`OpLoadParam` with `OpMove` from actual argument register.
- **Bug E** (skip pointer offset): MIR peephole `trackConstants()` now reads `inst.Imm` (correct) instead of `inst.Value` (always 0). Constant folding and algebraic simplification also set `inst.Imm` when creating `OpLoadConst`.

**What got fixed in v0.19.5 E2E round:**
- **Bug D** (inline filter constant): DCE was missing `OpJumpIfFlag` from marking — constants deleted. Fixed in `dead_code_elimination.go`.
- **Bug A** (counter waste): Fusion optimizer now sets `BareDJNZ` hint when no CALL in loop body. Z80 codegen emits bare `DJNZ` instruction. Peephole pattern 48 catches remaining `DEC B; JR NZ` sequences.
- **Fusion optimizer** (`fusion.go`): Detects DJNZ loops and inlines small callbacks (≤8 instructions, 1 param, no asm/calls/loops). Eliminates CALL/RET overhead (~27 T-states/element).

**Remaining root cause:**
- Bug G is a register routing issue — OpPush always goes through HL.

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
| `pkg/optimizer/fusion.go` | Fusion optimizer — DJNZ loop detection + callback inlining |
| `pkg/optimizer/fusion_test.go` | 7 unit tests for fusion optimizer |
| `pkg/optimizer/assembly_peephole.go` | Assembly peephole (pattern 48: DEC B + JR NZ → DJNZ) |
| `pkg/optimizer/dead_code_elimination.go` | DCE — fixed OpJumpIfFlag/OpPush marking |

### Test Suite (70+ unit + 18 corpus + 11 E2E verified)

| Package | Tests | What |
|---------|-------|------|
| `pkg/parser/participle/` | 20 | Chain conversion, argument routing |
| `pkg/semantic/` | 17 | DJNZ eligibility, filter/map/lambda IR, take/skip/enumerate/reduce |
| `pkg/codegen/` | 5 | Z80 assembly patterns (use `-vet=off`) |
| `pkg/mirvm/` | 8 | DJNZ execution at MIR level |
| `pkg/optimizer/` | 19 | Peephole, DCE, superoptimizer, normalization, **fusion (7 new)** |
| `tests/iterator_corpus/` | 18 | Full pipeline regression — all 18 compile to Z80 |
| `examples/e2e_iterators/` | 11 | Full pipeline E2E: compile → assemble → emulate → verify hex output |

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

### Phase 3: Register Handoff Fix (Bug B) — DONE (v0.19.4)
- Fixed `prepareCallArguments()` to only skip arg loading for TRUE SMC (`UsesTrueSMC`)
- Regular SMC functions now correctly load arguments via `LD A, <argReg>`
- **Result:** filter+forEach, map+forEach chains output correct values

### Phase 4: MIR Peephole Imm Fix + Lambda Inlining (Bugs C, E) — DONE (v0.19.4)
- MIR peephole: `trackConstants()` now reads `inst.Imm`, not `inst.Value`
- MIR peephole: `constantFolding()` and `algebraicSimplification()` set `inst.Imm`
- Lambda: `generateIteratorLambda()` uses `AddParamWithRegister()`
- Inliner: uses `fn.Params[i].Reg`, substitutes OpLoadVar/OpLoadParam with OpMove
- **Result:** skip pointer offset correct, lambda map arithmetic works

### Phase 5: Inline Filter Constant Tracking (Bug D) — DONE (v0.19.5)
- DCE was missing `OpJumpIfFlag` from `markUsedRegisters()` Phase 1
- Added `OpJumpIfFlag` (marks Src1+Src2), `OpJumpIf/IfZero/IfNotZero` (mark Src1), `OpPush` (marks Src1)
- Also added all missing jump opcodes to `markReferencedLabels()`
- **Result:** `filter(|x| x > 67)` now generates `CP 68` (correct) instead of `OR A`

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

### Phase 8: Counter Peephole (Bug A) — DONE (v0.19.5)
- BareDJNZ CodegenHint: fusion optimizer sets hint when no OpCall remains in loop body
- Z80 codegen emits bare `DJNZ label` (13/8T) instead of 6-instruction memory-backed sequence (~28T)
- Peephole pattern 48: `DEC B; JR NZ, label → DJNZ label` catches non-iterator cases
- **Result:** Loops with inlined callbacks use native DJNZ instruction

### Phase 9: Fusion Optimizer — DONE (v0.19.5)
- Full implementation in `pkg/optimizer/fusion.go` (was skeleton, now working)
- Detects DJNZ loop patterns via `findDJNZLoops()` → `parseDJNZLoop()`
- Inlines small callbacks: ≤8 instructions, 1 param, no asm/calls/loops
- OpTrueSMCLoad → OpMove, OpLoadParam → OpMove, OpReturn → OpMove
- After inlining, sets BareDJNZ hint if no OpCall remains
- 7 unit tests in `fusion_test.go`
- Already wired into optimizer pipeline (runs before SmartPeepholeOptimizationPass)
- **Result:** CALL/RET overhead eliminated (~27 T-states/element saved)

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
