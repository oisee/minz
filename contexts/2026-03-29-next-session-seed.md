# Next Session Seed — 2026-03-29 (updated end of session 13)

**Previous:** Session 12-13 — TermCondRet fix, assert harness fix, bool convention proven
**State:** VIR default, 1636+ asserts, 8 frontends verified, harness fixed

---

## Priority 0: Implement retFlag in PFCCO (~50 LOC)

GPU brute-force PROVEN design (456K sequences, 131K combos verified):

**Z→A branchless IMPOSSIBLE** on Z80. Z flag is write-only.

retFlag enum in PFCCO contract: Z3 chooses per-function from {A, CY, Z, A(0xFF)}

```smt
(declare-const ret_mode Int)  ; 0=A, 1=CY, 2=Z, 3=A_0xFF
; Cost per call site depends on caller usage (branch vs store)
```

Key primitives:
| CY→A(0xFF) | SBC A,A | 1i 4T branchless |
| CY?B:0 | SBC A,A; AND B | 2i 8T |
| CY?B:C | SBC A,A;LD D,A;LD A,B;XOR C;AND D;XOR C | 6i 24T CMOV |
| bool→int | SBC A,A; NEG | 2i 8T (0xFF→0x01) |

Architecture: regalloc (tables) ⊥ retFlag (epilog). Two independent layers.
GPU tables: NO regeneration needed.

VIR estimate: ~50 LOC additive extension in pfcco.go.

## Priority 1: fib(7)=13

VIR working on g.accVreg tracking + EX AF,AF' for save/restore A across CALL.
Same root cause as parse_digit (CALL arg setup + clobber restore).

## Priority 2: CALL Arg Setup + Clobber Restore

Root cause for parse_digit, fib, all inter-function calls:
- Missing LD A,B before CALL (arg not in expected register)
- Missing LD A,L after CALL (saved value not restored)
VIR: bridge.go translateCall / solver.go CALL modeling.

## Priority 3: shr4 Shift Count

shr4(0xAB)=0x55 — VIR const propagation for shift operand.

## Priority 4: Nested condret-sink

Pascal IsDigit, BoolAnd: condret-sink doesn't insert LD A,0/1 for nested if.
Same mechanism as is_digit fix but for deeper nesting.

## Priority 5: MZA SRL/AND Instructions

SRL and AND not assembling — blocks shift/bitwise Pascal tests.

---

## What Was Done (Session 12-13)

| Feature | Status |
|---------|--------|
| TermCondRet fix (VIR + PBQP) | ✅ |
| Assert harness PFCCO-aware | ✅ CRITICAL FIX |
| is_digit return-move reorder | ✅ |
| Pascal 54 Z80 asserts (9/9) | ✅ |
| ObjC 98 asserts (11/11) | ✅ |
| Frill pipe.frl 9 asserts | ✅ |
| C edge_cases 36 asserts | ✅ |
| ABAP OOP 21 asserts | ✅ |
| ABAP FUNCTION/ENDFUNCTION | ✅ preprocessor |
| ABAP name_test (FORM+CLASS) | ✅ no conflict |
| Eight Languages article + 11 images | ✅ |
| Bool convention GPU proven | ✅ CY per-function, Z3 decides |
| Z→A branchless impossible | ✅ PROVEN exhaustive |
| Branchless CMOV found | ✅ SBC A,A select pattern |
| Total asserts | ~1636 |

## Design Decisions (GPU PROVEN)

- **Bool return:** Z3 per-function from {A(0/1), CY, Z, A(0xFF)}
  - CmpEq → Z natural, CmpLt → CY natural
  - Caller branch → use flag directly (0T)
  - Caller store → materialize (SBC A,A for CY=4T, branch for Z=14T)
- **Bool representation:** 0x00/0xFF (SBC A,A, 1 instruction)
- **@error:** CY flag (orthogonal via A materialization)
- **CMOV:** SBC A,A;LD D,A;LD A,B;XOR C;AND D;XOR C (24T branchless)
- **Z flag:** write-only on Z80 (proven exhaustive 456K sequences)
- **GPU tables:** ⊥ retFlag (regalloc independent of epilog)
- **ABAP naming:** FORM=name, CLASS=ClassName_Method, FUNCTION=name (no conflicts)

## Session IDs

```bash
ddll explore
# VIR: cok1cgsq
# z80-optimizer: um2dy4ex
```
