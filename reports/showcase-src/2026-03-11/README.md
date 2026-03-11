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

## Multiple Return Values

Not yet in Nanz parser. `-> (u16, u16)` syntax gives parse error.
MIR2 itself supports `Contract.Returns[]` slice (multiple returns).
Nanz TODO: tuple return syntax + lowering.

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
