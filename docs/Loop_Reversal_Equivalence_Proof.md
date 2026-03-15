# Loop Reversal Equivalence Proof for DJNZ Optimization

**Status:** Proposal
**Date:** 2026-03-15
**Relates to:** Register allocator, DJNZ codegen, BUG-001 (PBQP spills)

---

## Motivation

Z80's `DJNZ` instruction (Decrement B, Jump if Not Zero) is the most efficient
loop primitive: 13 T-states taken, 8 not-taken, single 2-byte instruction.
But it counts **down** (B = n → 0), while most source loops count **up**
(`for i in 0..n`).

If we can prove that reversing iteration order produces identical results,
the compiler can emit DJNZ freely — eliminating a separate loop counter and
saving a register for actual work.

---

## Formal Framework

### Definitions

For a loop `for i in 0..n { body(i) }`, define:

- **E(i)** — the *effect* of iteration `i` (set of memory reads and writes)
- **W(i)** — the *write-set* of iteration `i` (addresses written)
- **R(i)** — the *read-set* of iteration `i` (addresses read, excluding loop variable)

The loop's total effect is the sequential composition:

```
Total = E(0) ∘ E(1) ∘ ... ∘ E(n-1)
```

### Commutativity of Effects

Two effects E(i) and E(j) **commute** (E(i) ∘ E(j) = E(j) ∘ E(i)) when:

1. **Disjoint writes:** W(i) ∩ W(j) = ∅, AND
2. **No read-write conflict:** W(i) ∩ R(j) = ∅ AND W(j) ∩ R(i) = ∅

Or the weaker condition:

3. **Same-value writes:** If W(i) ∩ W(j) ≠ ∅, both write the same value
   to the overlapping address.

### Theorem: Loop Reversal Equivalence

**If all pairs (E(i), E(j)) commute for i ≠ j, then any permutation of
iterations produces the same final memory state.**

*Proof:* Any permutation can be decomposed into a sequence of adjacent
transpositions. Each adjacent transposition swaps E(k) ∘ E(k+1) →
E(k+1) ∘ E(k), which is valid because they commute. By induction on
the number of transpositions, the final state is identical. ∎

**Corollary:** Forward iteration (0 → n-1) and reverse iteration
(n-1 → 0) produce the same result when all iteration effects commute.

---

## Sufficient Conditions (Static Checks)

The compiler can prove reversibility without runtime analysis by checking
these **sufficient conditions** on the loop body AST:

### Condition 1: Independent Indexed Stores

```nanz
for i in 0..n {
    array[i] = expr    // expr does NOT read array[0..n]
}
```

- W(i) = {&array[i]} — each iteration writes to a unique address
- R(i) ∩ W(j) = ∅ — expr doesn't read from array (or reads only outside 0..n)
- **Reversible: YES** ✅

Example:
```nanz
for i in 0..200 { board[i] = 0 }
// Reverse: for B in 200..0 { board[199 - B] = 0 }  — same result
```

### Condition 2: Commutative Accumulator

```nanz
for i in 0..n {
    acc = acc ⊕ f(i)    // ⊕ is commutative + associative
}
```

- Commutative operators: `+`, `*`, `&`, `|`, `^`, `min`, `max`
- Non-commutative: `-`, `/`, `%`, string concatenation
- **Reversible: YES** (for commutative ⊕) ✅

Example:
```nanz
for i in 0..n { sum = sum + board[i] }     // ✅ + is commutative
for i in 0..n { result = result - board[i] } // ❌ - is not commutative
```

### Condition 3: Pure Side-Effect Functions

```nanz
for i in 0..n {
    zx_poke(base + i, value)    // each call targets unique address
}
```

- If `base + i` is injective over 0..n (which it is), writes are disjoint
- **Reversible: YES** ✅

### Condition 4: Loop-Carried Dependency (BLOCKS reversal)

```nanz
for i in 1..n {
    a[i] = a[i - 1] + 1    // E(i) reads W(i-1)
}
```

- R(i) ∩ W(i-1) = {&a[i-1]} ≠ ∅
- **Reversible: NO** ❌

---

## Decision Procedure (Compiler Algorithm)

```
function canReverse(loop: ForLoop) -> bool:
    body = loop.body
    idx  = loop.indexVar

    // 1. Collect all array writes: array[f(idx)] = expr
    for each write W in body:
        // Check index expression is injective (monotonic function of idx)
        if W.index is not a function of idx alone:
            return false  // can't prove disjointness

    // 2. Check no read-write conflicts
    for each read R in body:
        if R.array == any W.array:
            if R.index could overlap W.index for different idx values:
                return false  // potential loop-carried dependency

    // 3. Check accumulator operations are commutative
    for each accumulator update A in body:
        if A.operator not in {+, *, &, |, ^}:
            return false

    return true
```

