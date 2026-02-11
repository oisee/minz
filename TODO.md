# MinZ TODO - Priority Roadmap

> **Last Updated:** 2026-02-11
> **Version:** v0.16.0-dev (Participle Parser)
> **Parse Rate:** 100% (116/116) | **Compile Rate:** 90.5% (105/116)
> **Parser:** Native Go (Participle) - tree-sitter replaced

---

## 🚨 P0: Critical Blockers (This Week)

These issues block basic usage and must be fixed immediately.

### 1. Document tree-sitter Setup Requirement
**Status:** ✅ DONE (in README update)
**Issue:** Users get cryptic errors without `tree-sitter init-config`
**Fix:** Added to README Quick Start section

### 2. Filter ANSI Codes from tree-sitter Output
**Status:** ✅ DONE
**Issue:** Tree-sitter warnings with ANSI codes cause parse failures
**Fix:** Added regex to strip ANSI codes + filter warning lines in parser.go
**Commit:** c54347f

### 3. Fix Pre-built Binary Architecture
**Status:** ✅ DONE
**Issue:** Binaries in repo were macOS ARM64, failed on Linux
**Fix:** Removed tracked binaries from repo, added to .gitignore. Users build from source.
**Commit:** 1e3a932

---

## 🔥 P1: High Priority (Next 2 Weeks)

Features that are documented/claimed but don't work.

### 4. Array Literal → DB Directive Generation
**Status:** ✅ DONE
**Issue:** Was generating redundant element stores alongside DB directive
**Fix:** Skip per-element stores when all elements are literals (DB handles it)
**Result:** `[10,20,30]` → clean `DB 10, 20, 30` with no redundant code
**Commit:** 31a41e3

### 5. Basic Pattern Matching
**Status:** ✅ WORKING (syntax clarification needed)
**Issue:** User error - `return` used outside block context
**Working syntax:**
- Expression: `State.IDLE => State.RUNNING` (direct value)
- Block: `State.IDLE => { return State.RUNNING; }` (with explicit return)
**NOT working:** `State.IDLE => return State.RUNNING` (return is statement, not expression)
**Note:** Rust-style `::` not supported - use dot notation
**Commit:** Verified Dec 2025

### 6. Enum Value Access (State.IDLE)
**Status:** ✅ DONE (dot syntax)
**Issue:** Was failing for enum value access
**Fix:** Fixed during error propagation work (enum resolution in semantic analyzer)
**Working:** `State.IDLE` syntax works correctly
**Note:** `State::IDLE` (Rust-style) not supported - use dot notation
**Commit:** Part of v0.15.2 error propagation

### 7. Function Pointer Passing
**Status:** 🟡 PARKED - Use lambdas instead
**Decision:** Runtime function pointers are poor fit for Z80 (indirect call overhead)
**Alternative:** Zero-cost lambdas already work! Use `.map(|x| x * 2)` syntax
**Future:** May add compile-time monomorphization if needed

---

## 🔧 P2: Medium Priority (Next Month)

Important features for language completeness.

### 8. Error Propagation (`?` operator)
**Status:** ✅ Core functionality working!
**Working:**
- Error type enums with CY flag + A register ABI
- `? ErrorType` return type syntax
- Z80 codegen: `SCF` for error, `OR A` for success, `JR NC` for checking
- Manual error handling with inline `asm { SCF }` works
- **`@error(code)` metafunction** - NEW! Sets CY flag and returns
**Remaining polish:**
- `?? @error` propagation syntax (minor cleanup)
- Enum value access (`ErrorType.Value` syntax)
- Type inference for `?`-suffixed function calls
**Effort:** Polish items ~4 hours each

### 9. Iterator Chain Optimization (DJNZ)
**Status:** ✅ MOSTLY DONE (3/4 issues fixed)
**Current State:**
- DJNZ instruction IS generated ✅
- Loop structure works ✅
- Labels properly emitted ✅
**Issues Fixed (Dec 2025):**
1. ✅ `djnz_loop_1:` label bug - DCE and sanitizeLabel fixes (commit 533b816)
2. ✅ `x * 2` now generates `SLA A` - peephole Imm optimization (commit 84331b4)
3. ✅ `x > 25` now generates `CP 26; JR C` - 8-bit comparison opt (commit 7270030)
4. 🟡 Register allocation still suboptimal (future work)
**Results:** Lambda code reduced from ~15 to ~4-7 instructions
**Remaining:** Full loop fusion, register optimization

### 10. Module System stdlib
**Status:** 🟡 Partial
**Issue:** `import math` parses but `math.abs` undefined
**Fix:** Need to implement stdlib modules
**Effort:** 1 week

### 11. Crystal Backend Completeness
**Status:** 🟡 Partial
**Issue:** Generates code but missing struct definitions
**Effort:** 2-3 days

---

## 📋 P3: Lower Priority (Future)

Nice to have, not blocking.

### 12. Local/Nested Functions
**Status:** ✅ PARSING WORKS (v0.16)
**Note:** Parser supports `fun inner() {}` inside functions
**Remaining:** Semantic analysis (scoping, captures)

### 13. Self Parameter in Methods
**Status:** 🔴 Not working
**Issue:** `impl` blocks with `self` don't compile
**Effort:** 2-3 days

### 14. Generic Types
**Status:** 🟢 PARKED - See [ADR-002](docs/ADR_002_Generics_Parked_Crystal_Interfaces.md)
**Decision:** Use Crystal-style `Type(T)` + Zero-Cost Interfaces instead of Rust `<T>`
**Alternative:** Function overloading + @define macros (already working!)

### 15. LSP Server
**Status:** 🔴 Not started
**Effort:** 2-3 weeks

