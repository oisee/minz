# PGO Mid Wins Implementation Plan 🚀

## Mid Win Strategy: 3-6 Week Improvements (20-40% Performance Gains)

### 🎯 MW1: Real TAS Profile Integration
**Effort**: 1 week
**Impact**: 25-30% better optimization decisions

#### Implementation Steps:
1. **TAS File Parser** (`pkg/tas/profile_loader.go`)
   - Parse binary .tas format with execution history
   - Extract block execution counts from cycle events
   - Build branch prediction statistics
   - Identify hot loops via SMC event patterns

2. **Profile Aggregation** 
   - Merge multiple TAS runs for statistical significance
   - Weight recent runs higher (temporal locality)
   - Detect phase changes in program behavior

3. **Advanced Metrics**
   ```go
   type ProfileMetrics struct {
       BlockFrequency   map[uint16]float64  // Normalized 0-1
       BranchBias       map[uint16]float64  // -1 to +1 (never/always taken)
       LoopDepth        map[uint16]int      // Nesting level
       WorkingSet       []uint16            // Hot memory pages
       SMCHotspots      []uint16            // Self-modifying locations
   }
   ```

### 🎯 MW2: Intelligent Code Layout Optimization
**Effort**: 1 week  
**Impact**: 30-35% reduction in memory contention (ZX Spectrum)

#### Z80 Memory Architecture Awareness:
1. **ZX Spectrum Contended Memory Map**
   ```
   0x4000-0x7FFF: CONTENDED (ULA steals cycles)
   0x8000-0xFFFF: UNCONTENDED (full speed)
   ```
   - Hot functions → 0x8000+ 
   - Cold error handlers → 0x4000+
   - Saves 3-4 T-states per memory access!

2. **CP/M Page Zero Optimization**
   ```
   0x0000-0x00FF: FAST (page zero addressing)
   0x0100-0xFFFF: NORMAL 
   ```
   - Most frequent variables → page zero
   - RST vectors for hot functions
   - Saves 1 byte + 3 T-states per access!

3. **Smart Function Ordering Algorithm**
   ```go
   func OptimizeLayout(functions []Function, profile ProfileMetrics) {
       // 1. Sort by execution frequency
       // 2. Pack hot functions together (cache locality)
       // 3. Place cold functions in contended memory
       // 4. Align loop starts to 16-byte boundaries
   }
   ```

### 🎯 MW3: Branch Prediction & Jump Table Optimization
**Effort**: 1 week
**Impact**: 20-25% speedup in control flow

#### Branch Optimization Techniques:
1. **Static Branch Prediction**
   ```z80
   ; BEFORE: Default fall-through
   JP Z, cold_path     ; 10 T-states (taken)
   hot_path:           ; 7 T-states (not taken)
   
   ; AFTER: Profile-guided layout  
   JP NZ, hot_path     ; 7 T-states (likely not taken)
   cold_path:          ; 10 T-states (rarely taken)
   ```

2. **Jump Table Generation**
   ```z80
   ; For switch with >5 hot cases
   LD HL, jump_table
   LD A, (selector)
   ADD A, A           ; x2 for word addresses
   LD E, A
   LD D, 0
   ADD HL, DE
   LD E, (HL)
   INC HL
   LD D, (HL)
   EX DE, HL
   JP (HL)            ; 17 T-states total!
   ```

3. **Conditional Move Patterns**
   ```z80
   ; Replace branches with arithmetic (branchless)
   ; if (a < b) x = y; else x = z;
   CP B               ; Compare
   SBC A, A           ; 0xFF if carry, 0x00 otherwise  
   AND (Y - Z)        ; Conditional difference
   ADD A, Z           ; Final value
   ```

### 🎯 MW4: Loop Optimization Suite
**Effort**: 1.5 weeks
**Impact**: 40% speedup in hot loops

#### Loop Transformations:
1. **DJNZ Optimization** (Already partially done)
   ```z80
   ; Original: 24 T-states per iteration
   loop:
     LD A, (HL)
     INC HL
     DEC B
     JP NZ, loop
   
   ; Optimized: 13 T-states per iteration!
   loop:
     LD A, (HL)
     INC HL
     DJNZ loop      ; Magic instruction!
   ```

2. **Loop Unrolling** (Profile-Guided)
   ```z80
   ; Unroll hot loops by factor of 2/4/8
   ; Based on iteration count from profile
   loop_unrolled_4:
     LD A, (HL) : INC HL
     LD A, (HL) : INC HL  
     LD A, (HL) : INC HL
     LD A, (HL) : INC HL
     DEC C
     JP NZ, loop_unrolled_4
   ```

3. **Loop Invariant Code Motion**
   ```minz
   // BEFORE: Computation inside loop
   for i in 0..n {
       x = expensive_const() * array[i]  // ❌
   }
   
   // AFTER: Hoist invariant
   let const_val = expensive_const()     // ✅
   for i in 0..n {
       x = const_val * array[i]
   }
   ```

4. **Strength Reduction**
   ```z80
   ; Replace MUL with shifts/adds for powers of 2
   ; x * 10 → (x << 3) + (x << 1) → x*8 + x*2
   LD A, X
   RLCA : RLCA : RLCA  ; x*8
   LD B, A
   LD A, X  
   RLCA                 ; x*2
   ADD A, B             ; x*10 in 14 T-states vs 100+!
   ```

