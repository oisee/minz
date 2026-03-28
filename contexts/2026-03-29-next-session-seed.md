# Next Session Seed — 2026-03-29

**Previous:** Session 12 — TermCondRet fix, Pascal 26/26, bool convention design
**State:** VIR default, TermCondRet fixed (VIR+PBQP), 1046+ asserts, Pascal working

---

## Immediate: Check VIR Results

```bash
ddll explore

# VIR: fib recursive fix? shr4? is_digit?
ddll send <vir>:main "fib accVreg+EX AF,AF' status? shr4? is_digit clobber?"
```

## Priority 1: fib(7)=13 → CP/M Screenshot → LinkedIn

VIR working on g.accVreg tracking + EX AF,AF' for save/restore A across CALL.
When fixed: `Hello Frill! fib(7)=13 gcd(12,8)=4` on CP/M → screenshot → LinkedIn.

3 layers of save bug identified:
1. SUB 1 kills n → LD E,A ✅
2. LD A,E for n-2 kills fib(n-1) → LD H,A ✅
3. ADD A,B uses wrong reg → needs accVreg tracking

## Priority 2: shr4 Shift Count

shr4(0xAB)=0x55 (shifts by 1 instead of 4). VIR const propagation for shift operand.

## Priority 3: is_digit + All Clobber Bugs

is_digit(65)=1 (wrong, should be 0). LD A,0 between two CPs kills original A.
Same class: condret-sink inserts return value before comparisons finish.
Fix: accVreg tracking in codegen (same mechanism as fib).

## Priority 4: BoolReturnElim Grace Rule

Design decided: bool=Z per-call-site, @error=CY always.
- Grace rule matches callee [CP→LD A,0/1→RET] + caller [CALL→CP 0/1→JR]
- Needs type info, cross-function analysis, liveness, condition flip
- NOT text peephole

## Priority 5: Pascal Corpus Growth

26 asserts working. Can add: recursive examples (fibonacci, tower of hanoi),
string operations, record field access, nested procedures, for..downto.

## Priority 6: New Frontends (COBOL/1С/BASIC)

BCD types ready. COBOL PIC 9 → DAA codegen.

---

## What Was Done (Session 12)

| Feature | Status |
|---------|--------|
| TermCondRet fix (VIR) | ✅ condret-sink reorder + condcode mapping |
| TermCondRet fix (PBQP) | ✅ saveAccForCondRet for BrIf/BrIf2 |
| Pascal 26/26 Z80 asserts | ✅ |
| Pascal 6/6 examples compile | ✅ |
| Frill book: images embedded | ✅ epub+pdf rebuilt, uploaded v0.24.0 |
| Frill book: expr_eval full source | ✅ |
| Frill book: parser_combinator full source | ✅ |
| Frill book: ASM + Nanz comparison | ✅ |
| Bool convention design | ✅ Z per-call-site, CY for @error |
| is_digit clobber bug found | ✅ reported to VIR |
| PSIL exploration | ✅ compilable through MinZ |

## Design Decisions Made

- **Bool return (pure `-> bool`, no tuples):** Z3 models flag path vs materialize
  - Flag path: callee sets Z, caller JR Z (cost=0)
  - Materialize: callee LD A,0/1, caller OR A/JR Z (cost=+1B+4T)
  - Z3 dual-mode decides per-call-site (same as constrained vs standalone+adapter)
- **Tuples with bool `(u8, bool)`:** ALWAYS materialize A(0/1), no flag optimization
- **@error:** CY flag always (orthogonal to bool)
- **Implementation:** Z3 solver (NOT Grace peephole — needs Z3 for optimal decision)
- **GPU tables:** No regeneration needed — Z_flag is final-return-only (not intermediate storage)
  - Postfilter: for bool functions, check if last instruction before RET already sets Z correctly → skip LD A,0/1
  - z80-optimizer confirmed: existing tables valid, postfilter sufficient

## Session IDs

```bash
ddll explore
# VIR: cok1cgsq
# z80-optimizer: um2dy4ex
# antique-toy: eo29c66e
```
