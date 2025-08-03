# 6502 Debugging Ecosystem Research for MinZ Integration

## Executive Summary

The 6502 debugging ecosystem presents a **mature, sophisticated toolchain** with excellent debugging capabilities across multiple platforms. Unlike the general 6502 Go toolchain (which was found to be immature for compiler backends), the debugging infrastructure is **production-ready and feature-rich**. However, integration complexity remains **high** due to platform fragmentation and the need for specialized debugging protocols.

**Key Finding**: 6502 debugging capabilities **exceed current Z80 support** in several areas, particularly source-level debugging, cycle profiling, and SMC tracing. The ecosystem provides world-class debugging tools that could significantly enhance MinZ's development experience.

**Recommendation**: 6502 debugging infrastructure is **excellent** and ready for integration, but should be implemented as part of a broader multi-target debugging framework rather than a 6502-specific solution.

---

## Tool Landscape Analysis

### Tier 1: Production-Quality Debuggers

#### **VICE C64 Emulator** ⭐⭐⭐⭐⭐
- **Platform**: Commodore 64, VIC-20, PET, Plus/4, C128
- **Debugging Features**:
  - Complete built-in monitor with breakpoints, watchpoints, trace logging
  - Source-level debugging with symbol file support (.DBG, .SYM formats)
  - Remote monitor API (port 6510) for external debugger integration
  - Cycle counting and performance profiler ('chis' command)
  - Memory examination, modification, and disassembly
- **SMC Support**: Full trace logging of self-modifying code events
- **Integration**: Excellent - Remote protocol allows Go-based debugger frontends
- **Maturity**: **PRODUCTION** - Decades of development, actively maintained

#### **Mesen NES Emulator** ⭐⭐⭐⭐⭐
- **Platform**: Nintendo Entertainment System (NES)
- **Debugging Features**:
  - Advanced assembly debugger with source-level integration
  - Performance profiler with function timing and bottleneck analysis
  - Comprehensive trace logger with conditional logging
  - Symbol import from CC65/CA65 (.DBG), NESASM (.FNS), ASM6f (.MLB)
  - Built-in assembler for runtime code modification
  - Call stack visualization and watch expressions
- **SMC Support**: Full execution trace with code modification tracking
- **Integration**: Good - Command-line integration, symbol file formats
- **Maturity**: **PRODUCTION** - Modern, actively developed, extensive features

#### **AppleWin Apple II Emulator** ⭐⭐⭐⭐
- **Platform**: Apple II family
- **Debugging Features**:
  - Built-in symbolic debugger with extensive command set
  - Profiler with cycle counting and performance analysis
  - Symbol table support (ACME, Merlin formats)
  - Multiple display modes (code, data, console)
  - Sophisticated breakpoint system with conditional logic
- **SMC Support**: Memory write tracking and code modification detection
- **Integration**: Moderate - Symbol file integration, but limited external API
- **Maturity**: **PRODUCTION** - Long-established, stable, well-documented

### Tier 2: Advanced External Debuggers

#### **RetroDebugger** ⭐⭐⭐⭐
- **Platform**: Multi-platform (C64 via VICE, Atari XL/XE, NES via NestopiaUE)
- **Debugging Features**:
  - Real-time debugging API host architecture
  - Source-level debugging with side-by-side assembly/source view
  - Champ 6502 profiler integration
  - Cross-platform debugging consistency
- **SMC Support**: Platform-dependent (inherits from underlying emulator)
- **Integration**: **EXCELLENT** - API-based architecture designed for external tools
- **Maturity**: **PRODUCTION** - Modern, cross-platform, actively maintained

#### **IceBro 6502 Debugger** ⭐⭐⭐
- **Platform**: VICE C64/VIC-20 integration
- **Debugging Features**:
  - 6502 simulator with graphical debugger interface
  - VICE remote monitor integration
  - Machine state synchronization (RAM, CPU, labels, breakpoints)
  - Source-level debugging with C64 Debugger format support
- **SMC Support**: Full state copying includes modified code regions
- **Integration**: Good - VICE remote monitor protocol
- **Maturity**: **STABLE** - Mature but specialized for VICE integration

### Tier 3: Development/Educational Tools

