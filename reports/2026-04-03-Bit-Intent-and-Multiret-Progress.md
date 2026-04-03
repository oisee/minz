# Bit Intent and Multi-Return Progress

Date: 2026-04-03
Author: Codex

## Summary

This pass turned the recent bit-selector and multi-return work into something more coherent across language, MIR2, Z80 codegen, tests, and user-facing examples.

The main result is that bit access is no longer just surface syntax. In the active pipeline it now survives long enough to become meaningful backend intent, and the backend can choose Z80-native instructions instead of reconstructing everything from generic masking and shifting.

## What Landed

### 1. Scalar Bit Selector Syntax

Nanz now accepts:

- `x.7`
- `ptr^.4`
- `arr[i].12`
- `obj.field.3` when `field` is scalar

and rejects aggregate bases such as `point.3`.

Write semantics are intentionally strict:

- reading `expr.N` is allowed for scalar expressions
- writing `expr.N = v` is allowed only when the base is an addressable scalar lvalue

So `(128).7` is valid as a read, but `(128).7 = 0` is not.

### 2. Direct MIR2 Bit Intent

Lowering now produces dedicated MIR2 operations for explicit source-level bit intent:

- `bit_get`
- `bit_set`
- `bit_reset`

This is better than immediately decomposing everything into `shr/and/or`, because the Z80 backend can still see what the user meant.

### 3. Z80-Native Bit Lowering

The active MIR2 Z80 backend now recognizes and emits:

- `SET` / `RES` for memory-backed bit stores
- `BIT` for memory-backed bit tests
- `SET` / `RES` for register-backed bit updates where that is the right direct path
- `u16` high/low byte cases for both memory-backed and register-pair-backed values

The backend also preserves support for older arithmetic-shaped patterns as fallback, so existing code that still lowers through masks can benefit too.

### 4. IX/IY Halves as First-Class Registers

`IXH/IXL/IYH/IYL` are now treated as first-class 8-bit registers in the backend model, not as special cases used only when convenient.

That matters for:

- bit instructions
- direct moves
- compares
- bytewise access to `u16` values in `IX`/`IY`

Fallback through `A`, pair shuffles, or memory round-trips should happen only for correctness reasons or proven cost wins.

### 5. User-Facing Examples

New runnable playground examples were added:

- `fun/bit_intent.nanz`
- `fun/tuple_return.nanz`
- `fun/triple_return_skip.nanz`

These are meant to be small, direct examples of the newly-stabilized language paths.

## Why This Matters

The important change is not just syntax sugar. It is preservation of intent.

Once the compiler knows that the user asked for:

- a bit test
- a bit set
- a bit reset
- a structured multi-return unpack

it can lower with more target awareness and fewer accidental pessimisers.

That gives three concrete wins:

1. Source becomes cleaner.
2. Backend decisions become more architecture-aware.
3. The examples and docs now show supported forms directly instead of forcing users to reverse-engineer them from tests.

## Verification

Focused parser and MIR2 tests were run during the work, including:

- bit-selector parsing and lowering
- direct MIR2 bit ops
- memory-backed and register-backed bit fusion
- multi-return parse coverage including triple return and blank identifier skipping

The new playground examples were also compiled via the current CLI with `--asserts mir2`.

## Current Gap

There is one important follow-up still open:

- direct MIR2 `bit_get/bit_set/bit_reset` intent is understood by MIR2 Z80 codegen
- but VIR lowering does not yet understand those MIR2 ops directly

So today the examples still compile successfully, but the default path reports a VIR-lowering miss and falls back for those functions.

That does not block the user-visible feature, but it is the next correctness-and-quality step if the goal is full parity across the current default backend stack.

## Next Steps

The next logical layer is to extend this same “intent survives lowering” approach to more hardware-shaped ideas:

- counted loops that map cleanly to `DJNZ`
- carry-aware wider arithmetic
- block copy/scan intent that can choose `LDIR`/`CPIR`
- alternate register set usage where it is semantically justified

The direction is now clearer: preserve intent earlier, decide hardware shape later.
