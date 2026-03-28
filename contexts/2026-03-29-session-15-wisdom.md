# Session Wisdom: Session 15 (2026-03-29)

## Breakthroughs

### Self-Hosting Pipeline — 2/5 Stages Complete!
- **Stage 1**: .nanz → tokenizer → parser → .lanz (valid Lanz S-expr)
- **Stage 2**: .lanz → S-expr parser → AST tree (roundtrip verified)
- Pipeline proven: `fun add(a: u8, b: u8) -> u8 { return a + b }` → `(fun add ((a u8) (b u8)) u8 (return (+ a b)))` → `ADD A, C / RET`

### Interned Strings with Pointer Equality
Pascal-style: [len: u8][chars...]. No null terminator.
Keyword check = pointer comparison (O(1) instead of strcmp).
91 bytes for tokenizer corpus, 24 bytes for Lanz parser.

### @extern Fix for Host Functions
Root cause of MIR2 VM "peek returns 0" bug: without @extern declaration,
`return peek(x)` lowered to `call @peek(x); ret void` — no Dst register.
Fix: `@extern fun peek(addr: u16) -> u8` tells lowering the return type.

### MZV Host Functions Added
peek, poke, @file_read, @file_size, @file_exists, @print_u8, @print_nl,
@print_char, @regalloc_lookup, @peephole_match. EnsureHeap for 64KB.

### VIR P0/P1/P2 All Fixed
- P0: CALL arg setup (4 sub-bugs)
- P1: cross-block param edge moves
- P2: loop_head label emission

### O(1) Regalloc Architecture
91% corpus without Z3. 5-level pipeline:
L0: cut vertex split, L1: enriched ≤6v, L2: EXX bipartite,
L3: GPU partition, L4: Z3 fallback (<1%).

## Known Issues
- `i = len` (assignment-as-break) in if block — Nanz parse error
- Lizp host function return values don't propagate (Lanz lowering)
- VIR edge move F→D pattern missing (scheme_r5rs Z80)
