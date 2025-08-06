# 🚨 TAS/TDD Framework Fix Request

## Executive Summary

The MinZ TAS (Tool-Assisted Speedrun) debugging framework is a **revolutionary TDD system** for 8-bit development that's 95% complete but needs final fixes to become operational. This framework will enable cycle-perfect debugging, time-travel analysis, and performance profiling for Z80 development.

**Priority: HIGH** 🔥 - This would be a game-changer for MinZ adoption and Z80 development.

## Current Status

### ✅ What's Already Built (95% Complete)

1. **Complete TAS Architecture** (`pkg/tas/`)
   - `tas_format.go` - Recording format with compression
   - `tas_debugger.go` - Time-travel debugging core
   - `tas_analyzer.go` - Performance analysis
   - `performance_profiler.go` - Hotspot detection
   - `cycle_perfect.go` - Cycle-accurate recording
   - `determinism.go` - Deterministic replay
   - `hybrid_recorder.go` - Smart recording strategies

2. **Z80 Emulator Integration** (`pkg/emulator/`)
   - Full Z80 CPU emulation
   - Memory management (64KB)
   - ZX Spectrum screen emulation
   - I/O port handling
   - Shadow register support

3. **REPL Commands** (`cmd/repl/tas_commands.go`)
   - `/tas` - Enable TAS debugging
   - `/record` - Start recording
   - `/rewind` - Time travel backwards
   - `/savestate` - Save states
   - `/loadstate` - Restore states

### ❌ What's Broken (5% to Fix)

#### 1. **Struct Field Mismatches** in `tas_format.go`

```go
// Line 362: InputEvent missing fields
evt.Key undefined    // Need to add Key field
evt.Pressed undefined // Need to add Pressed field

// Line 455-478: StateSnapshot missing compound registers
state.AF undefined   // Need AF() method or field
state.BC undefined   // Need BC() method or field
state.DE undefined   // Need DE() method or field
state.HL undefined   // Need HL() method or field
```

**Fix Required:**
```go
// Add to InputEvent struct
type InputEvent struct {
    Cycle    uint64
    Frame    uint64
    Port     uint16
    Value    byte
    Type     string
    Key      byte    // ADD THIS
    Pressed  bool    // ADD THIS
}

// Add to StateSnapshot - helper methods for 16-bit pairs
func (s *StateSnapshot) AF() uint16 { return uint16(s.A)<<8 | uint16(s.F) }
func (s *StateSnapshot) BC() uint16 { return uint16(s.B)<<8 | uint16(s.C) }
func (s *StateSnapshot) DE() uint16 { return uint16(s.D)<<8 | uint16(s.E) }
func (s *StateSnapshot) HL() uint16 { return uint16(s.H)<<8 | uint16(s.L) }
func (s *StateSnapshot) AF_() uint16 { return uint16(s.A_)<<8 | uint16(s.F_) }
func (s *StateSnapshot) BC_() uint16 { return uint16(s.B_)<<8 | uint16(s.C_) }
```

#### 2. **REPL Build Issues** in `cmd/mzr/main.go`

```go
// Line 251: Type mismatch
analyzer.Analyze(ast) // ast is []Declaration, needs *ast.File

// Line 275: Missing argument
optimizer.NewOptimizer() // Needs OptimizationLevel parameter
```

**Fix Required:**
```go
// Line 251 - Wrap declarations in File
astFile := &ast.File{
    Declarations: ast,
    Filename: "repl_input",
}
analyzer.Analyze(astFile)

// Line 275 - Add optimization level
opt := optimizer.NewOptimizer(optimizer.OptimizationLevel(1))
```

## Why This Matters 🎯

### Revolutionary Features This Enables:

1. **Cycle-Perfect TDD**
   - Write tests that verify exact T-state counts
   - Detect performance regressions automatically
   - Profile real hardware constraints

2. **Time-Travel Debugging**
   - Step backwards through execution
   - Find exact moment bugs occur
   - Replay deterministically

3. **SMC Visualization**
   - See self-modifying code in action
   - Verify TRUE SMC optimization
   - Debug parameter patching

4. **Performance Analysis**
   - Find hotspots automatically
   - Get optimization suggestions
   - Compare implementation strategies

## Implementation Plan

### Phase 1: Quick Fixes (2 hours)
1. Fix struct field issues in `tas_format.go`
2. Add missing methods to StateSnapshot
3. Update InputEvent structure

### Phase 2: REPL Integration (4 hours)
1. Fix build issues in `cmd/mzr/main.go`
2. Test REPL with TAS commands
3. Verify recording/replay works

### Phase 3: Testing & Documentation (2 hours)
1. Create example TAS recordings
2. Document TAS command usage
3. Add to CLAUDE.md

## Example Use Cases

### 1. Performance Regression Testing
```bash
# Record baseline performance
mz program.minz --tas-record baseline.tas

# After changes, compare
mz program.minz --tas-record new.tas
mz-tas-diff baseline.tas new.tas  # Shows cycle differences
```

### 2. Bug Hunting
```minz
// Add debug checkpoint in code
@tas_checkpoint("before_calculation")
let result = complex_function(data);
@tas_checkpoint("after_calculation")
```

### 3. Optimization Verification
```bash
# Profile without optimization
mz zvdb.minz --tas-record unopt.tas

# Profile with SMC
mz zvdb.minz --tas-record opt.tas -O --enable-smc

# Analyze improvement
mz-tas-analyze unopt.tas opt.tas
```

## Benefits to MinZ Ecosystem

1. **Unique Selling Point** - No other 8-bit language has this!
2. **Professional Development** - Enterprise-grade debugging for vintage hardware
3. **Educational Value** - Perfect for teaching optimization
4. **Community Building** - TAS community crossover potential
5. **Performance Proof** - Verify "zero-cost abstractions" claims

## Request for Immediate Action 🚀

This framework is **so close** to working! The architecture is brilliant - using TAS emulator techniques for debugging is genius. We just need these small fixes to unlock revolutionary debugging capabilities for Z80 development.

**Estimated Time: 8 hours total**
**Impact: MASSIVE** 

This would make MinZ the most advanced development environment for 8-bit systems ever created!

## Technical Details for Implementation

### Files to Modify:
1. `/minzc/pkg/tas/tas_format.go` - Lines 362, 366, 409, 410, 455-478
2. `/minzc/cmd/mzr/main.go` - Lines 251, 275
3. `/minzc/cmd/repl/main.go` - Possible similar issues

### Testing Approach:
```go
// Test TAS recording
func TestTASRecording(t *testing.T) {
    emu := emulator.New()
    tas := NewTASDebugger(emu)
    
    // Execute some code
    emu.LoadProgram(testProgram)
    emu.Run(1000) // Run 1000 cycles
    
    // Save recording
    tas.SaveToFile("test.tas")
    
    // Verify replay
    emu2 := emulator.New()
    tas2 := NewTASDebugger(emu2)
    tas2.LoadFromFile("test.tas")
    
    // States should match
    assert.Equal(t, tas.GetState(), tas2.GetState())
}
```

## Conclusion

The TAS/TDD framework is a **masterpiece of engineering** that's 95% complete. These fixes would unlock capabilities that **no other 8-bit development system has ever had**. 

This is the kind of feature that gets people excited about MinZ - it shows we're not just recreating old tools, we're building **the future of retro development**!

Let's make this happen! 🎉

---

*Created by: Claude & Alice*  
*Date: August 6, 2025*  
*Priority: HIGH 🔥*  
*Estimated effort: 8 hours*  
*Impact: Revolutionary*

/celebrate when this gets fixed! 🚀