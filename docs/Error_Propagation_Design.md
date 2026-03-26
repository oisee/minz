# Error Propagation Design: CY Flag + A Register

**Status:** Design phase. Reviewed by GPT-5.4, VIR (pending), Alice.

---

## Philosophy

Z80's Carry Flag (CY) is the **native error signal**. ROM routines use it. CP/M BDOS uses it.
`SCF` (Set Carry Flag) costs 1 byte, 4 T-states. `RET C` (conditional return on carry) costs
1 byte, 5/11 T-states. This is the cheapest possible error mechanism on Z80.

Nanz error propagation should be:
- **Zero-cost on success path** — `OR A` (clear CY) is already needed for many returns
- **Minimal on error path** — `SCF / LD A, code / RET` = 4 bytes, 15 T-states
- **Compiler-enforced** — caller MUST handle or propagate, can't silently ignore
- **Two styles** — untyped (just a number) or typed (enum), both compile identically

---

## Syntax

### Function Declaration

```nanz
// Untyped error — error code is a raw u8 number
fun safe_div(a: u8, b: u8) -> u8 ! {
    if b == 0 { @error(1) }
    return a / b
}

// Typed error — enum provides names at compile time
enum MathErr { DivByZero = 1, Overflow = 2 }

fun safe_add(a: u8, b: u8) -> u8 ! MathErr {
    var sum: u16 = u16(a) + u16(b)
    if sum > 255 { @error(MathErr.Overflow) }
    return a + b
}

// Void function that can error
fun init_device() -> void ! {
    if not_ready() { @error(1) }
    // ... init ...
}
```

### Call Site

```nanz
// MUST handle — compiler error if you don't check
var x: u8 = safe_div(10, y) ! { return }              // propagate to caller
var x: u8 = safe_div(10, y) ! { @print("err") }       // handle inline
var x: u8 = safe_div(10, y) !!                         // panic/halt on error (debug)

// Propagation shorthand (caller must also be !)
fun compute(a: u8, b: u8) -> u8 ! {
    var x: u8 = safe_div(a, b) !                       // propagate: RET C
    return x + 1
}
```

### Nesting

```nanz
// Inner error propagates before outer call executes
var z: u8 = safe_add(safe_div(10, x) !, 5) !

// Semantics:
// 1. Call safe_div(10, x) → check CY → if error, propagate immediately
// 2. Call safe_add(result, 5) → check CY → if error, propagate
```

---

## Z80 Codegen

### Error Return

```nanz
@error(N)
```

Compiles to:

```z80
    SCF             ; CY = 1 (error)
    LD A, N         ; A = error code
    RET             ; return to caller
```

4 bytes, 15 T-states. If N=0, optimize to `SCF / XOR A / RET` (but N=0 should be reserved for "no error").

### Success Return

```nanz
return value
```

Compiles to:

```z80
    ; ... value already in A (u8) or HL (u16) ...
    OR A            ; CY = 0 (success), also tests A for zero
    RET
```

2 bytes, 8 T-states. `OR A` is the cheapest way to clear CY. Side effect: sets Z flag if A=0 (harmless).

### Call + Handle

```nanz
var x: u8 = safe_div(10, y) ! { @print("err") return }
```

Compiles to:

```z80
    LD A, 10        ; first param
    LD C, <y>       ; second param (PFCCO)
    CALL safe_div
    JR C, .handle   ; CY=1 → error path
    ; A = result, use it
    ...
    JR .continue
.handle:
    ; A = error code
    LD HL, _str_err
    CALL abap_write_str
    RET
.continue:
```

### Call + Propagate

```nanz
var x: u8 = safe_div(a, b) !     // in a function that also returns !
```

Compiles to:

```z80
    CALL safe_div
    RET C           ; 1 byte! If CY=1, return immediately with CY+A intact
    ; A = result, continue
```

**`RET C` is the killer feature.** One byte, conditional return on carry. The Z80 was DESIGNED for this pattern. CY and A propagate through the return chain without any extra code.

### Call + Panic (debug)

```nanz
var x: u8 = safe_div(10, 0) !!   // halt on error
```

Compiles to:

```z80
    CALL safe_div
    JR NC, .ok      ; success → skip
    DI              ; disable interrupts
    HALT            ; freeze (debug: error code in A)
.ok:
```

---

## Register Convention

| Path | CY | A | HL | Notes |
|------|-----|---|----|-------|
| Success (u8) | 0 | return value | undefined | `OR A / RET` |
| Success (u16) | 0 | undefined | return value | clear CY before RET |
| Success (void) | 0 | undefined | undefined | `OR A / RET` |
| Error | 1 | error code (u8) | undefined | `SCF / LD A, N / RET` |

**A is dual-purpose:** return value on success, error code on error. Caller checks CY first to know which interpretation applies.

**HL undefined on error:** for u16-returning functions, HL has no meaningful value on the error path. Caller must check CY before using HL.

---

## Critical Constraints

### 1. No CY-clobbering between CALL and check

