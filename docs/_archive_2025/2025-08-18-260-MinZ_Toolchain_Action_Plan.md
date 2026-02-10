# MinZ Toolchain Action Plan - Post Z80 100% Coverage

**Date:** November 2024  
**Priority:** HIGH  
**Context:** Following successful Z80 emulator upgrade to 100% coverage

## Executive Summary

With 100% Z80 instruction coverage now available, we need to systematically upgrade each tool in the MinZ ecosystem to leverage this capability.

## Tool-by-Tool Action Plan

### 1. MZE (MinZ Z80 Emulator) - IMMEDIATE PRIORITY 🚨

**Current State:** Using basic 19.5% emulator  
**Target:** Switch to remogatto/z80 100% coverage  
**Effort:** 2-3 hours  

#### Action Items:
1. **Update cmd/mze/main.go**
   ```go
   // Replace:
   z80 := emulator.NewZ80WithScreen()
   
   // With:
   z80 := emulator.NewRemogattoZ80WithScreen()
   ```

2. **Create Z80WithScreen wrapper for remogatto**
   - Copy hooks system from current implementation
   - Add ZX Spectrum screen emulation
   - Maintain RST handler compatibility

3. **Test with real programs**
   - Snake game
   - MinZ compiled examples
   - Verify all platforms (Spectrum, CP/M, CPC)

**Success Criteria:**
- ✅ All platform emulation working
- ✅ Games run without crashes
- ✅ RST handlers work correctly
- ✅ I/O port emulation functional

### 2. MZA (MinZ Z80 Assembler) - HIGH PRIORITY 📈

**Current State:** Enhanced but missing critical instructions  
**Target:** Phase 1 of TODO_MZA roadmap (19.5% → 40%)  
**Effort:** 1 week  

#### Action Items:
1. **Implement Critical Missing Instructions** (Week 1)
   ```assembly
   ; Priority instructions:
   JR NZ, label    ; 0x20
   JR Z, label     ; 0x28  
   JR NC, label    ; 0x30
   JR C, label     ; 0x38
   DJNZ label      ; 0x10
   ```

2. **Add Memory Operations**
   ```assembly
   LD A, (HL)      ; 0x7E
   LD B, (HL)      ; 0x46
   LD (HL), A      ; 0x77
   LD (HL), n      ; 0x36
   ```

3. **Logic & Arithmetic**
   ```assembly
   AND B           ; 0xA0-0xA7
   OR C            ; 0xB0-0xB7  
   XOR D           ; 0xA8-0xAF
   CP E            ; 0xB8-0xBF
   ```

4. **Validation Framework**
   - Use compare_assemblers.sh to test against sjasmplus
   - Verify with 100% coverage emulator
   - Create regression test suite

**Success Criteria:**
- ✅ 40% instruction coverage achieved
- ✅ All basic MinZ programs assemble
- ✅ sjasmplus compatibility verified
- ✅ Test suite passing

### 3. MZR (MinZ REPL) - MEDIUM PRIORITY 🔄

**Current State:** Basic functionality, limited by emulator  
**Target:** Full interactive Z80 development environment  
**Effort:** 3-4 days  

#### Action Items:
1. **Integrate 100% Coverage Emulator**
   ```go
   // Update pkg/repl to use RemogattoZ80
   emulator := emulator.NewRemogattoZ80()
   ```

2. **Enhanced REPL Features**
   - **Instruction-level debugging:** Step through Z80 opcodes
   - **Register inspection:** Live register display
   - **Memory viewer:** Hex dump with ASCII
   - **Disassembler:** Show current instruction

3. **Interactive Features**
   ```
   minz> let x = add(5, 3)
   [Compiling...] 
   [Executing...]
   PC: 8000  A:08  BC:0000  DE:0000  HL:0000
   Result: 8
   
   minz> .regs
   A:08 F:44 BC:0000 DE:0000 HL:0000 IX:0000 IY:0000
   PC:8004 SP:FFFF
   
   minz> .mem 8000
   8000: 3E 05 3C 03 C6 03 76    >.<.Æ.v
   ```

