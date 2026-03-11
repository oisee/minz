# A Byte's Journey: How `abs_diff` Survived Five Translations and Stayed Correct

*A dispatch from inside the compiler — with every intermediate representation shown*

**Date:** 2026-03-10
**Packages:** `pkg/mir2c`, `pkg/mir2qbe`, `pkg/qbe2mir2`, `pkg/nanz`, `pkg/pipeline`
**Test:** `go test ./pkg/mir2c/ -v -run TestRoundTrip`
**Result:** 4 functions × 4 paths — all agree

---

## Prologue: Trust, but Verify

Saying "all four paths return the same answer" is easy. Showing exactly what happens at each step is another matter. Let's trace what `abs_diff(10, 3)` goes through before it outputs `7`.

Source in Nanz:

```nanz
fun abs_diff(a: u8, b: u8) -> u8 {
    if a > b {
        return a - b
    } else {
        return b - a
    }
}
```

Four execution paths that must all agree on `7`:

| Path | Route | Tool |
|------|-------|------|
| 1 | Nanz → MIR2 → VM | `mir2.NewVM` (interpreter) |
| 2 | Nanz → MIR2 → C99 → native | `mir2c` + `cc` |
| 3 | Nanz → MIR2 → QBE IL → native | `mir2qbe` + `qbe` + `cc` |
| 4 | Nanz → MIR2 → QBE IL → MIR2 → C99 → native | `mir2qbe` + `qbe2mir2` + `mir2c` + `cc` |

Path 4 is the "crazy double translation" — its purpose will become clear at the end.

---

## Layer 1: MIR2 — the Internal SSA Representation

After passing through the Nanz parser, HIR synthesis, and five optimization passes
(ElimDeadBlocks → ReorderBlocks → PropagateConstants → FoldConstants → DeadStoreElim),
we get the real dump (`m.Dump()`):

```
; MIR2 module: abs_diff
fun @abs_diff(%r1: u8 [acc], %r2: u8 [general]) -> u8 [acc]
  block @entry:
    %r3 = cmp.gt %r1, %r2 : bool [flag]
    br_if %r3, @if_then1(), @if_else3()
  block @if_then1:
    %r4 = sub %r1, %r2 : u8 [general]
    ret %r4
  block @if_else3:
    %r5 = sub %r2, %r1 : u8 [general]
    ret %r5
```

Three things are worth noting.

**Types with register classes.** `u8 [acc]` means "8-bit, class `acc`" — this register will live
in Z80's accumulator A. `u8 [general]` means any of B, C, D, E, H, L. `bool [flag]` is not
a register at all — it's the Z80 F register, alive only until the next `br_if`.

**No phi nodes.** MIR2 uses block parameters in the spirit of MLIR/Swift SIL. Instead of
`phi @then %r4, @else %r5`, we have two independent `ret` instructions. This is an architectural
choice: phi nodes and block parameters are isomorphic, but parameters are simpler for the
register allocator.

**Five registers, zero redundant instructions.** PropagateConstants eliminated three intermediate
`copy` instructions that existed after HIR synthesis.

---

## Layer 2: QBE IL — "The Translator Was Surprised"