These instructions modify CY and MUST NOT appear between a fallible CALL and JR C:
`ADD, ADC, SUB, SBC, AND, OR, XOR, CP, NEG, CCF, SCF, RLCA, RRCA, RLA, RRA`

These are SAFE between CALL and JR C:
`LD, PUSH, POP, JP, JR, CALL, RET, NOP, EX, EXX, DI, EI, HALT, IN, OUT`

**The codegen must guarantee this.** Z3/PBQP/LIR must not insert arithmetic between a fallible CALL and the CY check.

### 2. RET C requires clean stack

`RET C` returns from the CALLER, so SP must equal the function's entry SP at that point. If the caller has PUSH'd registers:

```z80
    PUSH IX          ; save register
    CALL safe_div
    RET C            ; BUG! SP is wrong — returns to PUSH IX's caller
```

Fix: POP before RET C, or don't PUSH before fallible calls:

```z80
    PUSH IX
    CALL safe_div
    JR NC, .ok
    POP IX           ; clean up stack
    RET              ; then propagate (CY still set from safe_div)
.ok:
    POP IX
```

**Alternative:** Don't use `RET C` shorthand when stack is dirty. Emit `JR C, .cleanup` instead.

### 3. Nested fallible calls

```nanz
var z: u8 = safe_add(safe_div(10, x) !, 5) !
```

The inner `safe_div(10, x) !` must be evaluated FIRST. If it errors, `safe_add` is never called. The compiler must sequence:

```z80
    ; Evaluate safe_div(10, x)
    LD A, 10
    LD C, <x>
    CALL safe_div
    RET C             ; propagate inner error
    ; A = result of safe_div
    ; Now call safe_add(A, 5)
    LD C, 5
    CALL safe_add
    RET C             ; propagate outer error
    ; A = final result
```

### 4. Interrupts

Interrupt handlers may clobber CY. If interrupts are enabled, the CY flag between CALL and JR C could be corrupted. Solutions:
- Document: "disable interrupts during error-sensitive sections"
- Or: compiler emits `DI` before fallible CALL, `EI` after check (expensive: 2 extra bytes per call)
- Or: accept the risk (interrupts are rare in most Z80 programs, and ISRs that clobber CY should PUSH AF / POP AF)

**Recommendation:** Accept the risk. ISRs should preserve flags (PUSH AF / POP AF is standard practice). Don't add DI/EI overhead.

---

## Comparison

| Language | Mechanism | Cost (error path) | Cost (success) | Compiler-enforced? |
|----------|-----------|-------------------|----------------|-------------------|
| **Nanz** | CY + A | 4 bytes, 15T | 2 bytes, 8T | Yes (! required) |
| Rust | Result<T,E> | ~20 bytes (match) | 0 (optimized away) | Yes (? required) |
| Go | if err != nil | ~10 bytes | 0 | No (convention) |
| Zig | try/catch | ~6 bytes | 0 | Yes (try required) |
| C | errno | 0 | 0 | No |
| **SDCC** | manual CY check | varies | 0 | No |

Nanz is the **cheapest typed error mechanism** on Z80. Rust's Result is zero-cost on modern CPUs but expensive on Z80 (enum tag dispatch = memory access). Nanz uses the CPU's own flag register — literally free hardware support.

---

## Implementation Plan

| Step | Component | LOC | Effort |
|------|-----------|-----|--------|
| 1 | Parser: `! ErrorType` in return, `@error(N)`, `!` at call | ~80 | 2 hours |
| 2 | AST: `FallibleFunc` flag, `ErrorReturn` node, `ErrorCheck` node | ~30 | 30 min |
| 3 | Semantic: enforce `!` at call sites of fallible funcs | ~50 | 1 hour |
| 4 | HIR: `ErrorReturn` stmt → `SCF / LD A, N / RET` | ~20 | 30 min |
| 5 | HIR: `ErrorCheck` → `JR C, .handler` or `RET C` | ~30 | 30 min |
| 6 | HIR: success return adds `OR A` before RET for `!` funcs | ~10 | 15 min |
| 7 | Tests: 5 examples covering all patterns | ~100 | 1 hour |
| **Total** | | **~320** | **~6 hours** |

---

## Open Questions

1. **Syntax: `@error(N)` vs `return error N`?** — GPT-5.4 recommends `@error(N)` (more distinct). Alice prefers? Both compile identically.

2. **Auto-propagation syntax: `!` vs `try` vs `?`?** — `!` is concise but overloaded (also used for unwrap in some languages). `try` is explicit. `?` matches Rust convention.

3. **Should `!!` (panic) exist?** — Useful for prototyping. Could be debug-only (`@if DEBUG`).

4. **RET C shorthand: always or only when stack is clean?** — Compiler can determine stack state. If PUSH'd → emit JR C + cleanup. If clean → emit RET C.

5. **Error code 0: reserved for "no error" or valid?** — Recommend: 0 = no error (matches CY=0 → A=0 convention). Error codes start at 1.
