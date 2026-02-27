# Development Priorities Audit (Feb 27, 2026)

## Context

Comprehensive audit of MinZ compiler priorities, implementation status, and CLAUDE.md accuracy. Triggered by discovery that several "planned" features were already implemented.

---

## Stale Documentation Findings

### Items incorrectly listed as IN PROGRESS / PLANNED

| Item | CLAUDE.md said | Actual status | Evidence |
|------|---------------|---------------|----------|
| **Native parser** | "IN PROGRESS (Q1 2026)" | **DONE** (Feb 8, 2026) | Participle parser at `pkg/parser/participle/`, tree-sitter fully removed |
| **Nested functions** | TOBE (Week 2-4) | **DONE** | `registerLocalFunctionSignature()`, 5+ examples use it |
| **Enum values** (`State::IDLE`) | TOBE (Future) | **DONE** | Parser tests confirm, both `.` and `::` syntax work |
| **Local/nested functions** | PARKED ("use lambdas") | **DONE** | Same as nested functions above |

### Items accurately described

| Item | Status | Notes |
|------|--------|-------|
| Pattern matching | WIP | Syntax parses, codegen TODO |
| @minz[[[]]] | WIP | Limited execution, real constraints |
| MIR interpreter | WIP | Arrays/structs working |
| LSP / DAP / WASM | TOBE | Not started, no code exists |
| Function overloading | DONE | `overload_resolution.go` fully implemented |

### Corrections applied to CLAUDE.md

1. Removed "CURRENT PRIORITY: Native Parser" banner
2. Added new priorities: register bugs, iterator fusion, LSP/DAP
3. Moved parser, nested functions, enum values to DONE
4. Removed nested functions from PARKED
5. Updated toolchain list (added MZX, MZD)
6. Updated version references from v0.18.0 to v0.19.1
7. Removed `grammar.js` from project structure (tree-sitter gone)
8. Added iterator chain fusion to WIP
9. Added array literal optimization to WIP

---

## Current Priority Assessment

### Priority 1: Register Allocator & Loop Codegen Bugs

**Impact:** Blocks ALL complex programs (not just iterators).

**Known bugs:**
1. **While/for loop register corruption** — same physical register assigned to two live virtual registers. Affects all backends.
2. **`loadToHL` stale values** — `OpAdd` skips load when it thinks HL already has the right value, but HL was modified by prior instructions.
3. **Loop rerolling too aggressive** — merges putchar sequences across newline() calls.

**Location:** `minzc/pkg/codegen/z80.go` (lines ~2226, ~4022-4080)

**Assessment:** These are deep codegen surgery. High effort, high risk of destabilizing working programs. But without fixing them, any loop-based feature (iterators included) will break on non-trivial cases.

### Priority 2: Iterator Chain Fusion

**What exists (surprisingly complete):**
- AST types: `IteratorChainExpr`, `IteratorOp` with 14 operation types
- Semantic analysis: ~1800 LOC across `iterator.go`, `iterator_chain.go`, `iterator_enhanced.go`
- DJNZ codegen: works for arrays <= 255 elements
- Lambda-to-function transform: works

**What's missing:**
- Parser doesn't emit `IteratorChainExpr` nodes — method chains like `.map().filter()` parse as regular `CallExpr`
- Fusion optimizer (`pkg/optimizer/fusion.go`, 201 lines) is skeleton only — all logic is TODO
- String iteration not implemented
- `flatMap()`, `zip()` not implemented

**Gap analysis:** The missing piece is **parser wiring** — either modify the parser to emit `IteratorChainExpr`, or add a transformation pass in the semantic analyzer to convert `CallExpr` chains. The semantic analysis and codegen infrastructure is ready.

**Risk:** Iterator chains will work for simple cases (single `.map()`, `.forEach()`) but will hit register allocator bugs on complex multi-stage chains.

### Priority 3: MZD ABI Propagation (NOT recommended for compiler integration)

MZD's `abi.go` does backward register scanning from CALL/RST sites to resolve parameter values for known platform APIs (CP/M BDOS, Spectrum ROM, Agon MOS). This is a **disassembler** feature. The compiler already knows its own calling conventions at compile time — there's no useful integration point right now.

The backward scanning techniques *could* theoretically improve register liveness analysis, but that's a research project, not a practical priority.

### Priority 4: Function Overloading

Listed in roadmap as "quick win, ~10 hours" but audit shows `overload_resolution.go` is **already implemented**. This may be a stale roadmap item.

---

## Recommended Order

| # | Task | Effort | Why |
|---|------|--------|-----|
| 1 | **Fix register allocator bugs** | High | Unblocks everything, currently breaks complex programs |
| 2 | **Wire iterator chains to parser** | Medium | Most infrastructure exists, high-visibility feature |
| 3 | **Implement fusion optimizer** | Medium | Turns iterator chains from "works" to "fast" |
| 4 | **LSP server** | High | Best developer experience improvement |
| 5 | **Generator syntax** | Medium | Builds on iterator infrastructure |

---

## Key Architecture Findings

### Parser: Participle (DONE)
- Pure Go, zero external deps
- 97% parse rate on examples
- Tree-sitter fully removed (Feb 11, 2026)
- No OOM issues

### Iterator Infrastructure: ~75% Complete
```
AST types ........... DONE (14 operations)
Semantic analysis ... DONE (~1800 LOC)
DJNZ codegen ........ DONE (arrays <= 255)
Lambda transform .... DONE
Parser wiring ....... MISSING (chains parsed as CallExpr)
Fusion optimizer .... SKELETON (all logic TODO)
String iteration .... MISSING
zip/flatMap ......... MISSING
```

### Codegen Bugs: The Real Bottleneck
The register allocator and `loadToHL` bugs affect the Z80 backend fundamentally. They're not iterator-specific — they break any program with:
- Multiple live variables in a loop
- Arithmetic in multi-expression contexts
- Heavy register pressure

Simple programs work. Complex ones hit edge cases.

---

*Report generated during CLAUDE.md audit. Corrections applied to CLAUDE.md in same commit.*
