# MinZ Stability Roadmap v2
## Path to v1.0: Quick Wins & Strategic Improvements

*Last Updated: August 13, 2025*

## 🎯 Current State (v0.14.0)

### Success Metrics
- **75% Compilation Success** with ANTLR parser (111/148 examples)
- **Zero Dependencies** - Self-contained binaries
- **Multi-Backend Support** - Z80, 6502, C, WASM, LLVM

### Tool Ecosystem Status
- ✅ **mz** - Compiler (working, formerly minzc)
- ✅ **mza** - Assembler (working)
- ✅ **mze** - Emulator (working)
- ✅ **mzr** - REPL (working, formerly minzr)
- 🚧 **mzv** - MIR VM (proposed)

## 🚀 Quick Wins (Hours to Days)

### 1. Array Literals Support (4 hours) ⚡
**Problem**: Can't write `[1, 2, 3]` directly
**Solution**: Add array literal parsing in ANTLR grammar
**Impact**: Enables 10+ more examples to compile
```minz
// Currently broken:
let nums = [1, 2, 3, 4, 5];  // ❌

// After fix:
let nums = [1, 2, 3, 4, 5];  // ✅
```

### 2. Enum Value Access (2 hours) ⚡
**Problem**: Can't access enum values properly
**Solution**: Fix enum member resolution in semantic analyzer
**Impact**: Enables state machines and enum-based code
```minz
enum State { IDLE = 0, RUNNING = 1, DONE = 2 }
let s = State::IDLE;  // Currently broken
```

### 3. Fix Debug Output (30 minutes) ⚡
**Problem**: Debug messages appearing in production
**Solution**: Remove or guard debug Printf statements
**Impact**: Clean compiler output

### 4. String Escape Sequences (2 hours) ⚡
**Problem**: Can't use `\n`, `\t`, `\"` in strings
**Solution**: Add escape sequence processing in lexer
**Impact**: Proper string handling
```minz
print("Hello\nWorld");  // Currently prints literally
```

### 5. MIR VM Tool - mzv (1 day) ⚡
**Problem**: Can't execute .mir files directly
**Solution**: Create mzv tool for MIR execution
**Impact**: Platform-independent code execution
```bash
mz program.minz -o program.mir
mzv program.mir  # New tool!
```

## 🎯 Medium Wins (Days to Weeks)

### 1. Function Overloading (3 days) 🎯
**Problem**: Need different names for same operation
**Solution**: Implement name mangling and resolution
**Impact**: Major usability improvement
```minz
// Currently need:
print_u8(42);
print_u16(1000);
print_str("hello");

// After:
print(42);       // Overloaded!
print(1000);     // Same name!
print("hello");  // Natural!
```

### 2. Complete Control Flow (2 days) 🎯
**Problem**: Some control flow statements don't work
**Solution**: Fix ANTLR visitor for all control structures
**Impact**: Full language expressiveness
```minz
// Fix:
- else if chains
- do-while loops
- switch statements
- break/continue in nested loops
```

### 3. Module Imports (3 days) 🎯
**Problem**: Import system partially broken
**Solution**: Fix module resolution and symbol table
**Impact**: Proper code organization
```minz
import std.io;
import math as m;
io.println("π = ", m.PI);
```

## 🏔️ Slow Wins (Weeks to Months)

### 1. Self-Hosting Compiler (2-3 months) 🏔️
**Approach**: Parser combinators in MinZ
**Milestone 1**: Tokenizer in MinZ (2 weeks)
**Milestone 2**: Parser combinators (1 month)
**Milestone 3**: Code generation (1 month)
**Milestone 4**: Bootstrap (2 weeks)

### 2. Complete Standard Library (1-2 months) 🏔️
```minz
// Full standard library modules:
- std.io      // I/O operations
- std.math    // Mathematical functions
- std.string  // String manipulation
- std.mem     // Memory management
- std.sys     // System calls
- std.fmt     // Formatting
```

### 3. Advanced Optimizations (1 month) 🏔️
- **Peephole Optimizer**: Pattern-based optimization
- **Register Allocator**: Graph coloring algorithm
- **Dead Code Elimination**: Remove unreachable code
- **Constant Propagation**: Compile-time evaluation

### 4. IDE Support (2 months) 🏔️
- **Language Server Protocol**: Full LSP implementation
- **VS Code Extension**: Syntax highlighting, completion
- **Debugging Protocol**: DAP implementation
- **Documentation Generator**: From comments to docs

## 📋 Implementation Priority

### Week 1: Critical Quick Wins
- [ ] Array literals (4h)
- [ ] Enum access (2h)
- [ ] Debug output fix (30m)
- [ ] String escapes (2h)
- [ ] Create mzv tool (8h)

### Week 2-3: Usability
- [ ] Function overloading
- [ ] Control flow completion
- [ ] Module import fixes

### Month 2: Stability
- [ ] Test suite expansion
- [ ] Error message improvement
- [ ] Documentation update
- [ ] Performance optimization

### Month 3: Ecosystem
- [ ] Standard library modules
- [ ] Package manager design
- [ ] IDE integration start
- [ ] Self-hosting prep

## 📊 Success Metrics Target

### Current (v0.14.0)
- Compilation: 75% (111/148)
- Parser: ANTLR default
- Dependencies: Zero
- Tools: 4 (mz, mza, mze, mzr)

### Target (v0.15.0) - End of Week 1
- Compilation: 85% (126/148)
- Features: Arrays, enums, strings
- Tools: 5 (+ mzv)

### Target (v0.16.0) - End of Month 1
- Compilation: 90% (133/148)
- Features: Overloading, modules
- Stability: Production-ready core

### Target (v1.0.0) - End of Quarter
- Compilation: 95%+ (140/148)
- Self-hosting: Feasible
- Ecosystem: Complete
- Documentation: Professional

## 🎉 Why These Wins Matter

### Quick Wins Impact
- **Array Literals**: Unlocks data structure examples
- **Enums**: Enables state machines
- **mzv Tool**: Platform independence
- **Clean Output**: Professional appearance

### Medium Wins Impact
- **Overloading**: Natural, intuitive API
- **Modules**: Real program organization
- **Control Flow**: Complete expressiveness

### Slow Wins Impact
- **Self-Hosting**: Ultimate validation
- **Stdlib**: Production readiness
- **IDE Support**: Developer adoption
- **Optimizations**: Performance leadership

## 🚦 Next Actions

1. **Today**: Fix array literals (4h)
2. **Tomorrow**: Fix enums (2h) + Create mzv (8h)
3. **This Week**: Complete all quick wins
4. **Next Week**: Start function overloading
5. **This Month**: Achieve 90% compilation rate

---

*"From quick fixes to grand visions, every improvement brings MinZ closer to perfection"*