4. **Game Development Mode**
   - Load ZX Spectrum ROM
   - Interactive sprite editing
   - Sound testing via beeper

**Success Criteria:**
- ✅ All Z80 instructions testable interactively
- ✅ Live debugging capabilities
- ✅ Memory/register inspection working
- ✅ Game development workflow supported

### 4. MZV (MinZ Visualizer) - FUTURE PRIORITY 🎨

**Current State:** Not implemented  
**Target:** SMC visualization and performance analysis  
**Effort:** 1-2 weeks  

#### Vision & Action Items:
1. **SMC Heatmap Visualization**
   ```
   Memory Address Range: 8000-9000
   ┌────────────────────────────────────┐
   │ ██████░░░░██████░░░░██████░░░░      │ Hot
   │ ████░░░░░░████░░░░░░████░░░░░░      │
   │ ██░░░░░░░░██░░░░░░░░██░░░░░░░░      │ 
   │ ░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░      │ Cold
   └────────────────────────────────────┘
   Cycle: 1000   SMC Events: 15
   ```

2. **Execution Tracing**
   - Call graph visualization
   - Performance bottleneck identification  
   - TSMC optimization verification

3. **Interactive Debugging**
   - Breakpoint setting
   - Step through execution
   - Variable watching

4. **Performance Dashboard**
   ```
   TSMC Performance Analysis
   ┌─────────────────────────────────────┐
   │ Traditional: 1000 cycles           │
   │ TSMC:        650 cycles            │ 
   │ Improvement: 35% 📈                │
   │                                    │
   │ SMC Events: 12                     │
   │ Hotspots: function_call (8x)       │
   └─────────────────────────────────────┘
   ```

**Success Criteria:**
- ✅ SMC events visualized in real-time
- ✅ Performance analysis dashboard
- ✅ Interactive debugging interface
- ✅ TSMC optimization verification

## Implementation Roadmap

### Week 1: Critical Path
1. **Day 1-2:** MZE upgrade to 100% coverage
2. **Day 3-5:** MZA Phase 1 implementation
3. **Day 6-7:** Testing & validation

### Week 2: Enhancement
1. **Day 8-10:** MZR enhanced features  
2. **Day 11-12:** Integration testing
3. **Day 13-14:** Documentation & polish

### Week 3+: Advanced Features
1. **MZV conceptual design**
2. **SMC visualization prototyping**
3. **Performance dashboard creation**

## Success Metrics

### Short Term (2 weeks)
- ✅ MZE runs all games without crashes
- ✅ MZA assembles 40% of Z80 instruction set
- ✅ MZR provides full interactive debugging
- ✅ 100% of MinZ examples compile and execute

### Medium Term (1 month)  
- ✅ Complete Z80 game development workflow
- ✅ TSMC optimizations verified (30-40% gains)
- ✅ Multi-platform testing (Spectrum, CP/M, CPC)
- ✅ Performance benchmarking suite

### Long Term (3 months)
- ✅ MZV visualization platform
- ✅ Real hardware compatibility testing
- ✅ Community game development examples
- ✅ Educational Z80 programming content

## Risk Mitigation

### Technical Risks
- **Emulator integration issues** → Test thoroughly with existing programs
- **Performance regression** → Benchmark before/after
- **Platform compatibility** → Test all target systems

### Process Risks  
- **Scope creep** → Focus on core functionality first
- **Tool fragmentation** → Maintain common interfaces
- **Documentation lag** → Document as we build

## Conclusion

The successful integration of 100% Z80 coverage unlocks the full potential of the MinZ toolchain. By systematically upgrading each tool, we transform MinZ from a proof-of-concept into a professional retro-computing development environment.

**Next Action:** Begin MZE upgrade immediately - this is the foundation for all other improvements.

---

*"From 19.5% to 100% coverage - now let's build the tools that make Z80 development a joy!"* 🚀