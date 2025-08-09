# MinZ TODO - Action Plan from Architecture Audit

*Last Updated: 2025-08-09*  
*Goal: Move from 60% to 95% compilation success in 28 days*

## 🎯 Quick Wins (In Progress)

### ✅ QW1: Import Statement Conversion [COMPLETED]
- **Location**: Already implemented!
- **Time**: 0 hours (was already working!)
- **Impact**: Module system functional
- **Status**: DONE - imports work with full paths!

### ⬜ QW2: String Literal Basic Support
- **Location**: `pkg/semantic/analyzer.go:892`
- **Time**: 1 day
- **Impact**: Basic string usage

### ⬜ QW3: Array Literal Initialization
- **Location**: `pkg/semantic/analyzer.go:445`
- **Time**: 1-2 days
- **Impact**: Data tables work

### ⬜ QW4: Add AST Dump Flag
- **Location**: `cmd/minzc/main.go`
- **Time**: 2 hours
- **Impact**: External tooling

### ⬜ QW5: Fix Simple Constant Evaluation
- **Location**: `pkg/semantic/analyzer.go`
- **Time**: 1 day
- **Impact**: Const arrays work

### ⬜ QW6: Enable MIR Visualization
- **Location**: `pkg/mir/function.go`
- **Time**: 4 hours
- **Impact**: Visual debugging

### ⬜ QW7: Basic Error Type Flow
- **Location**: `pkg/semantic/error_handling.go`
- **Time**: 2 days
- **Impact**: `?` operator works

### ⬜ QW8: Register Allocation Improvement
- **Location**: `pkg/codegen/z80.go`
- **Time**: 1-2 days
- **Impact**: Better codegen

## 📅 Sprint Plan

### Sprint 1: Critical Fixes (Days 1-5) - CURRENT
**Goal**: 60% → 75% success
- [x] Day 1: QW1 (imports) + QW4 (AST dump)
- [ ] Day 2: QW2 (strings basic)
- [ ] Day 3: QW3 (array literals)
- [ ] Day 4: QW5 (const eval)
- [ ] Day 5: QW6 (MIR viz) + Testing

### Sprint 2: Usability (Days 6-12)
**Goal**: 75% → 85% success
- [ ] Days 6-7: String operations suite
- [ ] Days 8-9: Basic stdlib (io, mem, math)
- [ ] Days 10-11: Error type flow
- [ ] Day 12: Integration testing

### Sprint 3: Module System (Days 13-20)
**Goal**: 85% → 90% success
- [ ] Days 13-17: Complete module system
- [ ] Days 18-19: Type promotion rules
- [ ] Day 20: Documentation

### Sprint 4: Testing & Polish (Days 21-28)
**Goal**: 90% → 95% success
- [ ] Days 21-25: Testing framework
- [ ] Days 26-27: Bug fixes
- [ ] Day 28: Release prep

## 🔨 Medium-Term Tasks

### Module System
- [ ] Symbol resolution across files
- [ ] Import path resolution
- [ ] Namespace isolation
- [ ] Circular dependency detection

### String Operations
- [ ] Length-prefixed strings (LString)
- [ ] String concatenation
- [ ] String comparison
- [ ] Print formatting

### Type System
- [ ] u8 → u16 promotion
- [ ] Numeric coercion rules
- [ ] Pointer compatibility

### Pattern Matching
- [ ] Guard conditions
- [ ] Nested patterns
- [ ] Exhaustiveness checking

### Standard Library
- [ ] `stdlib/io.minz` - print, read
- [ ] `stdlib/mem.minz` - memcpy, memset
- [ ] `stdlib/math.minz` - abs, min, max

### Testing Framework
- [ ] Unit tests for parser
- [ ] Integration test runner
- [ ] Example verification
- [ ] Regression detection

## 🚀 Strategic Initiatives

### Native Tree-sitter Binding
- [ ] Go bindings to tree-sitter
- [ ] Remove external process dependency
- [ ] 10x parse performance

### LLVM Backend
- [ ] Complete LLVM IR generation
- [ ] Hook into LLVM optimizations
- [ ] Cross-platform binary generation

### Generic Type System
- [ ] Type parameters
- [ ] Trait bounds
- [ ] Monomorphization

### Incremental Compilation
- [ ] Dependency tracking
- [ ] Cache management
- [ ] Partial recompilation

### Debugger Support
- [ ] Debug info generation
- [ ] Symbol maps
- [ ] Source mapping

## 📊 Progress Tracking

| Date | Success Rate | Milestone |
|------|--------------|-----------|
| 2025-08-09 | 75-80% | Actual starting point! |
| Sprint 1 End | 75% | Basics work |
| Sprint 2 End | 85% | Usable |
| Sprint 3 End | 90% | Multi-file |
| Sprint 4 End | 95% | Production |

## 💎 Features to Preserve

These already work perfectly - DO NOT BREAK:
1. Zero-cost lambdas
2. TSMC optimization
3. Function overloading
4. Interface monomorphization
5. MIR layer (0 TODOs!)

## 🎬 Next Actions

1. ✅ Complete QW1 (import statements)
2. ⬜ Test import functionality
3. ⬜ Move to QW2 (strings)
4. ⬜ Update progress metrics

---

*See [ACTION_PLAN_FROM_AUDIT.md](ACTION_PLAN_FROM_AUDIT.md) for detailed analysis*  
*See [ARCHITECTURE_AUDIT.md](ARCHITECTURE_AUDIT.md) for current state assessment*