# Report #071 — Arena Z80 Codegen: Invalid IXL/IXH Instructions (BUG-008)

**Date:** 2026-03-14
**Category:** Codegen / Register Allocator / Correctness
**Status:** 🔴 Blocking — Arena allocator cannot assemble via MZA on Z80 path

---

## Summary

The arena allocator (`08_arena_allocator.nanz`) passes all 11 asserts via MIR2 VM
but **cannot assemble** for Z80.  The generated assembly contains impossible
instructions that the Z80 CPU cannot encode.

| What | Detail |
|------|--------|
| **Invalid instructions** | `LD IXL, (IX+0)`, `LD IXH, (IX+1)` |
| **Why impossible** | DD prefix can't simultaneously remap dest (L→IXL) AND source ((HL)→(IX+d)) |
| **MZA response** | `unknown instruction or invalid operands: LD` — correctly rejected |
| **MIR2 VM tests** | All PASS (VM executes MIR2 directly, never touches Z80 codegen) |
| **Affected function** | `Arena_alloc` — struct field load via pointer when both ptr and dst allocated to IX |

---

## 1. The Bug

### Generated Assembly (from `ex20_arena_allocator.a80`, lines 37-38)

```z80
; fun Arena_alloc(self: ptr = HL, n: u16 = BC) -> u16 = HL
Arena_alloc:
    LD E, (HL)         ; result.lo = self.ptr.lo  ✅
    INC HL
    LD D, (HL)         ; result.hi = self.ptr.hi  ✅
    DEC HL
    LD H, D
    LD L, E
    ADD HL, BC         ; HL = next = result + n   ✅ (but self ptr LOST)
    PUSH HL
    POP BC             ; BC = next
    PUSH HL
    POP IX             ; IX = next
    INC IX
    INC IX             ; IX = &self.end (WRONG — IX = next+2, not self+2)
    LD IXL, (IX+0)     ; ❌ IMPOSSIBLE INSTRUCTION
    LD IXH, (IX+1)     ; ❌ IMPOSSIBLE INSTRUCTION
    LD H, IXH          ; undocumented but valid — moot since source is garbage
    LD L, IXL
    ...
```

### Why `LD IXL, (IX+d)` Cannot Exist

The Z80 DD prefix serves **one** purpose per instruction: remap the (HL) memory
operand to (IX+d).  The undocumented IXH/IXL register names use the **same** DD
prefix to remap H/L register names.

- `DD 6E dd` = `LD L, (IX+d)` — DD remaps source: (HL) → (IX+d), dest stays L
- `DD 26 nn` = `LD IXH, nn` — DD remaps dest: H → IXH, source is immediate
- `DD 6E dd` as `LD IXL, (IX+d)` — DD would need to remap **both** — impossible

There is exactly one DD prefix byte.  It can't do double duty.

### Even If It Existed: Self-Clobber Bug

```z80
LD IXL, (IX+0)     ; writing IXL changes IX itself!
LD IXH, (IX+1)     ; (IX+1) now reads from WRONG address
```

After the first load modifies IXL, the IX register value changes, so `(IX+1)` in
the second instruction computes from a corrupted base.  The correct pattern requires
scratch registers:

```z80
LD A, (IX+0)        ; scratch0 = lo  (IX still intact)
LD B, (IX+1)        ; scratch1 = hi  (IX still intact)
LD IXL, A           ; now safe to modify IX
LD IXH, B
```

### `LD H, IXH` / `LD IXH, H` — Also Invalid

The generated code also contains `LD H, IXH` and `LD L, IXL`.  These look valid
as undocumented instructions, but they are **not**:

- With DD prefix, H is **remapped** to IXH
- `DD 64` encodes `LD IXH, IXH` (a NOP), not `LD H, IXH`
- Similarly `DD 6D` = `LD IXL, IXL` (NOP), not `LD L, IXL`

MZA correctly rejects all four:

```
LD IXH, H  → rejected (DD: H becomes IXH → LD IXH, IXH)
LD IXL, L  → rejected (DD: L becomes IXL → LD IXL, IXL)
LD H, IXH  → rejected (same conflict)
LD L, IXL  → rejected (same conflict)
```

Valid undocumented IXH/IXL instructions require the **other** operand to be
A, B, C, D, or E — registers unaffected by the DD prefix:

```
LD IXH, A  ✅     LD A, IXH  ✅
LD IXL, B  ✅     LD C, IXL  ✅
LD IXH, H  ❌     LD H, IXH  ❌
```

### MZA Correctly Rejects

```
$ echo '    LD IXL, (IX+0)' > /tmp/test.a80
$ mza /tmp/test.a80
Assembly errors:
  line 1: unknown instruction or invalid operands: LD
```

The instruction table (`instruction_table.go:1265-1301`) correctly defines
`LD r, (IX+d)` only for r ∈ {A, B, C, D, E, H, L}.  No pattern exists for
IXH/IXL destinations.

---

## 2. Root Cause Analysis

### Three bugs compounding

**Bug A — Register allocator: no IX conflict constraint**

The PBQP allocator (`pbqp.go`) assigns `ClassIXY8` registers (IXH, IXL) as
destinations for loads from IX-indexed memory.  There is no constraint preventing
the combination where both the pointer register AND the destination register
are from the IX family with displacement addressing.

**Location:** `z80codegen.go:1662-1666`
```go
} else if isIXY(ptr) {
    g.emitf("    LD %s, %s", lowByte(dst), ptrIndirect(ptr, 0))  // lowByte(IX) = IXL
    g.emitf("    LD %s, %s", highByte(dst), ptrIndirect(ptr, 1)) // highByte(IX) = IXH
}
```

