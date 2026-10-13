# ADR-0045: Target-width raw pointer semantics

Status: VM target-width wrap implemented; 16-bit Z80 boundary conformance tested.
Scalar frontend pointee types are now preserved; see [source conformance](../grace-typed-pointers.md).
Target data-layout selection and C/QBE memory-model migration remain open.
See [implementation evidence](../grace-gap-closure.md).
Date: 2026-10-10.

## Problem

Nanz documents indexing as `(p+i)^` and Z80 pointer addition as `ADD HL,DE`
(`Nanz_Language_Book_v7.md`). MIR2 TyPtr is 16-bit by default, but VM pointer
instructions store int64 sums while moves and block/function arguments truncate.
An optimization changing copies or CFG can therefore change observed addresses.
Z80 wraps arithmetic and multi-byte accesses; VM accesses contiguous heap offsets.

## Target-memory contract

For target-address raw pointers of width W:

1. Pointer values are unsigned W-bit byte addresses. Arithmetic wraps modulo
   2^W at the producing instruction, independently of moves, calls and CFG.
2. Raw memory accesses are little endian. Each successive byte address wraps
   independently at the target address-space boundary.
3. VM availability/bounds diagnostics are a separate sandbox policy. A partial
   heap may reject an unavailable byte; enlarging it does not widen pointer types.
4. Raw pointers have no object provenance. One-past arithmetic is ordinary
   address arithmetic; slice/object checks require a separate language feature.
5. SoA256 low-byte advance is an explicit layout operation, not ordinary −1
   arithmetic. Resolve its current encoding across VM, Z80, C and QBE.

This best fits existing Z80 intent and avoids adding implicit trapping checks to
native arithmetic. A checked-pointer alternative would require a trap ABI and
native checks. UB would require explicit preconditions; it cannot merely excuse
inconsistent implementation behavior.

## Implementation gate

- Test pointer arithmetic observed directly, through move/cast, block edges,
  arguments and returns. All paths must agree.
- Cross-check VM and Z80 u8/u16/u32 accesses around 65535 with distinct sentinel
  bytes at both ends; preflight stores to avoid partial-write errors in VM.
- Separate SoA256 `0x12FF→0x1200` from ordinary +1 `→0x1300`.
- Specify target pointer width selection for non-Z80 and host-pointer C/QBE
  backends; do not reinterpret host addresses as 16-bit offsets silently.
- Re-run Grace differential tests with an independent wrap oracle. Remove the
  HIR guard only after conformance passes; keep index-overflow proof requirements
  for MIR threading (u8 index wrap is independent of pointer wrap).
- Re-measure code bytes/T-states and compilation overhead after guard removal.

The VM now implements items 1–3 for typed MIR pointers (16/24-bit tests).
The legacy SoA −1 encoding matches Z80 in VM; C/QBE remain unresolved.
Retain HIR guards pending expanded conformance and profitability measurements.
