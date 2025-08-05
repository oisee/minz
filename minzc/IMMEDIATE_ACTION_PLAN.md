# MinZ Immediate Action Plan
## Quick Wins + Strategic Progress

*Created: August 5, 2025*

## 🚀 Two-Week Sprint Plan

### Week 1: Quick Platform & Stability Wins

#### Days 1-2: i8080 Backend (Easy Win!)
**Why**: Minimal effort, maximum impact
- Fork Z80 backend, remove unsupported features:
  - No IX/IY registers
  - No shadow registers (EXX, EX AF,AF')
  - No DJNZ, JR instructions
  - No bit operations (SET, RES, BIT)
- **Platforms unlocked**: CP/M, Altair 8800, IMSAI 8080, Intel MDS
- **Effort**: 2 days (mostly deletions!)
- **Impact**: Another major platform family

#### Days 3-4: Core Stability Fixes
**Why**: Unblocks 20%+ more examples

1. **Add missing standard library** (2 days)
   ```minz
   // These are referenced but missing:
   fun print_u8(value: u8) -> void
   fun print_u16(value: u16) -> void  
   fun print_string(str: String) -> void
   fun mem_copy(dest: *u8, src: *u8, len: u16) -> void
   ```

#### Days 5-7: C Backend Prototype (Fun + Useful!)
**Why**: Debugging, portability, education
```c
// MinZ → C translation for:
// - Instant portability to any platform
// - Debugging MinZ semantics
// - Performance comparisons
// - Teaching tool
```

Features for MVP:
- Basic types (u8, u16, i8, i16, bool)
- Functions and parameters
- Control flow (if/else, while, for)
- SMC annotations as comments
- Simple struct support

### Week 2: Optimization Showcase + Backend Toolkit

#### Days 8-9: Zero-Page SMC for 6502
**Why**: Prove our optimization story
```asm
; Before: Regular parameter passing
LDA param_value
STA param_location

; After: Zero-page SMC
param: .byte $00  ; In zero page!
LDA param        ; 1 cycle faster!
```
- Implement zero-page allocation
- Add SMC patching for zp addresses
- Benchmark improvements
- **Expected**: 25-30% performance boost

#### Days 10-11: Backend Development Toolkit
**Why**: Accelerate future backend development
```go
// Shared infrastructure:
type BackendToolkit struct {
    RegisterAllocator
    InstructionSelector  
    PeepholeOptimizer
    StackFrameManager
}
```
- Extract common patterns from existing backends
- Create backend generator template
- Documentation generator
- Test suite framework

#### Days 12-14: Integration & Polish
- Test all new backends
- Update documentation
- Create demo programs
- Performance benchmarks

## 📊 Success Metrics

### Quantitative Goals
- ✅ 6 backends → 8 backends (Z80, 6502, WASM, GB, 68000, i8080, C)
- ✅ 60% examples compile → 80% compile
- ✅ 0 stdlib functions → 10+ essential functions
- ✅ Const declarations: Already working! ✓

### Qualitative Goals
- ✅ C backend provides "view source" for MinZ semantics
- ✅ i8080 opens entire CP/M ecosystem
- ✅ Zero-page SMC proves optimization capabilities
- ✅ Backend toolkit accelerates future development

## 🎯 Following Sprint (Weeks 3-4)

### Continue Stability March
1. **Fix interface self parameter**
2. **Complete @if implementation** 
3. **Module system basics**
4. **Better error messages**

### Advanced Backends (Choose 1-2)
- **eZ80**: Use our u24 types!
- **65816**: SNES support
- **ARM**: Raspberry Pi demo
- **x86 real mode**: DOS nostalgia

### Performance Revolution
- Register allocation framework
- Stack-based locals
- Peephole optimizer improvements

## 💡 Why This Plan Works

1. **Quick Wins Build Momentum**
   - i8080 in 2 days shows rapid platform expansion
   - Const fix immediately helps users
   - C backend is fun and attracts attention

2. **Strategic Progress**
   - Backend toolkit pays dividends forever
   - Stdlib functions unblock real projects
   - Zero-page SMC validates our optimization claims

3. **Balanced Approach**
   - Stability fixes (const, stdlib)
   - New platforms (i8080, C)
   - Performance (zero-page SMC)
   - Infrastructure (toolkit)

## 📅 Timeline Summary

**Week 1:**
- Mon-Tue: i8080 backend ✓
- Wed-Thu: Const + stdlib ✓
- Fri-Sun: C backend prototype ✓

**Week 2:**
- Mon-Tue: Zero-page SMC ✓
- Wed-Thu: Backend toolkit ✓
- Fri-Sun: Integration & polish ✓

**Result**: MinZ v0.9.6 with 8 backends, 80% example compatibility, and proof of concept for novel optimizations!

## 🚦 Go/No-Go Decision Points

After each milestone, evaluate:
1. Is this providing value?
2. Should we pivot priorities?
3. What did we learn?

The plan is flexible - we can adjust based on discoveries and feedback!

---

*"Move fast and ship working compilers!"* 🚀