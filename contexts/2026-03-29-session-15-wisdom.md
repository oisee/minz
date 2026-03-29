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

## Self-Hosting Status: ~5% of Nanz

Parses: `fun name(params) -> type { return expr }`
Missing: if/else, while, let/var, struct, enum, @extern, import,
nested expressions, string literals, multi-function emit.
Foundation proven: file → tokenize → parse → AST → Lanz → file → Z80.

## Critical Fix: EnsureHeap Before WriteHeapBytes

file_read host wrote 145 bytes to heap@49152 but WriteHeapBytes
silently dropped the write (heap too small). Adding EnsureHeap
before WriteHeapBytes fixed it. Same pattern needed for file_write.

## Critical Fix: @extern for Host Function Return Types

Without `@extern fun peek(addr: u16) -> u8`, Nanz lowering generates
`call @peek(x); ret void` — no Dst register, return value lost.
With @extern, generates `%r = call @peek(x); ret %r` — correct.

## WASM Backend (New!)

pkg/mir2wasm: MIR2 → WASM binary (174 bytes for 2 functions).
wazero runtime for verification. 6th backend: Z80/eZ80/QBE/C/WASM/VIR.

## Known Issues
- Multi-function emit_lanz: first function outputs nulls to buffer
- print_ast infinite recursion on complex AST (removed, needs fix)
- `i = len` (assignment-as-break) in if block — Nanz parse error
- Lizp host function return values don't propagate (Lanz lowering)
- VIR edge move F→D pattern missing (scheme_r5rs Z80)
- FatFS corpus bias: our functions too small (max 14v vs FatFS 35v)