#### **VSCode CC65 Debugger Extension** ⭐⭐⭐
- **Platform**: Cross-platform IDE integration
- **Features**: VICE/Mesen/AppleWin integration, breakpoints, source debugging
- **SMC Support**: Depends on underlying emulator
- **Integration**: **IDE-focused** - Good for development workflows
- **Maturity**: **ACTIVE** - Modern development tool

---

## Go Integration Analysis

### Current Go 6502 Emulator Debugging Support

#### **beevik/go6502** ⭐⭐⭐⭐
- **Debugging API**: Interactive console with comprehensive command parsing
- **Features**: Register inspection, memory modification, source maps
- **SMC Support**: Memory write tracking capabilities
- **Integration Potential**: **HIGH** - Clean Go packages, modular design
- **Current State**: Production-ready, actively maintained (v0.3.0, May 2024)

#### **pda/go6502** ⭐⭐⭐
- **Debugging API**: Stepping debugger with sophisticated breakpoint system
- **Features**: Breakpoints on instruction type, register values, memory locations
- **SMC Support**: Memory write notifications
- **Integration Potential**: **MEDIUM** - Good API design but stale (2013-2014)

#### **cjr29/go6502** ⭐⭐
- **Debugging API**: AttachDebugger functionality with instruction/memory notifications
- **SMC Support**: Memory store notifications via debugger attachment
- **Integration Potential**: **MEDIUM** - Basic but functional debugging hooks

### Integration Architecture Options

#### Option 1: Go-Native Debugging Stack
```go
// MinZ 6502 debugging could leverage existing Go emulators
type MinZ6502Debugger struct {
    emulator    *go6502.Emulator
    sourceMap   *SourceMapDB
    breakpoints *BreakpointManager
    tracer      *ExecutionTracer
}

func (d *MinZ6502Debugger) AttachSMCTracker() {
    d.emulator.AttachDebugger(func(addr uint16, oldVal, newVal byte) {
        if d.isCodeSegment(addr) {
            d.tracer.RecordSMCEvent(addr, oldVal, newVal)
        }
    })
}
```

#### Option 2: External Emulator Integration
```go
// Integration with production emulators via remote protocols
type ExternalDebugger struct {
    protocol    DebugProtocol // VICE remote monitor, Mesen API, etc.
    symbolTable *SymbolDB
    sourceFiles *SourceDB
}
```

---

## Feature Comparison: 6502 vs Z80 Debugging

| Feature | Z80 (Current MinZ) | 6502 Ecosystem | Winner |
|---------|-------------------|----------------|--------|
| **Source-Level Debugging** | Limited | Excellent (VICE, Mesen, Symbols) | **6502** |
| **Cycle Profiling** | Basic | Advanced (per-function, bottlenecks) | **6502** |
| **SMC Tracing** | Good (custom implementation) | Excellent (built into emulators) | **6502** |
| **Breakpoint Sophistication** | Basic | Advanced (conditional, multi-type) | **6502** |
| **Memory Visualization** | Basic | Advanced (multiple views, annotations) | **6502** |
| **External Tool Integration** | Limited | Excellent (remote protocols, APIs) | **6502** |
| **Cross-Platform Support** | Good | Excellent (Windows, macOS, Linux) | **6502** |
| **Real-Time Debugging** | Limited | Excellent (runtime modification) | **6502** |
| **Performance Analysis** | Basic | Advanced (profilers, hotspot analysis) | **6502** |
| **Documentation Quality** | Good | Excellent (decades of refinement) | **6502** |

**Overall Assessment**: 6502 debugging ecosystem is **significantly more advanced** than current Z80 capabilities.

---

## Platform-Specific Analysis

### **Commodore 64 (VICE) Ecosystem** ⭐⭐⭐⭐⭐

**Strengths**:
- Most mature 6502 debugging environment
- Remote monitor protocol perfect for external integration
- Extensive symbol format support
- Active community and continuous development
- Production-quality SMC debugging

**Integration Path**:
```bash
# VICE remote monitor integration
vice64 -remotemonitor &
# MinZ debugger connects to localhost:6510
minz-debug --target=6502-c64 --connect=vice program.minz
```

**SMC Capabilities**:
- Complete execution trace logging
- Memory write notifications
- Breakpoints on code modification
- Source-level SMC visualization

### **NES (Mesen) Ecosystem** ⭐⭐⭐⭐⭐

