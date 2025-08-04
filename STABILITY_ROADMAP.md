# MinZ Stability Roadmap
## From Experimental to Production-Ready

*Last Updated: August 4, 2025*

## 🎯 **Current State Analysis**

Based on README analysis, MinZ has:
- **✅ 60% of examples compile** - Core language solid
- **🚧 40% need advanced features** - Stability gap identified

### **Stable Components (Ready for Production)**
- ✅ **Basic functions** - Reliable implementation
- ✅ **Core types** (u8, u16, i8, i16, bool) - Working well
- ✅ **Structs and enums** - Basic implementation stable
- ✅ **Control flow** (if/else, while, for loops) - Solid
- ✅ **Error propagation system** - Recently stabilized
- ✅ **Z80 code generation** - Generates working assembly
- ✅ **Self-modifying code (SMC)** - Advanced optimization working

### **Experimental Components (Need Stabilization)**
- 🚧 **@minz Metafunctions** - Being redesigned with @minz[[[]]] syntax
- 🚧 **@if compile-time conditional** - Partially working, needs completion
- 🚧 **Iterator chains** - Functional programming syntax  
- 🚧 **Zero-cost interfaces** - Interface system
- 🚧 **Module system** - Import/export
- 🚧 **Standard library** - Missing core functions
- 🚧 **Lambdas** - Compile-time transformation
- 🚧 **Constant declarations** - `const` keyword issues

## 📋 **Stabilization Plan by Priority**

### **Phase 1: Core Language Stability (4-6 weeks)**
*Goal: Make core language 100% reliable*

#### **Week 1-2: Fix Fundamental Issues**
1. **🔧 Fix constant declarations** 
   - `const DEBUG: bool = true` should work
   - Integrate with compile-time evaluation
   - Priority: **Critical**

2. **🔧 Complete standard library core**
   - `print_u8`, `print_u16`, `print_string` functions
   - Basic memory operations (`mem.copy`, `mem.set`)
   - String operations (`str.len`, `str.concat`)
   - Priority: **Critical**

3. **🔧 Fix interface self parameter**
   - `self` parameter resolution in interface methods
   - Method dispatch optimization
   - Priority: **High**

#### **Week 3-4: Metaprogramming Stability**
4. **🔧 Complete @if implementation**
   - Fix constant emission vs variable assignment
   - Support all expression types in branches
   - Add comprehensive tests
   - Priority: **High**

5. **🔧 Implement new @minz[[[]]] syntax**
   - Compile-time MinZ code blocks (like @lua[[[...]]])
   - Template with parameters: @minz[[[...]]](params)
   - Named metafunctions: @fun for compile-time functions
   - Clear @ prefix = compile-time rule
   - Priority: **Medium**
   - Design: [docs/132_MinZ_Metafunction_Redesign.md](docs/132_MinZ_Metafunction_Redesign.md)

#### **Week 5-6: Advanced Features**
6. **🔧 Complete lambda system**
   - Function reference copying (`let f = someFunction`)
   - Curry optimization for partial application
   - Type inference for lambda expressions
   - Priority: **Medium**

### **Phase 2: Zero-Cost Abstractions (3-4 weeks)**
*Goal: Make experimental features production-ready*

#### **Week 7-8: Iterator System**
7. **🔧 Stabilize iterator chains**
   - Complete all iterator methods (map, filter, reduce, etc.)
   - Optimize chained operations to single loops
   - Add comprehensive error handling
   - Priority: **Medium**

8. **🔧 Optimize iterator performance**
   - DJNZ loop optimization verification
   - Memory usage optimization
   - Benchmark against hand-written loops
   - Priority: **Medium**

#### **Week 9-10: Interface System**
9. **🔧 Complete zero-cost interfaces**
   - Static dispatch optimization
   - Interface casting and type erasure
   - Multiple interface implementation
   - Priority: **Medium**

10. **🔧 Add interface inheritance**
    - Interface extension syntax
    - Default method implementations
    - Priority: **Low**

### **Phase 3: Module System & Tooling (3-4 weeks)**
*Goal: Complete ecosystem for real projects*

#### **Week 11-12: Module System**
11. **📦 Implement module import system**
    - `import` and `export` keywords
    - Module path resolution
    - Circular dependency detection
    - Priority: **High**

12. **📦 Package management prototype**
    - Simple dependency resolution
    - Version management
    - Priority: **Low**

