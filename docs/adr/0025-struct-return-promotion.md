# ADR-0025: Struct-Return Promotion to Tuple

## Status
Proposed (2026-03-15)

## Context

C uses `struct` as the sole mechanism for returning multiple values from a function. On Z80 this is a performance catastrophe: SDCC implements struct return via caller-allocated stack memory, each field requiring a separate load after return.

Nanz natively supports `-> (T1, T2)` multi-return with PFCCO optimization. PFCCO assigns each return value an optimal register based on call graph analysis — no stack, no loads, 0T overhead for result transfer.

The C frontend must recognize struct-return patterns and promote them to HIR tuple returns, so PFCCO can apply its full power.

## The Problem

```c
// Typical C pattern — returning multiple values
typedef struct { uint8_t q; uint8_t r; } DivResult;

DivResult divmod(uint8_t a, uint8_t b) {
    return (DivResult){ a / b, a % b };
}

DivResult res = divmod(17, 5);
uint8_t q = res.q;
uint8_t r = res.r;
```

**SDCC codegen:**
```asm
; caller setup
LD HL, _result_buf   ; pointer to caller-allocated struct
PUSH HL              ; pass as hidden argument
LD A, 17
LD C, 5
CALL _divmod
POP BC               ; remove hidden argument
; load results
LD HL, _result_buf
LD A, (HL)           ; q
INC HL
LD B, (HL)           ; r
; ~60T overhead just for result transfer
```

**Desired Nanz/PFCCO codegen:**
```asm
LD A, 17
LD C, 5
CALL _divmod         ; A=q, C=r — contract assigned by PFCCO
; 0T overhead — result already in correct registers
```

## Decision

### Struct-Return Promotion Pass

A new analysis pass (`StructReturnAnalysis`) detects C functions returning small structs and promotes them to HIR tuple returns when safe.

### Promotion eligibility conditions

A function `f` returning `struct S` is promotion-eligible if:

**1. Shallow Scalar Struct (SSS)**
- All fields are scalar types (u8, u16, i8, i16, bool)
- No nested structs, no arrays
- Size ≤ available register capacity (Z80: ≤ 4 bytes = 4×u8 or 2×u16)

**2. All call sites use immediate destructuring**

For each call site `res = f(...)`:
- All uses of `res` are field reads only (`res.q`, `res.r`)
- `res` is not passed to another function as a whole struct
- `res` is not stored in a global or another struct

**3. Non-recursive function** (v1 simplification)

### Detection algorithm

```
Pass: StructReturnAnalysis
Input: cc.AST (from modernc.org/cc/v4)
Output: map[FuncName]PromotionDecision

For each function F returning struct S:
  1. Check SSS(S) → if no: SKIP
  2. Collect all call sites CS = {c | c calls F}
  3. For each c in CS:
     a. Find result variable: res = c
     b. Collect uses(res)
     c. For each use u:
        - If u = res.field → OK, record field in used_fields
        - If u = &res or u = &res.field → ADDRESSABLE(field)
        - If u = pass_to_func(res) → NOT_ELIGIBLE, break
        - If u = store_to_mem(res) → NOT_ELIGIBLE, break
  4. If NOT_ELIGIBLE: SKIP
  5. Else: PROMOTE(F, used_fields, addressable_fields)
```

### HIR emission for promoted function

```c
// C:
DivResult divmod(uint8_t a, uint8_t b) {
    return (DivResult){ a / b, a % b };
}
```

```nanz
// HIR (after promotion):
fun divmod(a: u8, b: u8) -> (u8, u8) {
    return (a / b, a % b)
}
```

### HIR emission for call sites

**Case 1: No addressable fields (pure tuple)**

```c
DivResult res = divmod(17, 5);
uint8_t q = res.q;
uint8_t r = res.r;
```

```nanz
// HIR:
let (q, r) = divmod(17, 5)
```

PFCCO assigns `(q → A, r → B)` or any optimal contract. 0T overhead.

**Case 2: Partially addressable fields**

```c
DivResult res = divmod(17, 5);
uint8_t *pq = &res.q;   // field q is addressable
uint8_t r = res.r;       // field r — just a read
```

