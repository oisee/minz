# Nanz Showcase — 2026-03-11

## Today's Additions

### ex9a — Factorial (recursive)

```nanz
fun factorial(n: u8) -> u16 {
    if n <= 1 { return 1 }
    return n * factorial(n - 1)
}
```

**Status**: compiles, but codegen has TODO for `u8 * u16` (n * recursive_result).
The `n` is u8 in A, result of recursive call is u16 in HL.
Cross-width multiply needs an explicit `MUL u8→u16` lifting pass.

### ex9b — Factorial (fold over range, idiomatic)

```nanz
global acc: u16 = 1
fun factorial_fold(n: u8) -> u16 {
    acc = 1
    for i in 1..n+1 { acc = acc * i }
    return acc
}
```

**Status**: compiles and runs correctly. Uses 16-bit software mul loop (~320T/iter).
Iterator chain over range collapses to DJNZ mul loop. Global `acc` used as accumulator.

### ex10a — Fibonacci (recursive)

```nanz
fun fib(n: u8) -> u16 {
    if n <= 1 { return n }
    return fib(n - 1) + fib(n - 2)
}
```

**Status**: compiles but codegen can't add two u16 recursive returns (`? + ?`).
Both calls return HL; second call clobbers HL from first.
Needs caller-save of first result (PUSH HL before second CALL).

### ex10b — Fibonacci (iterative, optimal)

```nanz
fun fib_iter(n: u8) -> u16 {
    if n <= 1 { return n }
    let a: u16 = 0
    let b: u16 = 1
    for i in 1..n {
        let tmp: u16 = a + b
        a = b
        b = tmp
    }
    return b
}
```

**Status**: compiles, mostly correct loop structure. Minor register init bloat (redundant moves).
`ADD DE, HL` emitted — invalid Z80, should be `ADD HL, DE` with swap. Known 16-bit add direction issue.

### ex11 — Multiple Return Values (minmax / swap)

```nanz
fun minmax(a: u16, b: u16) -> (u16, u16) {
    if a <= b { return (a, b) }
    return (b, a)
}

fun smaller(x: u16, y: u16) -> u16 {
    let (lo, _) = minmax(x, y)  // _ discards hi — zero cost
    return lo
}
```

**Status**: ✅ WORKS. New feature landed today.

Generated Z80 for `swap(a: u16, b: u16) -> (u16, u16)`:
```z80
swap:
    EX DE, HL      ; swap a↔b in one instruction
    RET            ; HL=b (first return), DE=a (second return)
```
2 instructions, 3 bytes, 14T — optimal.

Z80 ABI for multi-return:
- Position 0 → HL (ClassPointer for u16)
- Position 1 → DE (ClassIndex)
- Position 2 → B  (ClassCounter)

**`_` blank identifier**: Discarded positions are DSE-killed — no register save or propagation needed. `smaller` compiles to `CALL minmax / RET` (hi in DE simply ignored).

**Known limitation**: Allocator does not model call-clobbered registers. Complex callers with live-across-call values in DE may get incorrect allocation. Simple use-immediately patterns work correctly.

## Known Gaps Exposed Today

| Issue | Root cause | Fix |
|-------|-----------|-----|
| `u8 * u16` cross-width mul | Nanz lowers `n * fib_result` as same-type, mismatches | Type coercion in lowerBinExpr |
| `? + ?` recursive return add | Both calls use HL; second clobbers first | Caller-save pass: PUSH HL around 2nd call |
| `ADD DE, HL` invalid | genBinOp16 ADD: when lhs≠HL, emits wrong direction | Always route through HL as accumulator |
| Redundant init moves in fib_iter | u16 `let a=0; let b=1` spills through u8 path | u16 const emit directly to pair reg |

## Key Wins from Today

- `SCF+CCF → OR A` (-1 byte, -4T per 16-bit sub)
- `sub(const_0, r)` correct via CondRetSink fix (no more carry clobber)
- `SBC DE, HL` invalid case fixed via EX DE,HL trick
- Integer literal widening: `0 - r:u16` now emits `const 0 : u16` not u8
- **Multiple return values** (`-> (u16, u16)`, `return (e1, e2)`, `let (a, _) = fn(...)`) — full pipeline working
  - `swap` → `EX DE,HL / RET` (3 bytes, 14T, optimal)
  - `_` blank identifier → zero-cost (DSE removes unused ExtraRet)
  - End-to-end: HIR.RetTys → MIR2.ExtraRets → Z80 physOverride → correct register binding
