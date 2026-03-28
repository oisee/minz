# Self-Hosting Pipeline: Nanz Compiles Nanz (2026-03-29)

## Summary

MinZ can now compile itself — partially. A Nanz tokenizer and parser, written
in Nanz, running on MZV (MIR2 VM), produces valid Lanz S-expressions that the
existing `mz` compiler turns into optimal Z80 machine code.

```
fun add(a: u8, b: u8) -> u8 { return a + b }
     ↓ self_parser.nanz (Stage 1, MZV)
(fun add ((a u8) (b u8)) u8 (return (+ a b)))
     ↓ self_lanz_parser.nanz (Stage 2, MZV)
AST tree: 17 nodes, 24 bytes interned strings
     ↓ mz (existing compiler)
add: ADD A, C / RET  (Z3-PFCCO optimal, 2 instructions)
```

## Pipeline Architecture

| Stage | Input | Output | Status | LOC |
|-------|-------|--------|--------|-----|
| 1. nanz-front | .nanz source | .lanz (S-expr HIR) | ✅ Nanz/MZV | 480 |
| 2. lanz-parse | .lanz S-expr | AST tree in arena | ✅ Nanz/MZV | 262 |
| 3. mir2-lower | AST tree | .mir2 (SSA IR) | 🔧 Go helper | — |
| 4. regalloc | .mir2 + tables | Z80 ASM | 🔧 Go helper | — |
| 5. assemble | .a80 | .com binary | ✅ mza (existing) | — |

## Key Techniques

### Interned Strings (Pascal-style)
```
Arena: [len:u8][chars...][len:u8][chars...] ...
       [3][f][u][n]  [3][a][d][d]  [2][u][8]
```
- No null terminator (1 byte saved per string vs C)
- O(1) length (vs strlen O(n))
- Dedup: same text → same pointer → keyword check = pointer equality
- `if str_ptr == KW_FUN` instead of `strcmp(text, "fun")`

### Arena Allocation (Zero GC)
Three arenas in 64KB address space:
- **0x8000**: AST nodes (8 bytes each: tag + 3×u16)
- **0x9000**: Interned strings (Pascal-style)
- **0xB000**: Token stream (4 bytes each: type + string_ptr)
- **0xC000**: Source text buffer

Bump allocation only. No free, no fragmentation, no GC.

### MZV Host Functions
```nanz
@extern fun peek(addr: u16) -> u8    // read VM heap
@extern fun poke(addr: u16, val: u8) -> void  // write VM heap
@extern fun @file_read(path: ^u8, buf: ^u8) -> u16  // host filesystem
```
`@extern` required — without it, `return peek(x)` loses return value
(Nanz lowering generates `call @peek; ret void` instead of `%r = call @peek; ret %r`).

### O(1) Register Allocation (Stage 4)
Backend provides `@regalloc_lookup` host function:
- Input: interference graph + operation bag (in MZV heap)
- Lookup: 37.6M precomputed shapes (GPU-enriched tables)
- Output: optimal register assignment
- Fallback: Z3 SMT solver for table misses (<1%)

## What's Next

1. **Stage 3**: Lanz AST → MIR2 SSA lowering (Go helper → Nanz port)
2. **Stage 4**: MIR2 → Z80 via enriched tables (Go helper → Nanz port)
3. **Full Nanz port**: All 5 stages in Nanz → true self-hosting
4. **CP/M target**: Self-hosting compiler as .com binary on Z80!

## Files

- `examples/nanz/self_tokenizer.nanz` — Stage 1: tokenizer v3 (interned strings)
- `examples/nanz/self_parser.nanz` — Stage 1: recursive descent parser
- `examples/nanz/self_lanz_parser.nanz` — Stage 2: S-expression parser
- `examples/nanz/tiny.nanz` — test input for tokenizer

## Metrics

| Metric | Value |
|--------|-------|
| Total self-hosted LOC | ~740 (Nanz) |
| Tokenizer tokens | 19 (for test input) |
| AST nodes | 17 |
| Interned string bytes | 24 (with dedup) |
| AST arena bytes | 64 |
| Token arena bytes | 76 |
| Z80 output | 2 instructions (ADD A,C / RET) |