When `dst` is allocated to IX and `ptr` is also IX, this blindly emits the
impossible combination.

**Bug B — Self-pointer lost after ADD HL, BC**

After `ADD HL, BC` (computing `next = result + n`), HL no longer points to
`self`.  The codegen then tries to derive `self.end` from the result pointer
(`PUSH HL; POP IX; INC IX; INC IX`) — but `next + 2 ≠ self + 2`.  The `self`
base pointer is gone.

**Bug C — `LD H, IXH` context — valid instruction, wrong data**

`LD H, IXH` is a valid undocumented instruction (`DD 64`).  But even if Bug A
were fixed (loading into H/L instead of IXH/IXL), the data would still be wrong
because IX points to `next+2`, not `self+2` (Bug B).

### Why MIR2 VM doesn't catch it

The MIR2 VM executes MIR2 opcodes directly — `OpLoad`, `OpStore`, `OpPtrAdd` —
using Go memory.  It never invokes the Z80 code generator.  The `via mir2`
asserts test the *algorithm* (correct) but not the *Z80 code* (broken).

The `via z80` path would catch this — MZA would refuse to assemble.

---

## 3. Fix Options

### Option 1: Constraint in PBQP (proper fix)

Add an interference edge or infinite-cost entry in the PBQP cost matrix that
prevents `ClassIXY8` registers from being assigned as destinations when the
source operand uses IX/IY-indexed addressing.

**Pros:** Prevents the bug at allocation time.  Clean.
**Cons:** Requires understanding PBQP cost matrix construction.  ~Medium effort.

### Option 2: Codegen guard + spill to temp register

In `z80codegen.go`, when emitting `LD dst, (IX+d)` and `lowByte(dst)` is
IXL/IXH, use a temporary register (A) instead:

```go
if isIXYHalf(lowByte(dst)) && isIXY(ptr) {
    g.emitf("    LD A, %s", ptrIndirect(ptr, 0))
    g.emitf("    LD %s, A", lowByte(dst))   // LD IXL, A — valid undocumented
    g.emitf("    LD A, %s", ptrIndirect(ptr, 1))
    g.emitf("    LD %s, A", highByte(dst))  // LD IXH, A — valid undocumented
}
```

**Pros:** Quick fix, ~10 LOC.
**Cons:** Doesn't fix Bug B (self-pointer loss).  Bandaid.

### Option 3: Rewrite Arena_alloc codegen strategy

For struct pointer loads where `self` is needed after arithmetic, save `self`
to stack or alternate register before clobbering HL:

```z80
Arena_alloc:
    PUSH HL            ; save self
    LD E, (HL)         ; DE = self.ptr
    INC HL
    LD D, (HL)
    INC HL
    LD A, (HL)         ; A = self.end.lo
    INC HL
    LD H, (HL)         ; H = self.end.hi
    LD L, A            ; HL = self.end

    PUSH HL            ; save end
    LD H, D
    LD L, E            ; HL = result (old ptr)
    ADD HL, BC         ; HL = next
    LD B, H
    LD C, L            ; BC = next

    POP HL             ; HL = end
    OR A
    SBC HL, BC         ; end - next
    JR C, .oom

    POP HL             ; HL = self
    LD (HL), C         ; self.ptr = next
    INC HL
    LD (HL), B
    EX DE, HL          ; HL = result
    RET
.oom:
    POP HL
    LD HL, 0
    RET
```

~24 bytes, ~70T happy path.  No IX, no undocumented instructions.

**Pros:** Correct, efficient, no IX needed.
**Cons:** This is hand-written — the *codegen* needs to produce this pattern.

### Recommended: Option 1 + Option 2

Option 1 (PBQP constraint) prevents the class of bug.  Option 2 (codegen guard)
is a safety net for any edge cases the constraint misses.

---

## 4. Broader Impact — What Else Is Affected?

Any struct method that:
1. Takes `self: ^Struct` as pointer parameter (in HL)
2. Performs arithmetic that clobbers HL
3. Then accesses another struct field

Will hit the same pattern.  Arena.alloc is the first example because it does
`ptr + n` (clobbers HL) then reads `self.end` (needs HL back).

This will block any non-trivial struct method on Z80.  Priority: **blocking**.

---

## 5. Test Gap

| Path | Tests | Status |
|------|-------|--------|
| MIR2 VM (`via mir2`) | 11/11 PASS | Algorithm correct |
| Z80 (`via z80`) | NOT TESTED | Would fail at MZA assembly |
| Native (`mzn`) | Works via QBE/C99 | Unaffected (no Z80) |

**Recommendation:** Add `via z80` asserts for arena tests.  These would have
caught this bug immediately.

---

## Files Referenced

| File | Relevance |
|------|-----------|
| `pkg/mir2/z80codegen.go:1662-1666` | Codegen emits impossible instruction |
| `pkg/mir2/z80codegen.go:459-474` | `ptrIndirect()` generates `(IX+d)` |
| `pkg/mir2/z80codegen.go:4485-4516` | `lowByte()`/`highByte()` decompose pairs |
| `pkg/mir2/pbqp.go` | PBQP allocator — missing conflict constraint |
| `pkg/mir2/z80cost.go:34-38` | LocIXY8 defined as valid locations |
| `pkg/z80asm/instruction_table.go:1265-1301` | Valid `LD r, (IX+d)` patterns |
| `reports/showcase-src/2026-03-14/ex20_arena_allocator.a80:37-38` | The invalid instructions |