**Strengths**:
- Modern debugger architecture
- Excellent performance profiling
- Advanced trace logging with conditional capabilities
- Built-in assembler for runtime testing
- Comprehensive symbol format support

**Integration Path**:
```bash
# Mesen debugging integration via symbol files
minzc program.minz --target=6502-nes --debug-symbols=mesen.dbg
mesen program.nes --dbgfile=mesen.dbg
```

**SMC Capabilities**:
- Instruction-level execution tracing
- Code modification detection and logging
- Performance impact analysis of SMC

### **Apple II (AppleWin) Ecosystem** ⭐⭐⭐⭐

**Strengths**:
- Sophisticated symbolic debugger
- Excellent profiler with cycle counting
- Multiple symbol format support
- Robust breakpoint system

**Integration Challenges**:
- Limited external API compared to VICE/Mesen
- Windows-focused (though cross-platform ports exist)
- Less modern architecture than Mesen

---

## Source-Level Debugging Feasibility

### **Technical Requirements for MinZ → 6502 Source Debugging**

#### 1. **Symbol File Generation**
```go
// MinZ compiler would generate debug symbols
type DebugSymbol struct {
    Name        string
    Address     uint16
    Type        SymbolType // Function, Variable, Label
    SourceFile  string
    SourceLine  int
    Size        int
}

// Export formats for different debuggers
func (c *Compiler) GenerateVICESymbols() []VICECommand
func (c *Compiler) GenerateMesenDBG() *MesenDBGFile
func (c *Compiler) GenerateAppleSymbols() *AppleSymbolTable
```

#### 2. **Source Map Generation**
```go
type SourceMap struct {
    MinZFile     string
    AssemblyFile string
    Mappings     []SourceMapping
}

type SourceMapping struct {
    MinZLine     int
    AssemblyAddr uint16
    Instructions []Instruction
}
```

#### 3. **SMC Source Tracking**
```go
// Track which MinZ constructs generate SMC
type SMCMapping struct {
    MinZConstruct string // "lambda", "interface_call", "optimized_loop"
    CodeRange     AddressRange
    SMCEvents     []SMCEvent
    OriginalCode  string
}
```

### **Feasibility Assessment**: **HIGHLY FEASIBLE** ✅

**Reasons**:
1. **Symbol Format Support**: All major 6502 debuggers support symbol import
2. **Source Mapping**: Existing tools already handle assembly-to-source mapping
3. **SMC Integration**: Advanced emulators already track code modifications
4. **Community Standards**: Well-established debugging file formats

### **Implementation Path**:
```go
// MinZ could generate comprehensive debugging information
minzc program.minz --target=6502-c64 \
    --debug-symbols=vice \
    --source-maps \
    --smc-tracking \
    --output=program.a65
```

---

## Self-Modifying Code (SMC) Debugging Analysis

### **6502 SMC Debugging Capabilities**

#### **Current Tool Support**:

1. **ca65 SMC Macro Package** ⭐⭐⭐⭐
   - Provides SMC-specific assembly macros for safety and documentation
   - `smc.inc` package with SMC_StoreOpcode, SMC_ChangeBranch, etc.
   - Makes SMC more maintainable and self-documenting
   - **Integration**: MinZ could generate ca65-compatible SMC code

2. **VICE SMC Tracing** ⭐⭐⭐⭐⭐
   - Complete execution trace with memory write detection
   - Breakpoints on code modification events
   - Watch/trace commands share numbered checkpoints
   - Step-through debugging of modified code
   - **Integration**: Remote monitor API exposes all SMC events

3. **Mesen SMC Analysis** ⭐⭐⭐⭐⭐
   - Sophisticated trace logger with conditional SMC logging
   - Performance profiler shows SMC impact on execution time
   - Runtime assembler for testing SMC patterns
   - **Integration**: Symbol files can annotate SMC regions

### **SMC Debugging Workflow**:
```
MinZ Source → Compiler Analysis → SMC Annotation → Symbol Generation → Debugger Integration
     ↓              ↓                   ↓                 ↓                    ↓
Lambda/Interface → SMC Pattern → Code Markers → Debug Symbols → Runtime Tracing
```

