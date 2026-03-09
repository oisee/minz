# ADR-0016: Nanz Error Propagation — `-> T ! E` and `?` Operator

## Status
Accepted

## Context

Nanz programs running on Z80 need a way to signal and propagate errors from callees to callers. Two approaches exist in the Z80 ecosystem:

1. **Return value convention**: return a flag or code in a specific register and let the caller check it explicitly (like Go's `(val, err)` tuple return).
2. **Flag convention**: use the Carry flag (CY) as an error signal — callers test `JR C, err_handler` — a Z80 idiom used by CP/M BDOS and many ROM routines.

The difficulty with option 1 alone is that every call site must explicitly handle the tuple, and there's no lightweight "propagate immediately" operator. The difficulty with option 2 alone is that the caller has no structured way to receive a rich error code.

The user's requirement (from design discussion, 2026-03-09):
- CY flag is sufficient for propagation (`?` operator)
- Error *code* should be returned explicitly (Go-style), not forced into a fixed register
- Fixing the error code to register A is inconvenient (A holds the success value on the happy path)
- The mechanism should work without runtime overhead for the Z80 target

## Decision

### Syntax

```nanz
// Declaration: -> SuccessType ! ErrorType
fun safe_div(a: u8, b: u8) -> u8 ! u8 { ... }
fun open(path: ^u8)         -> u16 ! u8 { ... }
fun validate(x: u8)         -> void ! u8 { ... }  // void success, error possible

// Error return (inside a fallible function)
return_err 1          // sets CY, puts 1 in error register, returns

// Success return (inside a fallible function)
return result         // clears CY (AND A), puts result in return register, returns

// Propagation operator ? (try)
let q = safe_div(a, b)?   // if CY set: forward error_code, return; else: q = success_value
```

### Calling convention (Z80)

A fallible function has **three logical return slots** in its MIR2 contract:

| Slot | Content | Register | MIR2 class |
|------|---------|----------|------------|
| 0 | success value | A (or HL for u16) | `ClassAcc` |
| 1 | error flag | CY | `ClassFlag, FlagCond: CmpGe` (CY=1 → error) |
| 2 | error code | C | `ClassGeneral` |

On the **success path**: slot 0 is valid, slot 1 = CY clear, slot 2 undefined.
On the **error path**: slot 1 = CY set, slot 2 = error code, slot 0 undefined.

The caller branches on slot 1 (CY flag), then reads slot 0 or slot 2 depending on the branch.

Z80 code shape:
```asm
safe_div:
    ; A=a (param 0, ClassAcc), C=b (param 1, ClassGeneral)
    LD A, C
    AND A
    JR NZ, .ok
    LD C, 1          ; ERR_DIVZERO
    SCF              ; CY = 1 = error
    RET
.ok:
    ; ... compute division → A
    AND A            ; CY = 0 = success  (4T overhead — cheapest CY-clear)
    RET
```

Error propagation with `?`:
```asm
    CALL safe_div    ; A=result or C=err, CY=0/1
    JR NC, .ok
    SCF              ; re-assert CY (already set, but explicit)
    RET              ; C passes through unchanged — zero extra work
.ok:
    ; continue with A
```

**Error code passes through for free**: between `CALL` and the propagating `RET`, nothing touches C on the error path, so the error code flows upward without any extra `LD` instructions.

### Why C for error code, not A?

A holds the success value. If a function fails, A may hold partial computation or garbage — we must not use it for the error code lest the caller's success path reads garbage. C is the "second return register" by the existing calling convention (`ClassGeneral`, position 1), and it is never clobbered by `AND A` / `SCF` / `RET`. On the error path C is already in the right place.

The allocator is not hard-wired to C specifically — it assigns `ClassGeneral`, which may resolve to C, D, E, H, or L. In practice, position-1 args/returns naturally land in C.

### HIR changes

```go
// hir.Func — one new field:
ErrTy mir2.Ty  // non-nil iff function is fallible

// New HIR expression:
type TryExpr struct {
    Call *CallExpr
    Ty   mir2.Ty    // success type (same as Call's function RetTy)
}

// New HIR statement:
type ErrorReturnStmt struct {
    ErrVal Expr    // error code expression
}
```

### MIR2 changes

```go
// New terminator alongside TermRet:
type TermErrRet struct {
    ErrVal Reg    // error code (ClassGeneral → C)
}
// Z80 codegen: LD C, <errval>;  SCF;  RET
```

Success `TermRet` in a fallible function gets a `AND A` prepended by codegen (detected from `f.Contract.Returns` having a `ClassFlag` slot).

### Nanz parser changes

- New token `tokQuestion` (`?`) — single char, added to lexer
- Return type: `parseFunDecl` checks for `tokBang` after `->` → parse `ErrorType`, set `f.ErrTy`
- New statement keyword `return_err` in `parseStmt` → `ErrorReturnStmt`
- Postfix `?` in `parsePostfix`: valid only after `CallExpr` → `TryExpr`

### `?` expansion in HIR lowering

`let x = foo(args)?` expands to:
```
%ok, %cy, %err = call foo(args)          ; 3 vregs from 3 return slots
br_if %cy, @error_path, @success(%ok)

@error_path:
    err_ret %err                          ; TermErrRet: SCF + RET

@success(%x):
    ; continue — x = %ok
```

## Consequences

### Positive
- Zero-cost propagation: `?` on the error path costs 0 extra instructions (C passes through, SCF is 4T, RET is 10T — same as normal return)
- Go-style explicit error codes: callers that need to inspect the error do so explicitly via the error-code register
- CY flag is the universal error signal: compatible with CP/M BDOS, ROM routines, and existing Z80 idioms
- Works with the existing MIR2 multi-return contract system — no new IR types, just a new terminator
- `void ! u8` works: functions that either succeed (with nothing to return) or fail with a code

### Negative
- 4T overhead on every successful return path (`AND A` to clear CY)
- Slightly larger call sites: callers that use `?` emit an extra `JR NC` + `SCF;RET` sequence (~10 bytes, ~14T overhead on the success path — the `JR NC` jumps over on success)
- `bool` return type conflicts with `ClassFlag`: a `-> bool ! u8` function cannot use CY for both the bool value and the error signal; codegen must materialise the bool into A (0/1) instead

### Neutral
- Error type is `u8` (alias `error` optionally). 256 distinct codes is sufficient for Z80 programs. No rich error structs needed.
- The error register class (`ClassGeneral`) is assigned by the allocator — not hard-wired to C. The optimizer can reassign it if C is occupied.
- `return_err` is a statement keyword, not an expression form. This avoids interaction with the expression parser and keeps the `?` operator unambiguous.

## Alternatives Considered

### `-> (T, bool)` tuple return
Forces two registers on every return. Callers cannot use the value without unpacking. Propagation with `?` would still require the same branch sequence. No advantage on Z80.

### `-> T | error` union type
`|` is already a binary operator in Nanz (bitwise-or, precedence 1). Using it as a type-level union requires significant parser lookahead to disambiguate from expression context, or a completely separate type-expression grammar. Rejected due to grammar complexity.

### `-> Result<T, E>` / generic result type
Generics are explicitly rejected in this project (see CLAUDE.md).

### Dedicated error register (always A)
The success value is already in A. Returning the error code also in A would mean callers can't read the success value and error code simultaneously — the caller would need to save A before the branch. This is strictly worse than using a second register.

### Implicit `?` (Zig's `try`)
`try foo(args)` as a statement (no postfix `?`). Zig-style is fine but doesn't compose as an expression: `let x = foo(a)? + foo(b)?` is natural with postfix `?` but requires two statements with `try`. We prefer the Rust `?` syntax since it composes.

## References

- Design discussion: session 2026-03-09 (context: error propagation for Nanz)
- Sub-agent design report: `/private/tmp/.../tasks/a6ab92473edf8d1cc.output`
- ADR-0010: Register-First Calling Convention (ClassAcc, ClassGeneral, ClassFlag)
- ADR-0013: MIR2 Pipeline and Frontend Strategy
- Z80 CP/M BDOS convention: CY=0 success, CY=1 error