#### **Week 13-14: Developer Experience**
13. **🛠️ Improve error messages**
    - Better syntax error reporting
    - Type mismatch explanations
    - Suggestion system for common mistakes
    - Priority: **Medium**

14. **🛠️ Add debugging support**
    - Source maps for assembly debugging
    - Variable inspection in REPL
    - Priority: **Medium**

## 🎯 **Success Criteria for Stability**

### **Phase 1 Success Metrics**
- ✅ **95%+ of examples compile** (up from 60%)
- ✅ **All constant declarations work**
- ✅ **Standard library functions available**
- ✅ **@if metafunction fully working**

### **Phase 2 Success Metrics**
- ✅ **Iterator chains perform identically to hand-written loops**
- ✅ **Interface dispatch has zero runtime overhead**
- ✅ **Lambda functions compile to direct calls**

### **Phase 3 Success Metrics**  
- ✅ **Multi-file projects work with imports**
- ✅ **Developer-friendly error messages**
- ✅ **REPL supports all language features**

## 🚀 **Migration Path for Users**

### **Immediate Actions (This Week)**
1. Use stable features only:
   ```minz
   // ✅ These work reliably
   fun main() -> u8 { return 42; }
   struct Point { x: u8, y: u8 }
   if condition { do_something(); }
   ```

2. Avoid experimental features:
   ```minz
   // 🚧 These need stabilization
   const DEBUG = true;  // Fix needed
   interface Drawable  // Fix needed
   numbers.map(|x| x * 2)  // Stabilization needed
   ```

### **After Phase 1 (6 weeks)**
1. All core language features stable
2. Constants and metafunctions working
3. Basic standard library available

### **After Phase 2 (10 weeks)**
1. Zero-cost abstractions ready
2. Performance optimizations verified
3. Advanced features stable

### **After Phase 3 (14 weeks)**  
1. **Production-ready MinZ v1.0**
2. Complete module system
3. Professional developer experience

## 📊 **Implementation Status Tracking**

| Feature | Current | Phase 1 | Phase 2 | Phase 3 |
|---------|---------|---------|---------|---------|
| Basic functions | ✅ Stable | ✅ | ✅ | ✅ |
| Constants | 🚧 Broken | ✅ Fixed | ✅ | ✅ |
| Standard library | 🚧 Missing | ✅ Core | ✅ Full | ✅ |
| @if metafunction | 🚧 Partial | ✅ Complete | ✅ | ✅ |
| Lambdas | 🚧 Basic | ✅ Stable | ✅ | ✅ |
| Iterators | 🚧 Research | 🚧 | ✅ Stable | ✅ |
| Interfaces | 🚧 Broken | 🚧 | ✅ Stable | ✅ |
| Modules | ❌ Missing | ❌ | ❌ | ✅ Complete |

## 🔧 **Technical Implementation Notes**

### **Immediate Fixes Needed**
1. **Constant evaluation pipeline**: `@if` works but constants don't integrate properly
2. **Variable assignment optimization**: Constants treated as runtime variables
3. **Standard library integration**: Functions exist but not accessible
4. **Interface method resolution**: `self` parameter binding broken

### **Architecture Decisions**
1. **Keep compile-time evaluation philosophy** - It's working well
2. **Maintain zero-cost abstraction goal** - Performance is excellent 
3. **Preserve Z80 optimization focus** - SMC and register allocation working
4. **Build on existing parser/semantic foundation** - Core is solid

## 🏆 **Vision: MinZ v1.0 Production Release**

**Target: November 2025 (14 weeks)**

### **What v1.0 Will Deliver**
- ✅ **100% reliable core language** - No experimental warnings
- ✅ **Complete standard library** - All essential functions
- ✅ **Zero-cost abstractions** - Lambdas, iterators, interfaces working
- ✅ **Module system** - Multi-file projects supported
- ✅ **Professional tooling** - Great error messages, debugging support
- ✅ **Performance verified** - Benchmarks prove optimization claims

### **Success Definition**
MinZ v1.0 will be the first systems programming language to deliver:
1. **Modern syntax** with Ruby-like developer happiness
2. **Zero-cost abstractions** that actually work on 8-bit hardware
3. **Compile-time metaprogramming** for embedded systems
4. **Production-ready reliability** for real Z80 projects

---

*This roadmap represents our commitment to moving MinZ from experimental research to production-ready systems programming language for Z80 platforms.*