### **Example SMC Debugging Session**:
```bash
# MinZ generates SMC-aware debugging info
minzc lambda_demo.minz --target=6502-c64 --smc-debug

# VICE session with SMC tracking
vice64 -remotemonitor lambda_demo.d64
(monitor) break smc_lambda_call
(monitor) trace on
(monitor) run

# SMC event detected:
# Address $3000: LDA #$42 → LDA #$37 (lambda parameter specialization)
# Source: lambda_demo.minz:15 |x: u8| => u8 { x + 5 }
```

### **MinZ SMC Debugging Features** (Proposed):
```go
type SMCDebugger struct {
    events    []SMCEvent
    patterns  map[string]SMCPattern
    source    *SourceMapper
}

type SMCEvent struct {
    Address     uint16
    OldCode     []byte
    NewCode     []byte
    Trigger     string // "lambda_curry", "interface_dispatch", "loop_opt"
    SourceRef   SourceLocation
    Timestamp   uint64
}
```

---

## Performance Profiling Capabilities

### **6502 Profiler Ecosystem**

#### **VICE Profiling** ⭐⭐⭐⭐
- **Commands**: `profile`, `profile reset`, `profile list`
- **Capabilities**: Cycle counting between breakpoints, instruction timing
- **Integration**: Remote monitor allows external profiler tools
- **Use Case**: Optimize performance-critical MinZ code sections

#### **Mesen Performance Profiler** ⭐⭐⭐⭐⭐
- **Features**: Function-level timing, bottleneck identification, cycle counting
- **Visualization**: Graphical profiler with hotspot highlighting
- **Integration**: Symbol file integration for function names
- **Advanced**: Call stack profiling, performance regression detection

#### **AppleWin Profiler** ⭐⭐⭐
- **Features**: Cycle counting, instruction-level timing
- **Integration**: Built into symbolic debugger
- **Use Case**: Traditional assembly optimization workflows

### **MinZ Performance Integration**:
```go
// Proposed MinZ profiler integration
type PerformanceProfiler struct {
    emulator    Emulator6502
    functions   map[string]FunctionProfile
    hotspots    []Hotspot
}

type FunctionProfile struct {
    Name         string
    CallCount    uint64
    TotalCycles  uint64
    SMCOverhead  uint64
    Optimizations []AppliedOptimization
}
```

---

## Development Environment Integration

### **IDE Support Analysis**

#### **VSCode Integration** ⭐⭐⭐⭐
- **Current**: CC65 debugger extension for VICE/Mesen/AppleWin
- **Features**: Breakpoints, variable inspection, source debugging
- **MinZ Integration Path**: 
  ```json
  // VSCode MinZ debugging configuration
  {
    "type": "minz-6502",
    "request": "launch",
    "program": "${workspaceFolder}/program.minz",
    "target": "6502-c64",
    "emulator": "vice",
    "debugSymbols": true,
    "smcTracing": true
  }
  ```

#### **Standalone Debuggers** ⭐⭐⭐⭐⭐
- **RetroDebugger**: Cross-platform GUI with source-level debugging
- **IceBro**: VICE integration with modern interface
- **C64 Debugger**: Advanced features with source correlation

### **Visualization Tools**

#### **Memory Viewers** ⭐⭐⭐⭐
- All major emulators provide sophisticated memory visualization
- Support for different data interpretations (hex, assembly, sprites, etc.)
- Real-time updates during execution

#### **Execution Flow** ⭐⭐⭐⭐
- Call stack visualization (Mesen)
- Execution trace with branching (VICE, AppleWin)
- Performance hotspot highlighting (Mesen profiler)

#### **SMC Visualization** ⭐⭐⭐
- Code modification highlighting
- Before/after instruction comparison
- Source-level SMC annotation

---

## Comparison with Z80 Debugging Quality

### **Current MinZ Z80 Debugging**:
- ✅ Cycle-accurate emulation (`minzc/pkg/emulator/z80.go`)
- ✅ REPL with step-through debugging
- ✅ Memory inspection and modification
- ✅ Register state visualization
- ✅ SMC event tracking (custom implementation)
- ❌ Limited source-level debugging
- ❌ Basic profiling capabilities
- ❌ No external debugger integration
- ❌ Limited visualization tools

