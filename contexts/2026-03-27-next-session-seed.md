# Next Session Seed — 2026-03-27

**Previous:** v0.23.0 Birthday Marathon (sessions 3-9)
**State:** C23 complete (13/13), 619 mir2 asserts, Z80 path broken

---

## Immediate: Z80 Codegen Fixes

**The #1 priority. mir2 VM = correct, Z80 = wrong. Fix the gap.**

### Fix 1: VIR F→A Move Pattern (one line)
```
cfgsolver.go: WARNING "no move pattern" → return error
```
Then PBQP fallback catches it. Currently: silent corruption, broken ASM.
VIR team confirmed fix. Expected: 17/18 → 18/18 C files.

### Fix 2: Condition Vreg Mismatch
```
abs_val: OR A tests x (%r1 in A), should test is_neg (%r2 in B/C)
Fix: LD A, %r2 before OR A. Or: codegen for cmp.ugt must load operand to A.
```
Affects: abs_val, gcd, any function with `if (var > 0)` where var ≠ A.

### Fix 3: mul Codegen on Z80
```
mul3(14) returns 0 via VIR Z80 output.
PBQP __mul8 runtime missing in VIR mode (emitRuntimeRoutines fix).
```

### Verify After Fixes
```bash
cat > /tmp/smoke.c << 'EOF'
#include <stdint.h>
uint8_t add(uint8_t a, uint8_t b) { return a + b; }
uint8_t mul3(uint8_t x) { return x * 3; }
uint8_t abs_val(uint8_t a, uint8_t b) { return a > b ? a - b : b - a; }
// assert add(3, 4) == 7 via z80
// assert mul3(14) == 42 via z80
// assert abs_val(10, 3) == 7 via z80
EOF
mz /tmp/smoke.c --vir -o /tmp/smoke.a80
mza /tmp/smoke.a80 -o /tmp/smoke.com
echo "" | mze /tmp/smoke.com -t cpm
```

## Priority 2: `--asserts` CLI Flag

```
mz program.c --asserts=mir     # only mir2 asserts (fast, default)
mz program.c --asserts=z80     # only z80 asserts (full verify)
mz program.c --asserts=all     # both (comprehensive)
```

Needed because: z80 path timeouts on complex functions. Default should be mir2 for CI speed.

## Priority 3: Books Update

- [ ] Nanz Language Book v8 — add @error ?, RLCA sled, ADT improvements
- [ ] C23 Book honesty update — Z80 path status after fixes

## Priority 4: Paper A Final Draft

All data ready. GPT-5.4 reviewed. `research/paper-a-draft.md`.

---

## What Was Done (Session 9)

| Feature | Status | Asserts |
|---------|--------|---------|
| @error `?` enforcement | ✅ | — |
| stdbool.h, assert.h, ctype.h, stdalign.h, stdnoreturn.h | ✅ | — |
| C11 anonymous structs/unions | ✅ | 6 |
| `_Alignof`, `typeof` | ✅ | 3 |
| Array designated init | ✅ | 6 |
| MZA INCBIN | ✅ | — |
| RLCA sled (9 bytes, 8 entries) | ✅ | — |
| `nullptr` | ✅ | 1 |
| `#embed` with limit/offset | ✅ | 10 |
| `constexpr` | ✅ | 6 |
| Enum underlying type | ✅ | 14 |
| `<stdbit.h>` (13 functions) | ✅ | 34 |
| `[[]]` attributes (strip) | ✅ | 9 |
| `auto`, `__builtin_unreachable` | ✅ | 11 |
| Digit separators `1'000` | ✅ | 9 |
| `_BitInt(N)` | ✅ | 12 |
| C23 Book (epub+pdf) | ✅ | — |
| v0.23.0 release | ✅ | — |
| **Total new** | **17 features** | **269 (C99+)** |

## Corpus Status

| Corpus | Files | Asserts | Backend |
|--------|-------|---------|---------|
| `examples/c89/` | 38 | 350 | mir2 ✅ |
| `examples/c/` | 19 | 269 | mir2 ✅ |
| **Total** | **57** | **619** | **mir2 ✅, z80 ❌** |

## Session IDs

```bash
ddll explore
# minz: ju6yy047
# minz-vir: jjjlhyva
# z80-optimizer: um2dy4ex
# antique-toy: eo29c66e
```

## Key Discovery: Z80 Assert Gap

**mir2 VM passes everything. Z80 emulator fails on basics (`mul3`, `abs_val`, `gcd`).**

This means:
1. Frontend + HIR + MIR2 semantics = CORRECT
2. MIR2 → Z80 lowering = BROKEN for conditionals + multiply
3. Dual asserts (mir2 + z80) = essential testing pattern
4. VIR team identified root causes, one-line fixes ready

**Next session goal: Z80 parity with mir2. When `mul3(14) == 42 via z80` passes, we can enable dual asserts everywhere.**