```nanz
// HIR — spill only addressable fields:
let (q_reg, r) = divmod(17, 5)   // tuple return
var q: u8 = q_reg                 // materialize q to memory
let pq: ^u8 = @addr(q)            // now address is valid
```

Compiler inserts `var q` (stack slot) only for `q`, `r` stays in register. Spill happens **after** CALL, not before.

**Case 3: All fields addressable**

Promotion is still beneficial — CALL without stack argument, spill after:

```nanz
let (q_reg, r_reg) = divmod(17, 5)
var q: u8 = q_reg    // 7T spill
var r: u8 = r_reg    // 7T spill
```

vs SDCC: hidden pointer + caller-allocated buffer = ~25T overhead before CALL.

### Address-of Materialization

Key insight: registers have no address, but spilling to a stack slot creates an addressable copy. Spill happens **after** CALL — PFCCO contract is already fulfilled.

```
Temporally:
CALL divmod     ; ← PFCCO contract: result in registers
                ; ← decision point (statically known)
[spill if address needed]
[use]
```

Analogous to NRVO/RVO in C++ and materialization of temporaries in Rust.

| Case | SDCC overhead | After promotion |
|---|---|---|
| No &res | ~60T (stack return) | 0T |
| &res.q for one field | ~60T | 7T (one spill) |
| &res for all fields | ~60T | 14T (two spills) |

Even in the worst case (all fields addressable) — 14T vs 60T. 4x win.

### Reverse pattern: Struct-argument demotion

Symmetric case — struct passed as argument and immediately destructured:

```c
void draw_point(Point p) { plot(p.x, p.y); }
draw_point((Point){10, 20});
```
→
```nanz
fun draw_point(x: u8, y: u8) { plot(x, y) }
draw_point(10, 20)
```

Same logic: if struct-argument is immediately destructured in body → demote to separate parameters. PFCCO assigns optimal registers for `(x, y)` based on what `plot` does.

### Promotion boundaries

**Not promotion-eligible:**

```c
DivResult results[10];
results[i] = divmod(a, b);          // struct lives in memory array

void log_result(DivResult r);
log_result(divmod(a, b));           // passed whole (unless log_result also promoted)

DivResult recursive_divmod(uint8_t a, uint8_t b) {
    return recursive_divmod(b, a % b);  // recursive (v1 limitation)
}

typedef struct { u8 a, b, c, d, e; } Big;  // >4 bytes, won't fit registers
```

**Cross-module calls:** if caller and callee in same module → promotion. If across `@extern` boundary → keep C ABI.

## Consequences

### Positive
- 0T overhead for multi-value returns (vs ~60T with SDCC)
- PFCCO applies full inter-procedural optimization to C code
- Common patterns (divmod, minmax, sincos, coordinate pairs) benefit most
- No HIR changes needed — tuple return, destructure, VarDecl all exist

### Negative
- Analysis pass complexity (~200 LOC use-def analysis)
- Addressable fields require spill logic (but still 4x better)
- Recursive functions excluded in v1

### Neutral
- Only affects C frontend — Nanz/Lanz/Pascal already use tuple returns natively
- Whole new code is in C frontend analysis pass, no backend changes

## Effort Estimate

| Component | Estimate | Complexity |
|---|---|---|
| StructReturnAnalysis (~200 LOC) | 1 day | Medium — use-def analysis |
| Address-of materialization in lowering | 0.5 day | Simple |
| Struct-argument demotion (reverse) | 0.5 day | Simple |
| Tests (divmod, minmax, swap, coordinate pairs) | 0.5 day | Low |
| **Total** | **~2-3 days** | |

Dependency: requires C89 frontend (ADR-0024).

## References

- ADR-0024: C89 Frontend Strategy
- ADR-0020: PBQP EdgeCost — promotion increases PFCCO effectiveness
- BUG-001: Affinity edges — after fix, promotion gives maximum benefit
- [SDCC struct return convention](https://sdcc.sourceforge.net/doc/sdccman.pdf)
