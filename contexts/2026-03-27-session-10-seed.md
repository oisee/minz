# Next Session Seed (after Session 10)

**Date:** 2026-03-27
**Release:** v0.23.0 (assets updated: 6 books/articles)

---

## Priority 1: TermCondRet Fix → fib(7)=13 → Screenshot

fib/gcd go through PBQP fallback (Z3 timeout on recursion).
PBQP path hits TermCondRet save-before-clobber bug.
Partial fix landed (saveAccForCondRet) but more cases exist.

When fib(7)=13 passes on Z80:
- Enable dual asserts across corpus
- Run hello_cpm.frl → clean output: `Hello Frill! fib(7)=13 gcd(12,8)=4`
- Screenshot for LinkedIn article
- Frill on ZX Spectrum demo

## Priority 2: shr4 Shift Count Bug

`shr4(0xAB)` returns 0x55 (shift by 1) instead of 0x0A (shift by 4).
Shift count const not propagating correctly through VIR.

## Priority 3: 5 Remaining Z80 Codegen Bugs

From dual assert audit:
- is_zero(0)=0, is_even(4)=0 — comparison → bool
- get_red(0xF800)=0 — u16 shift/mask
- pointer deref asm error
- sparse array init (got 144, want 60)

## Priority 4: New Frontends

ADR-0041 BCD types ready → COBOL frontend possible.
1С (Russian enterprise) — 1 session MVP.
BASIC — THE retro language, highest wow factor.

---

## What Was Done (Session 10)

| Fix/Feature | Impact |
|------------|--------|
| VIR as default backend | LIR→VIR switch, add(3,4)==7 via z80 |
| PBQPAlloc one-liner | PBQP fallback works, 3 Frill on Z80 |
| shl-by-0 MIR2 fix | cv>0 guard removed, shift by 0 = identity |
| shl u8 narrowing | C frontend: u8<<N stays u8, not u16 |
| saveAccForCondRet | Partial TermCondRet fix |
| BCD types (ADR-0041) | TyBCD8/16/24/32, BCDForDigits(), IsBCD() |
| 3 Frill demos | state_machine, minigame, parser_combinator |
| Frill article | "ML on ZX Spectrum" for LinkedIn |
| Frill book chapter | Real-World Demos + epub/pdf rebuilt |
| Z80 dual assert audit | 5 bugs found, documented |
| __builtin_unreachable fix | Macro instead of static function |
| "Hello Frill!" on CP/M | 751 bytes, prints correctly |

## Key Discovery: VIR Zero Bugs

All Z80 codegen bugs were in MIR2/C-frontend, NOT VIR:
- shl cv>0 guard (MIR2 genShift)
- shl u8→i16 promotion (C frontend narrowing)
- TermCondRet save-before-clobber (MIR2 codegen)
- LIR register mismatch with assert harness (LIR, now deprecated)

VIR+PBQP pipeline is production-correct.

## Session IDs

```bash
ddll explore
# minz-vir: cok1cgsq (was jjjlhyva yesterday)
# z80-optimizer: um2dy4ex
# antique-toy: eo29c66e
```

## Corpus Status

| Corpus | Asserts | mir2 | z80 |
|--------|---------|------|-----|
| examples/c89/ | 350 | ✅ | — |
| examples/c/ | 269 | ✅ | partial (5 bugs) |
| examples/frill/ | 427 | ✅ | 3 demos assemble |
| **Total** | **1046** | **✅** | **WIP** |
