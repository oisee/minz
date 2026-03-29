# Next Session Seed — 2026-03-30

**Previous:** Sessions 12-15 — self-hosting pipeline, VIR P0/P1/P2, fib(7)=13
**State:** Self-hosting Stage 1 works, ~1800 asserts, 8 frontends

---

## Priority 0: Fix Self-Hosting Bugs

1. **Multi-function emit_lanz**: first function outputs nulls to buffer. `out_str` poke chain — string literal address conflicts with output buffer?
2. **print_ast infinite recursion**: removed, need to fix AST traversal for debugging

## Priority 1: FatFS VIR_DUMP_GPU_BATCH

z80-optimizer found CORPUS BIAS: our 820 functions are leaf/small (max 14v).
FatFS = 10-35 vregs, 200-764 instructions. Need corpus dump to calibrate enriched tables.

## Priority 2: Self-Hosting Stage 2 → Stage 3

- Stage 2: Lanz S-expr parser ✅ (roundtrip verified)
- Stage 3: AST → MIR2 (Go helper ready, @regalloc_lookup host ready)
- Stage 4: assignment → peephole → .a80 (@peephole_match host ready)

## Priority 3: README Update

Add self-hosting, z88dk comparison (-54%), new articles to README.md header.

## Self-Hosting Pipeline Files

- `examples/nanz/self_tokenizer.nanz` — tokenizer v3 (interned strings)
- `examples/nanz/self_parser.nanz` — parser + Lanz emitter + file I/O
- `examples/nanz/self_lanz_parser.nanz` — S-expr parser
- `examples/nanz/tiny.nanz` — test input (3 functions)

## What Was Done

| Feature | Status |
|---------|--------|
| VIR P0/P1/P2 all fixed | ✅ |
| fib(7)=13 on Z80 | ✅ |
| Tetris on CP/M | ✅ |
| Self-hosting tokenizer v3 | ✅ |
| Self-hosting parser + Lanz emit | ✅ (single func) |
| Self-hosting S-expr parser | ✅ |
| @file_read/@file_write hosts | ✅ |
| EnsureHeap fix | ✅ |
| @extern for host return types | ✅ |
| Lizp → Scheme R5RS | ✅ |
| ABAP FUNCTION/ENDFUNCTION | ✅ |
| --asserts mir2/z80/none | ✅ |
| assert func() / assert not func() | ✅ |
| MinZ vs z88dk comparison (-54%) | ✅ |
| Eight Languages article | ✅ |
| Bool convention GPU-proven | ✅ |
| O(1) regalloc (91% without Z3) | ✅ |
| Branchless CMOV/ABS/MIN/MAX | ✅ |
| ~1800+ asserts, 8 frontends | ✅ |