### 16. Loop Reroll Generic Transformation
**Status:** 🟡 Detection working, transform TODO
**Issue:** Currently only transforms putchar → print_string; other patterns just annotated
**Working:**
- Multi-parameter pattern detection (1-7 params) ✅
- Captures all parameter values correctly ✅
- Putchar → print_string transformation ✅
**TODO:**
- Generic transformation: data table + loop for arbitrary repeated calls
- Example: `plot(10,20); plot(15,25);...` → `data: DB 10,20,15,25,...` + loop calling plot
**Effort:** 1-2 days
**Tag:** SIZE optimization

---

## 🧹 Participle Parser (v0.16) ✅ COMPLETE

Native Go parser replaces tree-sitter. No external dependencies.

### Final Stats
| Metric | Result |
|--------|--------|
| Parse rate | **100%** (116/116 examples) |
| Compile rate | **90.5%** (105/116 examples) |
| Corpus (old snapshots) | 82.4% (expected - old syntax) |

### Syntax Rules
| Feature | Syntax | Notes |
|---------|--------|-------|
| Function declaration | `fun foo() { }` | `fn` is for types only |
| Function type | `fn(u8) -> u8` | Higher-order functions |
| Error function | `fun name?() -> T ! Error` | `?` in name, `!` for error type |
| Error propagation | `value?`, `get()?` | Postfix `?` operator |
| Conditionals | `if cond { a } else { b }` | Ternary `?:` removed |
| Array literals | `[1, 2, 3]` | NOT `{1, 2, 3}` |
| Array types | `[u8; 10]` | Rust-style |
| Pattern match | `case val { Pat => result }` | Also `match` keyword |
| Attributes | `@[inline]`, `@[abi("smc")]` | Bracket form only |
| Local functions | `fun outer() { fun inner() {} }` | Nested declarations |
| Asm interpolation | `asm { LD HL, {var} }` | Variable substitution |

### Files Updated ✅
- [x] 52 functions: `fn` → `fun` in test_files/ and benchmarks/
- [x] 6 examples: `{1,2,3}` → `[1,2,3]` array syntax
- [x] error_propagation_demo: `? Error` → `! Error`
- [x] tsmc files: `@abi()` → `@[abi()]`
- [x] Converted `asm fun` → `fun { asm { } }`

### Remaining Experimental (18 files in examples/experimental/)
- `bits_8 { }` bitfield types
- `loop indexed` iteration
- Complex nested `case` expressions
- `@lua[[[...]]]` blocks

---

## ✅ Recently Completed

- [x] **Loop Reroll Multi-Parameter Patterns** (2026-02-11)
  - Pattern detection for 1-7 parameter function calls
  - Correct IR pattern order: p1, p2, p1_dup, p2_dup, Call (interleaved)
  - Captures all parameter values per repeat correctly
  - Putchar → print_string transformation working
  - Tagged as SIZE optimization
  - TODO: Generic transformation (data table + loop) for non-putchar patterns
- [x] **Participle parser v0.16** - 100% parse, 90.5% compile (2026-02-08)
  - Native Go parser replaces tree-sitter (no external deps)
  - `fun`/`fn` separation, `!` for error types, `case`/`match` expressions
  - Local functions, asm interpolation, attribute syntax
- [x] **@error(code) metafunction** - Error propagation working! (2025-12-17)
- [x] Ruby string interpolation (`#{var}`) - Working
- [x] CTIE compile-time execution - Working
- [x] Struct literals and field access - Working
- [x] For/while loops - Working
- [x] Global variables - Working
- [x] Multi-backend support (Z80, C, Crystal, WASM) - Working
- [x] Tree-sitter parser - REPLACED by Participle (v0.16)
- [x] Claims verification report - Done

---

## 📊 Success Metrics

### Current State (v0.16)
| Metric | Value |
|--------|-------|
| Examples parsing | 116/116 (100%) |
| Examples compiling | 105/116 (90.5%) |
| Parser | Native Go (Participle) |
| External deps | None (tree-sitter removed) |

### Target (v1.0)
| Metric | Target |
|--------|--------|
| Examples compiling | 95%+ |
| Core features working | 100% |
| Documentation accuracy | 100% |

---

## 🗓 Suggested Sprint Plan

### Week 1: Critical Fixes
- [x] P0 #2: Filter ANSI codes (DONE - c54347f)
- [x] P0 #3: Fix binary architecture (DONE - removed from git)
- [x] P1 #4: Array literal DB generation (DONE - clean output)
- [x] P1 #6: Enum value access (DONE - State.IDLE works!)

### Week 2: Core Features
- [x] P1 #5: Pattern matching (WORKS - syntax clarification)
- [ ] P1 #7: Function pointer passing (1-2 days)

### Week 3-4: Language Completeness
- [ ] P2 #8: Error propagation (2-3 days)
- [ ] P2 #10: Module stdlib (1 week)

---

## 🔗 Key Documents

- [Claims Verification Report](reports/2025-12-17-001-claims-verification.md)
- [Complete Language Specification](docs/230_MinZ_Complete_Language_Specification.md)
- [Internal Architecture](minzc/docs/INTERNAL_ARCHITECTURE.md)
- [CTIE Implementation](docs/178_CTIE_Working_Announcement.md)

---

## 💡 Principles

1. **Honesty First** - Don't claim features that don't work
2. **Test Before Commit** - Verify claims with actual compilation
3. **Document Gaps** - Mark features as "in progress" not "working"
4. **Fix Blockers First** - P0 before P1 before P2
5. **User Experience** - Setup should be obvious and work first try

---

*This TODO reflects actual verified state as of 2025-12-17. See claims verification report for methodology.*
