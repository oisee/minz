# ObjC Simple Example — Editorial Review

**Date:** 2026-04-06
**Scope:** Inspect `examples/objc/simple.m` + `simple.a80` — source intent vs emitted asm vs ABI comments.
**Requested by:** minz-vir session (hz8hkfc8)

---

## 1. Source Intent

`simple.m` declares a minimal ObjC class `Box` with one ivar (`value`) and two methods:

```objc
-(int)get       { return self->value; }
-(int)addN:(int)n { return self->value + n; }
```

Plus a trivial C function `identity(int x) { return x; }` and 4 assert-objc tests. The intent is to demonstrate zero-cost ObjC method dispatch on Z80: struct field access via pointer, addition, direct CALL.

---

## 2. Assembly Quality Assessment

### `Box_get` — return self->value (field at offset 0)

```z80
Box_get:
    LD A, (HL)      ; load low byte of value
    INC HL
    LD H, (HL)      ; load high byte
    LD L, A         ; HL = value
    LD D, H         ; ← redundant: copy HL→DE
    LD E, L         ; ← redundant: copy HL→DE
    RET
```

**Verdict: 2 redundant instructions.** The `LD D,H / LD E,L` copies HL to DE for no reason — the return convention is `ret=HL` per the ABI comment. This is a VIR solver artifact: it solved return in DE then moved to HL, but didn't eliminate the dead DE copy. Optimal would be 4 instructions (LD A,(HL) / INC HL / LD H,(HL) / LD L,A / RET = 5 ops), so this is 7 ops — 40% overhead.

**Optimal (5 ops, 30 bytes → 22 bytes):**
```z80
Box_get:
    LD A, (HL)
    INC HL
    LD H, (HL)
    LD L, A
    RET
```

### `Box_addN` — return self->value + n

```z80
Box_addN:
    LD B, H         ; ← redundant: save HL to BC
    LD C, L         ; ← redundant
    LD H, B         ; ← redundant: restore HL from BC (no-op round-trip!)
    LD L, C         ; ← redundant
    LD A, (HL)      ; load low byte of value
    INC HL
    LD H, (HL)      ; load high byte
    LD L, A         ; HL = value
    ADD HL, DE      ; value + n
    RET
```

**Verdict: 4 redundant instructions.** The `LD B,H / LD C,L / LD H,B / LD L,C` sequence is a complete no-op — it copies HL→BC→HL. This is a solver artifact from save-before-overwrite logic that wasn't cleaned up by peephole. The function is 10 ops; optimal is 6.

**Optimal (6 ops):**
```z80
Box_addN:
    LD A, (HL)
    INC HL
    LD H, (HL)
    LD L, A
    ADD HL, DE
    RET
```

### `identity` — return x

```z80
identity:
    LD A, L         ; ← wrong: truncates u16 to u8
    RET
```

**Verdict: Incorrect.** The ABI comment says `params=[BC] ret=HL` and `x: u16 = HL → ret u16 = HL`. If x arrives in HL and returns in HL, the function should be a single `RET`. Instead it does `LD A, L` which truncates to 8 bits and leaves A (not HL) as the "result". This is either a type confusion (treating int as u8) or a return-register mismatch.

**Optimal (1 op):**
```z80
identity:
    RET
```

### `__objc_test_2` — test harness for `Box{value:10}.addN(5)`

```z80
__objc_test_2:
    LD DE, 2        ; base address of Box struct
    LD BC, 10       ; value = 10
    LD A, C
    LD (DE), A      ; store low byte
    INC DE
    LD A, B
    LD (DE), A      ; store high byte
    DEC DE
    LD A, (DE)      ; ← re-reads the byte just written (redundant)
    INC HL          ; ← BUG: should be INC DE (HL not set to anything meaningful)
    LD H, (HL)      ; ← BUG: reads from garbage HL address
    LD L, A
    LD BC, 5
    ADD HL, BC
    RET
```

**Verdict: Contains a bug.** After `DEC DE`, DE points back to the struct base. The code then does `LD A, (DE)` (re-reads the low byte — redundant but ok). But then `INC HL` — HL was never set to the struct address! It should be `INC DE` or the struct base should be in HL. The `LD H, (HL)` reads from an uninitialized HL pointer. This test harness would produce wrong results on real Z80 hardware.

The test probably passes in MIR2 VM because the VM tracks virtual registers differently, but the Z80 codegen is broken for this inlined test.

---

## 3. ABI Comment Accuracy

### Header comments:
```
; Box_get: params=[BC] ret=HL (Z3-PFCCO)
; Box_addN: params=[BC,HL] ret=HL (Z3-PFCCO)
; identity: params=[BC] ret=HL (Z3-PFCCO)
```

### Per-function comments:
```
; fun Box_get(self: u16 = HL) -> u16 = HL ; clobbers: F
; fun Box_addN(self: u16 = HL, n: u16 = DE) -> u16 = HL ; clobbers: F
; fun identity(x: u16 = HL) -> u16 = HL ; clobbers: F
```

**Discrepancy:** The header says `Box_get: params=[BC]` but the per-function comment says `self: u16 = HL`. The actual code uses HL as the self pointer. The header `params=[BC]` is wrong — it should be `params=[HL]`. Same discrepancy for `identity: params=[BC]` vs `x: u16 = HL`.

For `Box_addN`, the header says `params=[BC,HL]` but the per-function says `self: u16 = HL, n: u16 = DE`. The actual code expects self in HL and n in DE. The header `[BC,HL]` doesn't mention DE at all — it's wrong.

**Summary:**

| Function | Header claims | Per-func claims | Actual code uses | Correct? |
|----------|---------------|-----------------|------------------|----------|
| Box_get | params=[BC] | self=HL, ret=HL | HL in, HL out | Header wrong |
| Box_addN | params=[BC,HL] | self=HL, n=DE, ret=HL | HL+DE in, HL out | Header wrong |
| identity | params=[BC] | x=HL, ret=HL | HL in, A out(!) | Both wrong (ret should be HL but code returns A) |

The per-function ABI comments are closer to reality than the header summary, but `identity` has a codegen bug that makes even the per-function comment wrong.

---

## 4. Recommendation

**Replace or explicitly caveat.** This snippet should not be cited as showcase quality in its current state:

1. **`Box_get`** has 2 dead instructions (40% overhead) — passable but not "showcase"
2. **`Box_addN`** has a 4-instruction no-op round-trip — clearly a solver/peephole gap
3. **`identity`** has a codegen bug (truncates u16 to u8, wrong return register)
4. **`__objc_test_2`** has a codegen bug (uses uninitialized HL instead of DE)
5. **ABI header comments** contradict the per-function comments and the actual code

### Options:

**A. Replace with a hand-verified snippet** from a function where VIR produces genuinely optimal output (e.g., the `abs_diff` or `next_light` examples in `Eight_Languages_One_Binary.md` are much stronger showcases).

**B. Caveat in place:** Add a note like "VIR output, not yet peephole-optimized — shows zero-cost dispatch structure but contains redundant register shuffles."

**C. Fix the peephole** to eliminate the dead LD D,H/LD E,L and the BC round-trip, then re-emit. After peephole fix, `Box_get` and `Box_addN` would be genuinely tight code worth showcasing.

**Recommendation: Option A for now (use stronger examples), Option C as follow-up when VIR peephole improves.**