`mir2qbe.Compile(m)` translates MIR2 to [QBE](https://c9x.me/compile/) format.
Real output:

```
export function w $abs_diff(w %r1, w %r2) {
@entry
	%r3 =w csgtw %r1, %r2
	jnz %r3, @if_then1, @if_else3
@if_then1
	%r4 =w sub %r1, %r2
	ret %r4
@if_else3
	%r5 =w sub %r2, %r1
	ret %r5
}
```

**First information loss: type.** `u8` became `w`. QBE has four types: `w` (32-bit),
`l` (64-bit), `s` (float32), `d` (float64). No 8-bit integers — QBE targets modern 64-bit
architectures where the register file is always 32/64-bit. Our byte gets "widened" to a
32-bit word at the IL level.

**Second information loss: register classes.** `[acc]`, `[general]`, `[flag]` — all disappear.
QBE doesn't know about the Z80, and it doesn't need to. Every variable becomes a single-class `w`.

**However, the block structure is preserved perfectly.** Five registers, same CFG topology.
`csgtw` is "compare signed greater-than word" — a direct mapping of `cmp.gt`.

> **A latent bug exposed by this test suite earlier:** `csgtw` is a *signed* comparison,
> even though `u8` is unsigned. This doesn't manifest in the range `[0..127]`. For values
> `>127` there could be divergence. The test `abs_diff(200, 100)` passes because `200 − 100 = 100`
> is computed correctly in both branches regardless of comparison sign. Formally, this is a bug
> in `mir2qbe`. Good candidate for an ADR.

---

## Layer 3: MIR2 After the Round Trip — "Do You Recognise Yourself?"

`qbe2mir2.Parse(qbeIL)` reads QBE IL and reconstructs MIR2. Real dump:

```
; MIR2 module:
fun @abs_diff(%r1: u16 [general], %r2: u16 [general]) -> u16 [general]
  block @entry:
    %r3 = cmp.gt %r1, %r2 : bool [flag]
    br_if %r3, @if_then1(), @if_else3()
  block @if_then1:
    %r4 = sub %r1, %r2 : u16 [general]
    ret %r4
  block @if_else3:
    %r5 = sub %r2, %r1 : u16 [general]
    ret %r5
```

The function survived three translations and looks *almost* identical to the original.
Block structure, instructions, register names — all preserved. Two differences:

**`u8` became `u16`.** QBE's `w` (32-bit) maps to `u16` — the closest MIR2 type.
We cannot recover `u8`: that information was lost in Layer 2. The byte became a word.

**All classes became `[general]`.** Z80 register allocation hints are gone for good.
This module is semantically correct but "blind" to the Z80 allocator.

**The module name is empty** — QBE IL doesn't carry module names, only function names.

---

## Layer 4: C99 — Born Twice

From the round-tripped MIR2, `mir2c.Compile(m2)` generates C. Real output:

```c
uint16_t abs_diff(uint16_t r1, uint16_t r2) {
    uint8_t r3;
    uint16_t r4;
    uint16_t r5;

b_entry:;
    r3 = (uint8_t)(((int16_t)r1 > (int16_t)r2) ? 1 : 0);
    if (r3) {
        goto b_if_then1;
    } else {
        goto b_if_else3;
    }
b_if_then1:;
    r4 = (uint16_t)(r1 - r2);
    return r4;
b_if_else3:;
    r5 = (uint16_t)(r2 - r1);
    return r5;
}
```

Note that `r3` is `uint8_t` — `bool [flag]` in MIR2 maps to a single-byte flag.
The rest are `uint16_t`. Compare with the C generated *directly* from the original MIR2 (Path 2):

```c
uint8_t abs_diff(uint8_t r1, uint8_t r2) {
    uint8_t r3;
    uint8_t r4;
    uint8_t r5;
    // ... same structure, but everything is u8
}
```

The two functions are semantically identical with different types. The `main.c` harness
declares parameters as `int` — both variants correctly promote values on call.

---

## The Hard Case: `fib` and Phi Nodes in QBE

For the loop function, the picture is more interesting. The original MIR2 uses block parameters:

```
; MIR2 module: fib
fun @fib(%r1: u8 [acc]) -> u8 [acc]
  block @entry:
    %r2 = const 0 : u8 [general]
    %r3 = const 1 : u8 [general]
    %r4 = const 0 : u8 [general]
    jmp @loop_head1(%r2, %r3, %r4, %r1)
  block @loop_head1(%r5: u8, %r6: u8, %r7: u8, %r8: u8):
    %r9 = cmp.lt %r7, %r8 : bool [flag]
    br_if %r9, @loop_body2(), @loop_exit3(%r5, %r6, %r7, %r8)
  block @loop_body2:
    %r10 = add %r5, %r6 : u8   ← new b = a + b
    %r11 = const 1 : u8
    %r12 = add %r7, %r11 : u8  ← i + 1
    jmp @loop_head1(%r6, %r10, %r12, %r8)
  block @loop_exit3(%r13: u8, ...):
    ret %r13
```

Four loop variables: `a`, `b`, `i`, `n`. Passed explicitly as jump arguments.

QBE doesn't know block parameters — it uses **phi nodes**:

```
export function w $fib(w %r1) {
@entry
	%r2 =w copy 0
	%r3 =w copy 1
	%r4 =w copy 0
	jmp @loop_head1
@loop_head1
	%r5 =w phi @entry %r2, @loop_body2 %r6     ← a: 0 → b_prev
	%r6 =w phi @entry %r3, @loop_body2 %r10    ← b: 1 → a+b
	%r7 =w phi @entry %r4, @loop_body2 %r12    ← i: 0 → i+1
	%r8 =w phi @entry %r1, @loop_body2 %r8     ← n: n → n (self-referencing!)
	%r9 =w csltw %r7, %r8
	jnz %r9, @loop_body2, @loop_exit3
@loop_body2
	%r10 =w add %r5, %r6
	%r11 =w copy 1
	%r12 =w add %r7, %r11
	jmp @loop_head1
@loop_exit3
	%r13 =w phi @loop_head1 %r5
	...
	ret %r13
}
```

**The self-referencing phi.** `%r8 =w phi @entry %r1, @loop_body2 %r8` — the variable `n`
never changes, so on the back-edge it refers to itself. This is valid SSA.
`qbe2mir2` must correctly recognise this and produce a block parameter that
receives itself as the argument on the back-edge.

After the round trip:

```
; MIR2 module:
fun @fib(%r1: u16 [general]) -> u16 [general]
  block @entry:
    %r2 = const 0 : u16
    %r3 = const 1 : u16
    %r4 = const 0 : u16
    jmp @loop_head1(%r2, %r3, %r4, %r1)
  block @loop_head1(%r5: u16, %r6: u16, %r8: u16, %r10: u16):
    %r11 = cmp.lt %r8, %r10 : bool [flag]
    br_if %r11, @loop_body2(), @loop_exit3(%r5, %r6, %r8, %r10)
  block @loop_body2:
    %r7 = add %r5, %r6 : u16    ← a+b (was %r10, now %r7)
    %r12 = const 1 : u16
    %r9 = add %r8, %r12 : u16   ← i+1 (was %r12, now %r9)
    jmp @loop_head1(%r6, %r7, %r9, %r10)
  block @loop_exit3(%r13: u16, ...):
    ret %r13
```

**The structure is identical, but register names are shuffled.** Original: `jmp @loop_head1(%r6, %r10, %r12, %r8)`. After round-trip: `jmp @loop_head1(%r6, %r7, %r9, %r10)`. The same four values `(b, a+b, i+1, n)`, different names — because `qbe2mir2` assigns names to phi nodes in processing order, not in the order of the original SSA. It's an anagram: same letters, different arrangement.

---

## Layer 5: Z80 — Ground Truth

Path 5 isn't executed via emulator — it's only logged. What the Z80 backend actually
generates from the *original* MIR2 (without type loss through QBE):

```z80
abs_diff:
    CP C         ; compare A with C (sets flags)
    NEG          ; A = -A
    ADD A, C     ; A = C - A_orig  (= b - a)
    LD C, A      ; C = |a - b|
    RET CGT      ; if a > b — return now (A is still correct)
.abs_diff_if_then1:
    NEG          ; else: A = -(b - a) = a - b
    RET
```

Nine instructions. `RET CGT` is a conditional return on the "greater-than" flag — the result of
the `TermCondRet` peephole: when a return follows a comparison, no separate `JR`/`JP` is needed
because the `CP` flags are still live.

For `fib`, Z80 generates 22 instructions:

```z80
fib:
    LD D, 0 / LD E, 1 / LD H, 0          ; a=0, b=1, i=0
    LD A, D / LD D, E / LD E, H          ; parallel copy:
    LD H, C / LD C, A                    ; (a,b,i,n) → registers
.fib_loop_head1:
    LD A, E
    CP H
    JRS NC, .fib_loop_exit3              ; i >= n → exit
.fib_loop_body2:
    LD A, C
    ADD A, D / LD C, A                   ; b = a + b
    INC E                                ; i++
    LD A, D / LD D, C / LD C, A         ; parallel swap a ← b
    JRS .fib_loop_head1
.fib_loop_exit3:
    LD A, C / RET
```

The parallel copy `(a, b) ← (b, a+b)` requires a temporary on the Z80 — three
instructions through the accumulator.

---

## The Loss Map

| Property | Original MIR2 | After QBE | After qbe2mir2 |
|---|---|---|---|
| Type `u8` | ✅ preserved | ❌ → `w` (32b) | ❌ → `u16` |
| Class `[acc]` | ✅ A on Z80 | ❌ lost | ❌ all `[general]` |
| Class `[flag]` | ✅ F on Z80 | ❌ lost | ✅ reconstructed |
| Block parameters | ✅ native | → phi nodes | ✅ reconstructed |
| CFG topology | ✅ | ✅ | ✅ |
| Register names | original | same | shuffled |
| Module name | ✅ `fib` | — | ❌ empty |
| Computational correctness | ✅ | ✅ | ✅ |

**Code quality of the round-tripped module:** zero for Z80. Without register class hints
the allocator can't know that `a` should live in A. For the C99 target it doesn't matter —
`cc` handles register allocation.

---

## Epilogue: Why Does This Path Even Exist?

Path 4 exists not for performance but as a **crash test for `qbe2mir2`**.
If the QBE→MIR2 parser makes an error reconstructing phi nodes or block parameters,
the result will diverge from Path 1 (MIR2 VM). A mismatch between paths immediately
localises the bug:

| Divergence | Root cause |
|------------|-----------|
| Paths 1 and 2 differ | bug in `mir2c` |
| Paths 2 and 4 differ | bug in `qbe2mir2` |
| Path 3 differs from all | bug in `mir2qbe` (or in the `qbe` tool) |

This is exactly how a bug in `mir2qbe` was found: all `CmpUlt` comparisons were emitting
`ceqw` (equals) instead of `cultw` (unsigned less-than). Path 1 gave the correct answer
through the VM; Path 3 gave wrong output through QBE-native. The bug was localised to one
package in a single test run.

Four paths. One answer: `abs_diff(10, 3) = 7`.

---

## Appendix: Running the Tests

```bash
# All round-trip tests (requires cc in PATH; qbe optional)
go test ./pkg/mir2c/ -v -run TestRoundTrip -count=1

# Watch intermediate representations for one case
go test ./pkg/mir2c/ -v -run "TestRoundTrip_Fib/fib\(\[8\]\)" -count=1

# Full test suite
go test ./pkg/... -vet=off
```

The tests dynamically detect which tools are available (`qbe`, `cc`) and skip
paths whose dependencies are missing.

---

<details>
<summary>🇷🇺 На русском</summary>

→ [Жизнь байта: как `abs_diff` пережила пять трансляций и осталась собой](./2026-03-10-052-Round_Trip_Deep_Dive_RU.md)

</details>