### **Potential 6502 Debugging**:
- ✅ All Z80 features plus:
- ✅ **Advanced source-level debugging** (multiple emulators support)
- ✅ **Professional profiling tools** (function-level, bottleneck analysis)
- ✅ **External debugger ecosystem** (VICE, Mesen, AppleWin, RetroDebugger)
- ✅ **Sophisticated visualization** (call stacks, memory views, execution flow)
- ✅ **SMC debugging excellence** (built-in tracing, conditional logging)
- ✅ **IDE integration** (VSCode extensions, modern workflows)
- ✅ **Cross-platform support** (Windows, macOS, Linux)

### **Quality Assessment**: 6502 debugging would be **significantly superior** to current Z80 support.

---

## Recommendations

### **Primary Recommendation**: **IMPLEMENT 6502 DEBUGGING SUPPORT** ✅

**Rationale**:
1. **Mature Ecosystem**: 6502 debugging tools are production-ready and feature-rich
2. **Superior Capabilities**: Exceeds current Z80 debugging in all major areas
3. **Integration Ready**: Well-established protocols and file formats
4. **Development Impact**: Would significantly enhance MinZ development experience

### **Implementation Strategy**:

#### **Phase 1: Foundation** (4-6 weeks)
```go
// Core debugging infrastructure
type MultiTargetDebugger interface {
    AttachEmulator(emulator TargetEmulator) error
    LoadSymbols(symbolFile string) error
    SetBreakpoint(location SourceLocation) error
    TraceExecution(filter TraceFilter) <-chan ExecutionEvent
}

type Target6502Debugger struct {
    emulator   Emulator6502
    symbols    *SymbolDatabase
    smc        *SMCTracker
    profiler   *PerformanceProfiler
}
```

#### **Phase 2: External Integration** (6-8 weeks)
- VICE remote monitor integration
- Mesen symbol file generation
- AppleWin debugging support
- VSCode extension development

#### **Phase 3: Advanced Features** (8-10 weeks)
- Source-level debugging UI
- Performance profiler integration
- SMC visualization tools
- Cross-platform debugger GUI

### **Architecture Decision**: **Multi-Target Debugging Framework**

Rather than 6502-specific debugging, implement a **unified debugging architecture**:

```go
// Unified debugging for all MinZ targets
type MinZDebugger struct {
    target     TargetArchitecture // Z80, 6502, 68000, etc.
    emulator   TargetEmulator
    symbols    *UniversalSymbolDB
    profiler   *CrossTargetProfiler
}
```

This approach:
- ✅ Improves Z80 debugging by adopting 6502 best practices
- ✅ Provides consistent debugging experience across targets
- ✅ Leverages existing 6502 tool maturity
- ✅ Enables future target additions (68000, RISC-V, etc.)

### **Resource Requirements**:
- **Timeline**: 18-24 weeks for full implementation
- **Developer**: 1 full-time developer with emulator experience
- **Priority**: High - debugging quality directly impacts development velocity

### **Success Metrics**:
- Source-level debugging for MinZ constructs (lambdas, interfaces, SMC)
- Performance profiling showing optimization benefits
- External debugger integration (VICE/Mesen/AppleWin)
- Cross-platform debugger GUI
- IDE integration (VSCode extension)

---

## Conclusion

The 6502 debugging ecosystem represents a **mature, sophisticated toolchain** that far exceeds MinZ's current Z80 debugging capabilities. While the general 6502 Go compiler toolchain may be immature, the debugging infrastructure is **production-ready and world-class**.

**Key Opportunities**:
1. **Leverage mature emulators** (VICE, Mesen, AppleWin) for immediate advanced debugging
2. **Implement source-level debugging** using established symbol file formats
3. **Advanced SMC debugging** with built-in tracing and visualization
4. **Professional profiling** with function-level analysis and bottleneck detection
5. **Modern IDE integration** through VSCode extensions and debugging protocols

**Strategic Impact**: Implementing 6502 debugging support would not only enable a new target platform but **significantly enhance MinZ's overall development experience** by adopting debugging best practices from the most mature 8-bit development environment.

The investment in 6502 debugging infrastructure would pay dividends across all MinZ targets and establish a foundation for future multi-target debugging excellence.

---

**Research Date**: August 3, 2025  
**Analysis Scope**: 6502 debugging tools, emulator integration, SMC debugging, profiling capabilities  
**Key Sources**: VICE documentation, Mesen debugging features, AppleWin manuals, Go 6502 emulator analysis  
**Recommendation**: **PROCEED** with 6502 debugging implementation as part of multi-target architecture  