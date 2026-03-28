# Next Session Seed — 2026-03-29 (end of session 14)

**Previous:** Sessions 12-14 — fib(7)=13, Tetris CP/M, VIR P0 CALL fix, bool GPU-proven
**State:** VIR production for non-leaf, 1700+ asserts, 8 frontends, Tetris on CP/M

---

## Immediate: Check VIR Results

```bash
ddll explore
ddll send <vir>:main "P1 condret-sink status? P2 loop_head? BuildGPUDesc?"
ddll send <z80-optimizer>:main "corpus dump ready?"
```

## What's Fixed

- **VIR P0 CALL arg setup** ✅ — 4 bugs (vreg collision, DstHint, peephole, coalescing)
- **VIR TermCondRet** ✅ — condret-sink reorder + condcode mapping
- **Assert harness PFCCO** ✅ — correct register loading for VIR functions
- **validateCallArgSetup** ✅ — auto-PBQP fallback safety net
- **fib(7)=13** ✅ — recursive via PBQP fallback (island skip)
- **Tetris CP/M** ✅ — VIR default, 23/38 VIR + 15 PBQP
- **Hello Frill!** ✅ — VIR default, fib(7)=13 gcd(12,8)=4

## Priority 1: P1 condret-sink (VIR working)

Cross-block edge moves not generated. bool_and(1,0)=1 because LD A,B
not inserted at block edge. CFG solver per-instruction vars within block,
but edge moves between blocks missing.

Affects: Pascal IsDigit, BoolAnd, SubSafe, Lizp max_byte.

## Priority 2: P2 loop_head label (VIR)

While-loop label not emitted in VIR ASM. JP .func_loop_head1 but label
not defined. Affects Lizp fact_loop, sum_loop.

## Priority 3: BuildGPUDesc corpus dump (VIR)

z80-optimizer evaluator ready. Expand BuildGPUDesc (+ops/fixed/costs, ~30 LOC).
VIR_DUMP_GPU_BATCH → JSON per function. z80-optimizer will precompute
ALL 7M combinations for ≤6 vregs. Replaces Z3 for 95%+ functions.

GPU regalloc architecture:
- hash(shape, op_bag) → O(1) optimal assignment
- infeasible detection → decompose by cut vertex → recurse
- enricher v4: CALL save cost, mul8_safe, width-aware feasibility

## Priority 4: Print number fix

Tetris Score displays garbage. print_number/WriteU8 codegen issue.

## Priority 5: Bool return convention implementation

Design GPU-proven:
- bool = CY flag (caller JR C/NC)
- materialize = SBC A,A → 0xFF/0x00 (4T branchless)
- Z→A branchless IMPOSSIBLE (proven 456K sequences)
- CMOV: SBC A,A;LD D,A;LD A,B;XOR C;AND D;XOR C (24T)
- retFlag: Z3 per-function from {A, CY, Z, A(0xFF)}
- Peephole first, Z3 dual-mode second

## What Was Done (Sessions 12-14)

| Feature | Status |
|---------|--------|
| VIR P0 CALL arg fix (4 bugs) | ✅ |
| VIR TermCondRet (both paths) | ✅ |
| Assert harness PFCCO-aware | ✅ |
| VIR validateCallArgSetup safety net | ✅ |
| fib(7)=13 on Z80 (PBQP) | ✅ |
| Tetris on CP/M (VIR default) | ✅ |
| Hello Frill! CP/M (VIR default) | ✅ |
| --asserts mir2/z80/none CLI | ✅ |
| assert func() / assert not func() | ✅ |
| assert == true/false literals | ✅ |
| Lizp → Scheme R5RS (define, lambda, predicates) | ✅ |
| Lizp ident sanitize (hyphen→underscore) | ✅ |
| ABAP FUNCTION/ENDFUNCTION | ✅ |
| ABAP OOP asserts (21) | ✅ |
| ObjC math + game_ecs (22 asserts) | ✅ |
| Pascal bubble_sort + records | ✅ |
| Pascal 54 Z80 asserts | ✅ |
| C edge_cases 36 asserts | ✅ |
| Eight Languages article + 11 images | ✅ |
| Bool convention GPU-proven | ✅ |
| Branchless ABS/MIN/MAX/CMOV | ✅ |
| div3 EXACT (mul171>>9) | ✅ |
| Non-blocking tui_read_key (BDOS 6) | ✅ |
| Total asserts | ~1750+ |

## Assert Corpus

| Language | Asserts | Z80 Status |
|----------|---------|------------|
| Frill | 436 | ✅ 14/15 |
| C89 | 353 | ✅ |
| C99+ | 305 | ✅ |
| ObjC | 98 | ✅ 11/11 |
| Pascal | 54 | ✅ 9/9 (leaf) |
| Nanz | 371+ | ✅ |
| Lizp | 52 | ✅ mir2, Z80 partial |
| ABAP | 31 | ✅ mir2 |
| PL/M | 9 | ✅ |

## Known VIR Bugs (P1, P2)

1. **P1 condret-sink** — cross-block edge moves missing (VIR working on it)
2. **P2 loop_head label** — while-loop label not emitted
3. **Recursive** → PBQP fallback (island skip workaround)
4. **Print number** — WriteU8 codegen garbled

## GPU Regalloc Revolution (z80-optimizer)

- Enricher v4: 123K shapes, width-aware, CALL save cost
- operation_bag + shape → O(1) lookup
- Infeasibility detection before codegen
- 7M combinations for ≤6 vregs (GPU minutes)
- 755 arithmetic sequences with clobber info
- Awaits corpus dump from VIR

## Session IDs

```bash
ddll explore
# VIR: cok1cgsq
# z80-optimizer: um2dy4ex
```
