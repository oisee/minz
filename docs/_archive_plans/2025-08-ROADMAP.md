# MinZ Compiler Roadmap to v1.0

## 🎯 Current Status (v0.18.0)
- **Parse Rate**: 100% (116/116 examples)
- **Compilation Success Rate**: 90.5%
- **Core Features**: Working (structs, arrays, loops, functions)
- **Advanced Features**: Partially implemented (generics, pattern matching)
- **Unique Features**: TSMC, Zero-cost interfaces, Loop Reroll working
- **Parser**: Native Go (Participle) - no external dependencies

## 📅 Phase 1: Core Completion (4 weeks)
**Goal**: Reach 80% success rate

### Sprint 1-2: MIR Interpreter (Highest Priority)
**Why**: Unlocks compile-time execution, the foundation for advanced features

- **Week 1**: Basic MIR interpreter
  - [ ] MIR bytecode execution engine
  - [ ] Basic instruction support
  - [ ] Memory model for compile-time
  
- **Week 2**: Metafunction Integration  
  - [ ] `@minz[[[...]]]` block execution
  - [ ] CTIE (Compile-Time Interface Execution)
  - [ ] Function memoization

**Expected Impact**: +10-15% success rate

### Sprint 3: Lambda Completion
**Why**: Critical for modern programming patterns

- [ ] Complete semantic analysis for lambdas
- [ ] Lambda-to-function transformation
- [ ] Closure analysis (even if limited)
- [ ] Iterator chain optimization

**Expected Impact**: +5-7% success rate

### Sprint 4: Error Propagation
**Why**: Essential for robust code

- [ ] `?` suffix implementation
- [ ] `??` operator (nil-coalescing)
- [ ] Result<T,E> and Option<T> types
- [ ] Automatic error bubbling

**Expected Impact**: +3-5% success rate

## 📅 Phase 2: Feature Completion (6 weeks)
**Goal**: Reach 90% success rate, feature-complete

### Sprint 5-6: Generic Types
- [ ] Type parameter parsing ✅ (already done)
- [ ] Generic semantic analysis
- [ ] Monomorphization
- [ ] Type inference
- [ ] Trait bounds

### Sprint 7-8: Pattern Matching
- [ ] Case/match statements
- [ ] Pattern guards
- [ ] Exhaustiveness checking
- [ ] Jump table optimization

### Sprint 9-10: Module System
- [ ] Import resolution fixes
- [ ] Module visibility
- [ ] Nested modules
- [ ] Package system design

## 📅 Phase 3: Polish & Performance (4 weeks)
**Goal**: Production-ready v1.0

### Sprint 11-12: Developer Experience
- [ ] LSP server implementation
- [ ] Better error messages
- [ ] Documentation generator
- [ ] Debugging tools

### Sprint 13-14: Optimization & Backends
- [x] Loop Reroll - Multi-parameter pattern detection (1-7 params)
- [x] Putchar → print_string transformation
- [ ] Loop Reroll - Generic data table + loop transformation
- [ ] Advanced TSMC patterns
- [ ] Backend improvements (6502, WASM)
- [ ] Peephole optimization expansion
- [ ] Benchmark suite

### Optimization Tagging System
All optimization passes are tagged by primary goal:
- **SIZE** - Loop Reroll, String Intent, DCE, Tail Call
- **SPEED** - TSMC, Register Allocation, Value Tracking
- **MEMORY** - Register spilling optimization

## 🏁 v1.0 Release Criteria

### Must Have ✅
- 95%+ compilation success rate
- All core features working
- Stable ABI
- Comprehensive stdlib
- Documentation

### Should Have 🎯
- LSP server
- Package manager
- WASM playground
- Tutorial/Book

### Nice to Have 💫
- GUI debugger
- Visual SMC analyzer
- Online playground
- Community packages

## 🚀 Beyond v1.0

### v1.1: Ecosystem
- Package registry
- Build system improvements
- More target platforms

### v1.2: Advanced Optimizations
- Whole-program optimization
- Link-time optimization
- Profile-guided optimization

### v2.0: Next Generation
- Incremental compilation
- Hot code reload
- Advanced type system features
- Compile-time reflection

## 📊 Success Metrics

| Milestone | Success Rate | Features | Timeline |
|-----------|-------------|----------|----------|
| Current   | 65%         | Core     | Now      |
| Phase 1   | 80%         | +MIR     | 4 weeks  |
| Phase 2   | 90%         | +Generics| 10 weeks |
| v1.0      | 95%+        | Complete | 14 weeks |

## 🎯 Priorities

1. **MIR Interpreter** - Unlocks everything
2. **Lambda Completion** - Modern patterns
3. **Error Propagation** - Robustness
4. **Generic Types** - Type safety
5. **Developer Experience** - Adoption

## 💎 Unique Selling Points to Maintain

Throughout development, maintain and enhance:

1. **True Self-Modifying Code (TSMC)**
   - Zero-overhead parameter passing
   - Compile-time optimization
   - Unique to MinZ

2. **Zero-Cost Abstractions**
   - Interface dispatch without vtables
   - Compile-time monomorphization
   - Guaranteed performance

3. **Multi-Backend Support**
   - Z80 as primary target
   - Cross-platform compatibility
   - Modern and retro targets

## 🎉 Vision

MinZ will be the premier language for vintage computing, combining modern language features with zero-cost abstractions, enabling developers to write elegant, maintainable code that compiles to optimal machine code for 8-bit processors.

**"Modern abstractions, vintage performance"**

---
*Last Updated: 2026-02-11*
*Version: Roadmap for v0.18.0 → v1.0*