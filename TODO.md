# MinZ TODO - Priority Roadmap

> **Last Updated:** 2025-12-18
> **Version:** v0.15.4-dev
> **Compilation Success Rate:** 81% (47/58 examples)
> **Latest Session:** [reports/2025-12-18-001-djnz-optimization-session.md](reports/2025-12-18-001-djnz-optimization-session.md)

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
**Status:** 🔴 Not working
**Issue:** 7+ examples fail due to this
**Effort:** 2-3 days

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

---

## ✅ Recently Completed

- [x] **@error(code) metafunction** - Error propagation working! (2025-12-17)
- [x] Ruby string interpolation (`#{var}`) - Working
- [x] CTIE compile-time execution - Working
- [x] Struct literals and field access - Working
- [x] For/while loops - Working
- [x] Global variables - Working
- [x] Multi-backend support (Z80, C, Crystal, WASM) - Working
- [x] Tree-sitter parser integration - Working (81% success)
- [x] Claims verification report - Done

---

## 📊 Success Metrics

### Current State
| Metric | Value |
|--------|-------|
| Examples compiling | 48/59 (81%) |
| Working core features | 12/20 |
| Documented features working | ~60% |

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