### 🎯 MW5: Smart Register Allocation
**Effort**: 1 week
**Impact**: 15-20% reduction in memory access

#### Register Pressure Management:
1. **Profile-Guided Spilling**
   ```go
   // Spill cold variables first
   func AllocateRegisters(vars []Variable, profile ProfileMetrics) {
       // Sort by access frequency * loop depth
       priority := func(v Variable) float64 {
           freq := profile.BlockFrequency[v.Block]
           depth := profile.LoopDepth[v.Block]
           return freq * math.Pow(10, float64(depth))
       }
   }
   ```

2. **Shadow Register Utilization**
   ```z80
   ; Use shadow registers for hot inner loops
   EXX              ; Switch BC/DE/HL with BC'/DE'/HL'
   inner_loop:
     ; Use BC', DE', HL' freely
   EXX              ; Switch back
   ```

3. **IX/IY Strategic Usage**
   ```z80
   ; Reserve IX/IY for struct pointers in hot paths
   LD IX, object_ptr
   LD A, (IX+field_offset)  ; Direct struct access
   ```

### 🎯 MW6: Inline Expansion Intelligence
**Effort**: 1 week
**Impact**: 10-15% call overhead reduction

#### Smart Inlining Decisions:
1. **Cost Model**
   ```go
   func ShouldInline(fn Function, callSite CallSite, profile ProfileMetrics) bool {
       callFreq := profile.BlockFrequency[callSite.PC]
       fnSize := len(fn.Instructions)
       
       // Hot + Small = Always inline
       if callFreq > 0.8 && fnSize < 10 {
           return true
       }
       
       // Cold = Never inline (code size matters)
       if callFreq < 0.1 {
           return false
       }
       
       // Cost/benefit analysis
       callCost := 17  // CALL + RET T-states
       benefit := callFreq * float64(callCost)
       cost := float64(fnSize * 4)  // Average instruction size
       
       return benefit > cost
   }
   ```

2. **Partial Inlining**
   ```minz
   // Original function
   fun process(x: u8) -> u8 {
       if x == 0 { return 0 }  // Fast path
       return complex_computation(x)  // Slow path
   }
   
   // After partial inlining at hot call sites
   // Fast path inlined, slow path remains function call
   ```

## 📊 Mid Win Performance Projections

| Optimization | Spectrum | CP/M | MSX | Generic Z80 |
|-------------|----------|------|-----|-------------|
| MW1: Real TAS Profiles | +10% | +10% | +10% | +10% |
| MW2: Code Layout | +30% | +15% | +20% | +10% |
| MW3: Branch Optimization | +15% | +15% | +15% | +15% |
| MW4: Loop Suite | +25% | +25% | +25% | +25% |
| MW5: Register Allocation | +10% | +10% | +10% | +10% |
| MW6: Smart Inlining | +8% | +8% | +8% | +8% |
| **Combined Effect** | **40-50%** | **35-40%** | **35-45%** | **30-35%** |

## 🔧 Implementation Priority Order

### Phase 1: Foundation (Week 1-2)
1. **MW1**: Real TAS profile loading ← Enables everything else
2. **MW2**: Code layout optimization ← Biggest platform-specific win

### Phase 2: Control Flow (Week 3-4)  
3. **MW3**: Branch prediction ← Complements layout
4. **MW6**: Smart inlining ← Reduces branch pressure

### Phase 3: Deep Optimization (Week 5-6)
5. **MW4**: Loop optimization suite ← Highest complexity, highest reward
6. **MW5**: Register allocation ← Final polish

## 🎯 Success Metrics

### Benchmark Suite Targets:
- **Dhrystone**: 35% improvement (control flow heavy)
- **Sieve of Eratosthenes**: 45% improvement (loop heavy)
- **Bubble Sort**: 50% improvement (branch + loop)
- **Matrix Multiply**: 40% improvement (register pressure)
- **Game Loop**: 30% improvement (mixed workload)

### Real-World Impact:
- **ZX Spectrum Games**: 2-3 extra sprites without slowdown
- **CP/M Applications**: 40% faster compile times
- **MSX**: Smooth scrolling at 50fps (was 30fps)

## 🚀 Beyond Mid Wins: Slow Win Preview

### SW1: Whole Program Optimization (2 months)
- Cross-module inlining
- Global dead code elimination  
- Link-time optimization

### SW2: Auto-Vectorization for Z80 (3 months)
- SIMD-style parallel operations
- 16-bit operation fusion
- String instruction optimization

### SW3: Speculative Optimization (3 months)
- Runtime code generation
- Adaptive recompilation
- Profile-guided deoptimization

## 💡 Revolutionary Insight

**The Z80 is more sophisticated than modern devs realize!**

With proper PGO, we can achieve performance that rivals hand-written assembly while maintaining high-level abstractions. The combination of:
- Cycle-perfect profiling (TAS)
- Platform-aware optimization (contended memory)
- Smart instruction selection (DJNZ, EXX, etc.)
- Self-modifying code (TSMC)

Creates a compiler that understands vintage hardware better than most humans!

## 🎬 Next Action Items

1. Start with MW1: Parse real TAS files
2. Implement MW2: Platform-specific layout
3. Create benchmark suite for validation
4. Document performance improvements
5. Share results with retro computing community

---
*"Optimization isn't about making code fast - it's about removing what makes it slow"* 🚀