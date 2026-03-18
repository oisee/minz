# Report 095 — LIR Becomes Default Backend

**Date:** 2026-03-18
**Branch:** `feat/lir-backend`
**Commits:** 21 (d7e9a08..current)

---

## Milestone

The LIR backend (ISLE combining + WFC register allocation + PBQP hints) is now the **default code generation backend** for MinZ. The `--lir` flag defaults to `true`.

**948/948 = 100%** functions compile and are VM-verified across all 4 frontends.

## Showcase: LIR vs Production (Side-by-Side)

### Identical Output (5/5 leaf functions)

```
Function     │ Production (PBQP)     │ LIR (ISLE+WFC)
─────────────┼───────────────────────┼──────────────────────
add(a,b)     │ ADD A, C / RET        │ ADD A, C / RET         ✅ IDENTICAL
sub(a,b)     │ SUB C / RET           │ SUB C / RET            ✅ IDENTICAL
double(x)    │ ADD A, A / RET        │ ADD A, A / RET         ✅ IDENTICAL
add3(a,b,c)  │ ADD A,C / ADD A,D /RET│ ADD A,C / ADD A,D /RET ✅ IDENTICAL
mul_add(a,b,c)│ADD A,C / SUB D / RET │ ADD A,C / SUB D / RET  ✅ IDENTICAL
```

### LIR Advantages

```
mul8(a,b)    │ TODO: stub (broken!)  │ LD B,C / JP __mul8     ✅ LIR WORKS
main()       │ RST 0x10 / JP 0x0010  │ CALL putchar / JP putchar
             │ (resolved addrs)      │ (symbolic, assembler resolves)
```

Production's `mul8` emits a TODO stub. LIR correctly calls the `__mul8` runtime.

### Production Advantages

```
abs_diff(a,b)│ SUB C / RET NC /NEG/RET│ (multi-block: flat path)
             │ (4 insts, optimal)     │ (not yet optimal)
main()       │ RST 0x10               │ CALL putchar
             │ (1 byte, 11T)          │ (3 bytes, 17T)
```

Production uses `RST 0x10` (restart instruction, 1 byte) for addresses 0x00-0x38. LIR doesn't have RST optimization yet. Production also handles multi-block functions (if/else) optimally with conditional returns (`RET NC`, `RET Z`).

## Architecture Changes

### Import Cycle Fix
`lanz → pipeline → lir → isle → lanz` cycle broken by extracting the S-expression parser into `isle/sexpr.go` (self-contained, no external imports).

### Default Backend Switch
`--lir` flag changed from `default: false` to `default: true`. PBQP fallback remains: if any function fails LIR, entire module falls back to PBQP+Z80Codegen.

### OpCallIndirect
Function pointers via `__call_hl` trampoline (`JP (HL)`, 1 byte). Standard Z80 indirect call pattern.

## Quality Summary

| Category | Count | Status |
|----------|-------|--------|
| Leaf functions (arithmetic, logic) | ~80% of corpus | **Identical to production** |
| Function calls (direct) | All | **Correct** (CALL + tail call opt) |
| Function calls (indirect) | All | **Correct** (`__call_hl` trampoline) |
| Runtime multiply | All | **Working** (`__mul8`/`__mul16`) |
| Multi-block (if/else, loops) | Some | **Correct but not optimal** |
| RST optimization | 0 | **Not yet** (production advantage) |

## What's Next

1. **Multi-block optimization** — conditional returns (`RET Z`, `RET NC`), if-else codegen
2. **RST optimization** — detect `CALL 0x0010` → `RST 0x10` (saves 2 bytes)
3. **EXX shadow stream** — L3 resource layer for bulk save/restore
4. **Merge to master** — all tests pass, architecture documented
