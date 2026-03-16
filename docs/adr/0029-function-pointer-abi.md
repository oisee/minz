# ADR-0029: Function Pointer ABI & Multi-Frontend Support

**Status:** Draft
**Date:** 2026-03-16
**Context:** Function pointers work in C89 (MIR2 VM only). Z80 codegen needs a standard ABI.

---

## Problem

PFCCO (Per-Function Calling Convention Optimization) assigns registers per-function
based on PBQP allocation. Direct calls work: the caller knows the callee's contract
and emits parallel copies to match.

But **indirect calls** (`CALL __call_hl`) don't know the callee at compile time.
The caller must place arguments in **agreed-upon** registers. The callee must
read arguments from those same registers.

Currently OpCallIndirect says "args are already in place" — a hack that only works
when the preceding HIR lowering happens to put them in the right spots.

## Decision: Standard Callable ABI

Functions whose address may be taken conform to a **fixed** calling convention:

```
┌──────────┬───────────┬───────────┐
│ Position │ u8 / bool │ u16 / ptr │
├──────────┼───────────┼───────────┤
│ param 0  │ A         │ HL        │
│ param 1  │ C         │ DE        │
│ param 2  │ B         │ BC        │
│ return   │ A         │ HL        │
└──────────┴───────────┴───────────┘
Clobbers: A, F (minimum). Callee may clobber more but must document.
```

This matches `classForParam()` exactly — the standard ABI IS the default class
assignment. The key change: functions callable through pointers must have their
Contract params **pinned** to these locations (not reassigned by PBQP).

### Detection: Which functions get standard ABI?

**Address-taken analysis** (whole-module, automatic):
1. Scan all OpAddrOf instructions in the module
2. If `OpAddrOf("foo")` exists and `foo` is a function → mark `foo` as address-taken
3. Address-taken functions get standard ABI (Contract params pinned)
4. Non-address-taken functions keep PFCCO freedom

**Explicit annotation** (future, optional):
- Nanz: `@callable fun handler(x: u8) -> u8 { ... }`
- C89: Implicit — any function assigned to a pointer is address-taken

### PFCCO coexistence

Direct calls to address-taken functions can still use PFCCO! The caller knows
the callee's Contract (which IS the standard ABI) and emits parallel copies.
No performance loss for direct calls.

Indirect calls use `emitCallArgs` with a synthetic Contract based on the standard ABI.

## Frontend Syntax

### Nanz
```nanz
// Function type declaration
type Transform = fn(u8) -> u8
type Reducer = fn(u8, u8) -> u8
type Action = fn(u8) -> void

// Usage as parameter
fun apply(f: Transform, x: u8) -> u8 {
    return f(x)
}

// Usage as variable
fun main() {
    let f: Transform = double
    let result = f(5)

    // Or pass directly
    let r2 = apply(|x| x * 2, 10)
}
```

Implementation: `type X = fn(...)` creates a `FuncTy` in the type system.
Variables of `FuncTy` lower to `TyPtr` (function address). Calls through
such variables lower to `CallIndirectExpr`.

### Lizp
```lizp
;; Functions are first-class — natural fit
(defun apply (f x) (funcall f x))
(defun double (x) (* x 2))

;; Lambda
(apply (lambda (x) (* x 3)) 5)

;; Higher-order
(defun map (f lst)
  (if (null lst) nil
    (cons (funcall f (car lst))
          (map f (cdr lst)))))
```

Lizp already has lambda support; `funcall` maps to OpCallIndirect.

### Pascal
```pascal
type Transform = function(x: byte): byte;

function Apply(f: Transform; x: byte): byte;
begin
    Apply := f(x);
end;
```

### C89 (already done)
```c
int (*fp)(int, int) = add;
int result = fp(3, 4);
```

## Implementation Plan

### Phase 1: Standard ABI enforcement
1. Add `FuncTy` to MIR2 type system (param types + return type)
2. Address-taken detection in pipeline (scan OpAddrOf for function symbols)
3. Pin Contract params for address-taken functions
4. `genCall(OpCallIndirect)`: build synthetic Contract, use `emitCallArgs`

### Phase 2: Nanz fn type syntax
1. Parser: `type X = fn(T1, T2) -> R`
2. Semantic: resolve FuncTy, check call signatures
3. HIR: CallIndirectExpr for fn-typed variable calls

### Phase 3: Lizp funcall
1. `(funcall f args...)` → CallIndirectExpr
2. Lambda → address-taken anonymous function

## Z80 Codegen Detail

For `OpCallIndirect` with standard ABI:

```asm
; f(x, y) where f: fn(u8, u8) -> u8
; x is in whatever reg PBQP chose, y same
; Standard ABI: param0=A, param1=C

    LD A, <x_phys>      ; parallel copy: move x → A
    LD C, <y_phys>      ; parallel copy: move y → C
    LD HL, <f_phys>     ; function pointer → HL
    CALL __call_hl      ; JP (HL)
    ; return value in A

__call_hl:
    JP (HL)             ; 4T trampoline
```

Note: HL is clobbered by the function pointer itself. If param0 is u16 (needs HL),
the function pointer must be saved to IX first:

```asm
; f(ptr) where f: fn(ptr) -> u8, ptr needs HL
    PUSH <f_phys>       ; save fn pointer
    POP IX              ; fn ptr → IX
    LD HL, <ptr_phys>   ; param0 → HL
    ; Can't use CALL __call_hl (HL is param, not fn ptr)
    ; Use CALL __call_ix instead
    CALL __call_ix      ; JP (IX)

__call_ix:
    JP (IX)             ; 8T (DD prefix)
```
