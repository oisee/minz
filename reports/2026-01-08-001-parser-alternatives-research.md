# MinZ Parser Alternatives Research Report

**Date:** 2026-01-08
**Author:** Claude Opus 4.5 (background agent)
**Status:** Complete

## Executive Summary

This report analyzes parsing alternatives for the MinZ compiler to replace the current tree-sitter-based approach that causes OOM issues (60GB+ RAM consumption). Based on the research, **a hand-written recursive descent parser** is the recommended approach for achieving a self-contained Go toolchain with predictable memory usage.

## 1. Current State Analysis

### Current Architecture

The MinZ compiler (`minzc/`) currently uses multiple parsing approaches:

| Parser | Status | Implementation |
|--------|--------|----------------|
| **Tree-sitter (CLI)** | Primary, causes OOM | Shells out to `tree-sitter parse`, reads S-expression output |
| **ANTLR4** | Experimental (5% success) | Generated from `MinZ.g4`, visitor-based AST conversion |
| **Native tree-sitter** | Disabled | Go bindings via `go-tree-sitter`, CGO issues |

**Key Files:**
- `minzc/pkg/parser/parser.go` - Main parser orchestration (2300+ lines)
- `minzc/pkg/parser/antlr_parser.go` - ANTLR4 implementation (1900+ lines)
- `minzc/pkg/parser/sexp_parser.go` - S-expression parser for tree-sitter output
- `grammar.js` - Tree-sitter grammar definition (1134 lines)
- `minzc/grammar/MinZ.g4` - ANTLR4 grammar (598 lines)

## 2. Parser Options Comparison

### Option A: Hand-Written Recursive Descent Parser (RECOMMENDED)

| Aspect | Assessment |
|--------|------------|
| **Memory Usage** | Excellent - O(n) with source file size |
| **Dependencies** | None - Pure Go, self-contained |
| **Maintainability** | Good - Direct code, easy to debug |
| **Error Recovery** | Good - Full control over error messages |
| **Performance** | Excellent - Single pass |
| **Implementation Effort** | Medium - 2-4 weeks |

**Pros:**
- Complete control over memory allocation
- No CGO, no external binaries
- Follows Go standard library pattern (`go/parser`, `text/template`)
- Easy to add MinZ-specific features
- Rob Pike's Lexical Scanning pattern is well-established

**Cons:**
- Must handle left recursion manually (via precedence climbing)
- No incremental parsing
- More initial code to write

**Reference:** The Z80 assembler parser at `minzc/pkg/z80asm/parser.go` demonstrates this pattern.

### Option B: ANTLR4 with Go Target

| Aspect | Assessment |
|--------|------------|
| **Memory Usage** | Good - ~2-10x source size |
| **Dependencies** | Moderate - ANTLR runtime |
| **Maintainability** | Good - Grammar separate from code |
| **Error Recovery** | Excellent - ALL(*) algorithm |
| **Implementation Effort** | Low - Grammar exists, fix visitor |

**Current Issue:** Only 5% success rate due to incomplete visitor implementation.

### Option C: PEG Parser (Pigeon)

| Aspect | Assessment |
|--------|------------|
| **Memory Usage** | Excellent - Linear with memoization |
| **Dependencies** | Minimal - Generated pure Go |
| **Error Recovery** | Limited - PEG doesn't allow ambiguity |
| **Implementation Effort** | Medium - Must rewrite grammar |

### Option D: Participle (Struct-Based)

| Aspect | Assessment |
|--------|------------|
| **Memory Usage** | Excellent - Minimal allocations |
| **Dependencies** | Single library |
| **Performance** | Excellent - 3x faster than pigeon |
| **Implementation Effort** | Medium - Define AST structs with tags |

### Option E: go-tree-sitter (NOT RECOMMENDED)

| Aspect | Assessment |
|--------|------------|
| **Memory Usage** | Risky - Unpredictable |
| **Dependencies** | CGO required |
| **Implementation Effort** | Low - Grammar exists |

## 3. Memory Efficiency Analysis

| Parser Type | Memory per 1KB Source | 1MB File | 10MB File |
|-------------|----------------------|----------|-----------|
| Tree-sitter (external) | ~60KB+ (variable) | 60MB+ | Unpredictable |
| ANTLR4 | ~5-10KB | 5-10MB | 50-100MB |
| Recursive Descent | ~1-2KB | 1-2MB | 10-20MB |
| Participle | ~1KB | 1MB | 10MB |
| PEG (pigeon) | ~2-3KB | 2-3MB | 20-30MB |

## 4. MinZ-Specific Considerations

### Grammar Complexity

The MinZ grammar has 27 conflicts in tree-sitter, requiring:
- ANTLR (ALL*) handles conflicts well
- PEG requires grammar refactoring
- Hand-written parser needs careful operator precedence

### Special Features

| Feature | Complexity | Notes |
|---------|------------|-------|
| `fun`/`fn` keywords | Low | Simple alternation |
| Metafunctions (`@print`, `@minz[[[...]]]`) | Medium | Context-sensitive |
| Inline assembly (`asm { ... }`) | Medium | Raw content capture |
| Operator overloading | Low | Semantic, not syntax |
| Generics (`<T: Constraint>`) | Medium | Angle bracket disambiguation |

## 5. Recommendation Summary

| Criterion | Hand-Written | ANTLR4 | PEG | Participle | go-tree-sitter |
|-----------|-------------|--------|-----|------------|----------------|
| **Memory** | Excellent | Good | Excellent | Excellent | Risky |
| **Self-Contained** | Yes | Runtime | Yes | Library | CGO |
| **Time** | 4 weeks | 1-2 weeks | 2-3 weeks | 2-3 weeks | 1 week |
| **Verdict** | **Primary** | Fallback | Alternative | Alternative | Not recommended |

## 6. Implementation Strategy

### Phase 1: Lexer (Week 1)
Create `pkg/parser/rdp/lexer.go` with token definitions for all MinZ tokens.

### Phase 2: Expression Parser (Week 2)
Implement precedence climbing for binary expressions.

### Phase 3: Statement/Declaration Parser (Week 3)
Parse functions, structs, enums, impl blocks.

### Phase 4: Special Features (Week 4)
- Metafunctions (`@minz[[[...]]]`, `@lua[[[...]]]`)
- Inline assembly blocks
- Generic parameters

### Migration Path
1. Create new parser at `minzc/pkg/parser/rdp/`
2. Use `MINZ_USE_RDP=1` environment variable
3. Compare output with tree-sitter
4. Switch default at >90% success rate

## 7. References

- [Go parser package](https://pkg.go.dev/go/parser)
- [Handwritten Parsers & Lexers in Go](https://blog.gopheracademy.com/advent-2014/parsers-lexers/)
- [Participle - Go parser library](https://github.com/alecthomas/participle)
- [Pigeon - PEG parser generator](https://github.com/mna/pigeon)

## 8. Conclusion

For MinZ's goal of a **self-contained Go toolchain**, a **hand-written recursive descent parser** is optimal. It eliminates external dependencies, provides predictable memory usage, and gives full control over MinZ-specific features.

Estimated timeline: **4 weeks** for complete implementation with >90% success rate.
