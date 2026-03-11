# ADR-0018: Z80 ABI Annotations and Recursive Function Conventions

**Status**: Accepted
**Date**: 2026-03-11
**Context**: Nanz/MIR2 Z80 backend

---

## Context

The MIR2 Z80 codegen emits raw assembly with minimal documentation. As programs grow
more complex (recursive functions, multi-return, caller-save discipline), developers
and AI tooling need to understand the generated code's calling convention at a glance.

Three related issues prompted this ADR:

1. **Readability**: Generated `.a80` has no per-function documentation.
2. **Recursive functions**: Recursive calls clobber live registers without saving them,
   causing incorrect results when live values span a CALL boundary.
3. **Clobber-save discipline**: The ABI does not yet define caller-save vs callee-save
   conventions; clobbered registers are not automatically preserved.

---

## Decision

### 1. ABI Annotation Comments

Every generated function now begins with a structured comment:

```z80
; fun name(param0: type = REG, ...) -> ret: type = REG [recursive] ; clobbers: REG, ...
name:
    ...
```

**Fields:**
| Field | Description |
|-------|-------------|
| `param: type = REG` | Param name, HIR type, allocated physical register |
| `-> type = REG` | Return type and canonical register (from ReturnClass) |
| `-> (type=REG, ...)` | Multi-return: all positions listed |
| `[recursive]` | Present when the function contains a direct self-call |
| `clobbers: REG, ...` | Sorted list of phys registers written by the function body (excluding params and returns) |

**Examples:**
```z80
; fun add_u8(acc: u8 = A, x: u8 = C) -> u8 = A
add_u8:
    ADD A, C
    LD C, A
    RET

; fun gcd(a: u16 = HL, b: u16 = DE) -> u16 = HL [recursive] ; clobbers: B, C, F
gcd:
    ...

; fun minmax(a: u16 = HL, b: u16 = DE) -> (u16 = HL, u16 = DE)
minmax:
    EX DE, HL
    RET
```

**Implementation**: `funcAnnotation(f *Func, ar *AllocResult)` in `z80codegen.go`.
- Param registers: from `ar.Loc(param.Reg)` (post-allocation physical location).
- Return registers: from `classToRegName(return.Class, return.Ty)` — canonical register
  for the class. Note: contract optimisation may change classes before codegen runs,
  so this is a best-effort annotation (always structurally correct, sometimes approximate).
- Clobbers: all instruction `Dst` phys locs not in param/return set.
- Recursive: any `OpCall` with `inst.Sym == f.Name`.

---

### 2. Recursive Function Conventions

**Current limitation**: The allocator does not model call-clobbered registers.
A recursive function that keeps values in registers across its own `CALL` will get
incorrect results if those registers are written by the callee (itself).

**Established conventions** (to be enforced by future passes):

#### Register allocation for recursive functions
- **Prefer spill-to-stack over memory-backed virtuals**: Recursive functions must
  save/restore live registers around their self-calls. This is done explicitly via
  `PUSH`/`POP` in the generated code.
- **Parameter count limit**: A recursive function cannot have more live-across-call
  values than available caller-save registers. If this limit is exceeded, the compiler
  MUST emit a diagnostic and refuse to compile (rather than silently generating wrong code).

#### Z80 caller-save / callee-save split

| Register | Convention |
|----------|-----------|
| `A`, `F` | Caller-save: caller must preserve if needed across any CALL |
| `BC` | Caller-save: B used as DJNZ counter; BC pair is caller-save |
| `DE` | Caller-save: commonly used as secondary pointer / index |
| `HL` | Caller-save: primary pointer / 16-bit result |
| `IX`, `IY` | Callee-save: expensive to save/restore (4 bytes each); functions must not clobber unless they save/restore |
| Shadow `A'`, `BC'`, `DE'`, `HL'` | Reserved: used by specific passes (DWord, SMC); do not clobber |

#### Recursive function requirements
1. All live values across a recursive CALL must be in caller-save registers **and**
   explicitly `PUSH`ed before the `CALL` and `POP`ed after.
2. Function parameters passed in registers are clobbered by the recursive call —
   the caller (the function itself) must save them if they're needed post-call.
3. For simple tail-recursive patterns, the compiler should transform to iteration
   (DJNZ loop) where possible — no register save needed.

**Status**: Detection (`[recursive]` annotation) is implemented. PUSH/POP insertion
is NOT YET IMPLEMENTED. Recursive functions with live-across-call registers may
produce incorrect results. This is tracked as a known limitation.

---

### 3. Clobber List in Annotations

The `; clobbers:` list in the annotation serves as the ABI contract for callers.
A future **caller-save insertion pass** can use this information to automatically
`PUSH`/`POP` live registers around function calls.

Example of what the future pass would do:
```z80
; Caller with live HL before CALL to fn that clobbers HL:
    PUSH HL          ; inserted by caller-save pass
    CALL gcd
    POP HL           ; inserted by caller-save pass
```

---

## Consequences

### Positive
- Every function is self-documenting in the `.a80` output
- `[recursive]` flag immediately alerts developers to potential correctness issues
- `clobbers:` list lays the groundwork for automatic caller-save insertion
- Multi-return ABI is explicit in the annotation

### Negative / Limitations
- Annotation is best-effort for return registers (contract optimisation may change
  classes before annotation runs)
- Recursive functions are NOT yet automatically correct — developer must manually
  ensure live values are saved
- Clobber list is based on static IR analysis; runtime-conditional paths are
  conservatively included (may over-report)

---

## Future Work

- **ADR-0019**: Caller-save insertion pass — automatically `PUSH`/`POP` caller-save
  registers around `CALL` instructions based on liveness and clobber annotations
- **Recursive detection improvements**: Detect mutual recursion (A→B→A)
- **Tail-call optimization**: Transform direct tail-recursive calls to jumps (no stack growth)
- **Stack frame annotation**: For functions that use `IX` as a frame pointer, emit
  frame layout as a comment