### Why Nanz Makes This Easier Than C

- **No pointers** → no aliasing problem (`a[i]` and `b[j]` can't alias unless `a == b`)
- **No hidden mutation** → function calls are visible in the AST
- **Explicit array indexing** → index expressions are analyzable
- **Value types** → no reference semantics to track

In C, `a[i] = b[i]` could alias if `a` and `b` overlap. In Nanz, arrays
are distinct unless they're literally the same global — trivially checkable.

---

## DJNZ Codegen Patterns

### Pattern A: Index Not Used (trivial)

```nanz
// Source:
for i in 0..n { body_without_i() }

// Z80:
    LD B, n
.loop:
    ; ... body ...
    DJNZ .loop
```

No index register needed. B is the counter. **Already implemented.**

### Pattern B: Index Used for Array Access (reversed walk)

```nanz
// Source:
for i in 0..n { array[i] = value }

// Z80 (reversed, HL walks backward):
    LD HL, array + n - 1
    LD B, n
.loop:
    LD (HL), value
    DEC HL
    DJNZ .loop
```

Two registers: B = counter, HL = address. No spills needed.

### Pattern C: Index Used as Argument (compute from B)

```nanz
// Source:
for i in 0..n { f(i) }

// Z80:
    LD B, n
.loop:
    ; compute A = n - B (the forward index)
    LD A, n
    SUB B
    ; use A as argument to f
    CALL f
    DJNZ .loop
```

Adds `LD A,n / SUB B` (2 instructions, 11 T-states). Worth it only if
the alternative is spilling B to memory (~40+ T-states per iteration).

### Pattern D: Nested Loop (outer in C, inner in B)

```nanz
// Source:
for y in 0..20 {
    for x in 0..10 {
        board_set(x, y, 0)
    }
}

// Z80:
    LD C, 20         ; outer counter (C or memory)
.outer:
    LD B, 10         ; inner counter (B for DJNZ)
.inner:
    ; ... body ...
    DJNZ .inner
    DEC C
    JR NZ, .outer
```

---

## Application to Tetris

| Loop (tetris_v2.nanz) | Reversible? | Reason | DJNZ Benefit |
|------------------------|-------------|--------|--------------|
| `for i in 0..200 { board[i] = 0 }` | ✅ | Independent stores | HL walk + DJNZ (but n>255, needs u16 counter) |
| `for c in 0..4 { piece_dx(t,r,c) }` | ✅ | Pure function, disjoint effects | B=4, compute c=4-B |
| `for line in 0..8 { fill pixel }` | ✅ | Independent stores (different screen addr) | HL walk + DJNZ |
| `for y in 0..20 { for x in 0..10 }` | ✅ | Disjoint attr writes | Nested C/B |
| `for py in 0..4 { for px in 0..4 }` | ✅ | Disjoint attr writes | Nested C/B |
| `for flash in 0..20 { border }` | ✅ | No memory effects (I/O only) | B=20, DJNZ |
| `for cy in 2..22 { fill_cell }` | ✅ | Disjoint screen writes | B=20, compute cy=22-B |
| `clear_lines` inner `for sx in 0..10` | ❌ | Reads board[sx, sy-1] while writing board[sx, sy] — different rows but same array | Needs deeper analysis |

**Note:** `for i in 0..200` needs special handling since 200 < 256 fits in B,
but just barely. For n > 255 (like screen init 0x4000..0x5800 = 6144 iterations),
DJNZ can't be used directly — need an outer/inner loop or BC 16-bit counter.

---

## Implementation Roadmap

1. **Phase 1:** Add `canReverse()` analysis pass to MIR2 (static check on loop body)
2. **Phase 2:** Emit DJNZ for reversed loops when B is available
3. **Phase 3:** Pattern-match array walks into HL+DJNZ (Pattern B)
4. **Phase 4:** Nested loop register allocation (B for inner, C for outer)

**Prerequisite:** BUG-001 (PBQP register allocator) must be fixed first.
Currently B gets spilled to memory ($F0xx), making DJNZ pointless.

---

## References

- Kennedy & Allen, "Optimizing Compilers for Modern Architectures" — dependence analysis
- Wolfe, "High Performance Compilers for Parallel Computing" — loop transformations
- Z80 User Manual — DJNZ instruction (opcode 0x10, 13/8 T-states)
- ADR-0006, ADR-0007 — Register allocator known issues
