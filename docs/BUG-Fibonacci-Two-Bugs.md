# BUG: Fibonacci — Two Independent Bugs

**Status:** Open
**Found:** 2026-03-21
**Affects:** Any function with mutable variables in a while loop + swap pattern

---

## Bug 1: Assert VM Argument Passing for Mutable-Variable Functions

### Symptom

```nanz
fun fib(n: u8) -> u8 {
    var a: u8 = 0
    var b: u8 = 1
    var i: u8 = 0
    while i < n { ... }
    return a
}

assert fib(2) == 1 via mir2    // ❌ FAILS: got 2, want 1
```

But with a wrapper:

```nanz
fun f2() -> u8 { return fib(2) }
assert f2() == 1 via mir2      // ✅ PASSES
```

### Root Cause

`pipeline.RunAsserts` compiles the assert target function and calls it
on a fresh VM with the argument value. When the function has mutable
variables (`var a`, `var b`) initialized in the entry block, the VM
or assert harness interferes with the initial values.

The wrapper `f2()` takes no arguments, calls `fib(2)` internally,
and the VM handles this correctly because the argument flows through
normal calling convention.

### Verified: MIR2 IR is Correct

```
fun @fib(%r1: u8 [acc]) -> u8 [acc]
  block @entry:
    %r2 = const 0 : u8       ; a = 0
    %r3 = const 1 : u8       ; b = 1
    %r4 = const 0 : u8       ; i = 0
    jmp @loop_head1(%r2, %r3, %r4, %r1)
  block @loop_head1(%r5, %r6, %r7, %r8):
    %r9 = cmp.ult %r7, %r8   ; i < n
    cond_ret %r9, [%r5]      ; return a
  block @loop_body2:
    %r10 = add %r5, %r6      ; temp = a + b
    %r12 = add %r7, 1        ; i + 1
    jmp @loop_head1(%r6, %r10, %r12, %r8)  ; a←b, b←temp, i++
```

The back-edge correctly maps: `(a, b, i, n) ← (b, a+b, i+1, n)`.

### Workaround

Wrap assert targets in zero-argument functions:

```nanz
fun f0() -> u8 { return fib(0) }
fun f5() -> u8 { return fib(5) }
assert f0() == 0 via mir2  // ✅
assert f5() == 5 via mir2  // ✅
```

### Where to Look

- `pkg/pipeline/pipeline.go` — `RunAsserts` function
- `pkg/mir2/vm.go` — `Call()` argument setup
- Check how entry block params interact with `var` initializers

---

## Bug 2: Z80 Regalloc — Two Variables in Same Register

### Symptom

```asm
fib_v1:
    LD A, 0       ; a = 0
    LD A, 1       ; b = 1 — OVERWRITES a! Both in A!
    LD D, 0       ; i = 0
.loop:
    ...
    ADD A, A      ; a + a (NOT a + b!)
```

Output: 0, 1, 2, 4, 8, 16, 32... (powers of 2, not Fibonacci).

### Root Cause

PBQP allocator assigns both `a` (u8) and `b` (u8) to register A.
With only 7 physical Z80 registers (A, B, C, D, E, H, L) and 4 live
variables (a, b, i, n), the allocator should use 4 different registers.

The back-edge parallel copy `(a, b, i, n) ← (b, a+b, i+1, n)` requires
`a` and `b` to be in DIFFERENT registers because `a` is read while `b`
is being written (swap pattern).

PBQP's interference graph should mark `a` and `b` as interfering
(both live at the same point), but it fails to do so — possibly because
`cond_ret` returns `a` which collapses the ClassAcc preference for both.

### Affected Programs

- Fibonacci (any variant with a/b swap)
- Any loop with `a = b; b = f(a_old)` pattern
- ABAP fibonacci.abap, fizzbuzz.abap (MOD uses similar pattern)

### Not Affected

- Programs without mutable variable swaps
- Programs where swap variables have different types (u8 vs u16)
- `via mir2` asserts (VM handles variables correctly, through wrappers)

### Where to Look

- `pkg/mir2/pbqp.go` — interference graph construction
- `pkg/mir2/alloc.go` — register allocation
- `pkg/mir2/z80codegen.go` — `emitParallelCopy`
- Check if block params of `loop_head` are marked as simultaneously live

### Test Case

```nanz
// examples/tests/fib_parallel_copy.nanz
fun fib(n: u8) -> u8 {
    var a: u8 = 0
    var b: u8 = 1
    var i: u8 = 0
    while i < n {
        let temp: u8 = a + b
        a = b
        b = temp
        i = i + 1
    }
    return a
}

assert fib(0) == 0 via z80   // ✅ passes (no loop iteration)
assert fib(1) == 1 via z80   // ✅ passes (1 iteration, a=1 correct by accident)
assert fib(2) == 1 via z80   // ❌ FAILS: got 2 (a+a=2, not fib)
```

---

## Summary

| Bug | Layer | Symptom | Workaround |
|-----|-------|---------|------------|
| #1 Assert args | pipeline/VM | `assert fib(2)==1 via mir2` fails | Use wrapper: `fun f2() -> u8 { return fib(2) }` |
| #2 Regalloc swap | Z80 codegen | `a` and `b` both in register A | None — needs PBQP fix |

Both bugs produce the same visible effect (wrong Fibonacci numbers) but are
completely independent. Bug #1 is in the test infrastructure, Bug #2 is in
the register allocator.
