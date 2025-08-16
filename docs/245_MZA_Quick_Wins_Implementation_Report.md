# MZA Quick Wins Implementation Report

## Overview: Parallel Improvements While Building Phase 2

**Current Status**: 12% success rate achieved in Phase 1  
**Phase 2 Target**: 40-60% with table-driven encoder  
**Quick Wins Target**: Additional 10-15% improvement through practical features

These quick wins can be implemented in parallel with Phase 2 and provide immediate user value.

---

## 🎯 Quick Win #1: Device/Target Support (High Impact)

### Problem
MinZ generates code for different Z80 platforms (ZX Spectrum, Amstrad CPC, etc.) but MZA doesn't handle platform-specific requirements.

### Implementation
```go
type Z80Target struct {
    Name        string
    MemoryMap   map[string]uint16  // ROM/RAM regions
    EntryPoint  uint16             // Default ORG address
    Constraints []Constraint       // Platform limitations
}

var Z80Targets = map[string]Z80Target{
    "spectrum": {
        Name: "ZX Spectrum 48K",
        MemoryMap: map[string]uint16{
            "ROM_START": 0x0000,
            "ROM_END":   0x3FFF, 
            "RAM_START": 0x4000,
            "RAM_END":   0xFFFF,
            "SCREEN":    0x4000,
            "ATTRS":     0x5800,
        },
        EntryPoint: 0x8000,
        Constraints: []Constraint{
            NoCodeInROM,
            ScreenMemoryConflict,
        },
    },
    "cpm": {
        Name: "CP/M Z80",
        MemoryMap: map[string]uint16{
            "TPA_START": 0x0100,
            "BDOS":      0xF200,
            "BIOS":      0xF600,
        },
        EntryPoint: 0x0100,
    },
}
```

### Usage
```bash
mza program.a80 --target=spectrum -o program.tap
mza program.a80 --target=cpm -o program.com
mza program.a80 --target=generic -o program.bin
```

### Expected Impact
- **+5% success rate** - Platform-specific memory layout validation
- **Better error messages** - "Code at $0000 conflicts with Spectrum ROM"
- **User experience** - Automatic platform configuration

---

## 🎯 Quick Win #2: .SNA / .TAP Export Support (High Value)

### Problem
MZA currently only outputs raw binary. Users need .SNA (snapshot) and .TAP (tape) files for emulators and real hardware.

### Implementation

#### .SNA (Snapshot) Support
```go
type SNAFile struct {
    Header   SNAHeader
    RAM      [49152]byte  // 48K RAM dump
    Registers Z80State
}

type SNAHeader struct {
    I, HLalt, DEalt, BCalt, AFalt uint16
    HL, DE, BC, IY, IX uint16
    IFF2, R, AF, SP uint16
    IntMode, BorderColor byte
}

func (a *Assembler) ExportSNA(filename string, entryPoint uint16) error {
    sna := SNAFile{
        Header: SNAHeader{
            SP: 0xFFFE,  // Stack pointer
            AF: 0x0000,  // Accumulator and flags
            // ... other registers
        },
    }
    
    // Copy assembled code to RAM image
    copy(sna.RAM[entryPoint-0x4000:], a.GeneratedCode)
    
    return writeSNAFile(filename, sna)
}
```

#### .TAP (Tape) Support  
```go
type TAPBlock struct {
    Flag   byte    // 0x00=header, 0xFF=data
    Type   byte    // 0x03=code block
    Name   [10]byte // Program name
    Length uint16  // Data length
    Start  uint16  // Start address
    Param2 uint16  // For code: start address again
    Data   []byte  // Actual code
    Checksum byte  // XOR of all data
}

func (a *Assembler) ExportTAP(filename string, progName string, startAddr uint16) error {
    // Create header block
    header := TAPBlock{
        Flag: 0x00,
        Type: 0x03,
        Name: padName(progName, 10),
        Length: uint16(len(a.GeneratedCode)),
        Start: startAddr,
        Param2: startAddr,
    }
    
    // Create data block
    data := TAPBlock{
        Flag: 0xFF,
        Data: a.GeneratedCode,
    }
    
    return writeTAPFile(filename, []TAPBlock{header, data})
}
```

### Usage
```bash
mza game.a80 --target=spectrum --format=sna -o game.sna
mza demo.a80 --target=spectrum --format=tap -o demo.tap
mza util.a80 --target=cpm --format=com -o util.com
```

### Expected Impact
- **+3% success rate** - Better output format handling
- **Massive user value** - Direct emulator/hardware compatibility
- **Workflow improvement** - No external tools needed

---

## 🎯 Quick Win #3: Directive Support (Medium Impact)

### Problem
MinZ generates assembly with directives that MZA doesn't support: `INCLUDE`, `DEFB`, `DEFW`, `DEFS`, etc.

### Current Gaps
```asm
; These fail in MZA but are common in MinZ output:
    DEFB $01, $02, $03     ; Define bytes
    DEFW $1234, $5678      ; Define words  
    DEFS 100, $00          ; Define space
    INCLUDE "constants.asm" ; Include file
```

### Implementation
```go
// Add to directive handling
var Directives = map[string]DirectiveHandler{
    "DEFB": handleDEFB,
    "DB":   handleDEFB,  // Alias
    "DEFW": handleDEFW, 
    "DW":   handleDEFW,  // Alias
    "DEFS": handleDEFS,
    "DS":   handleDEFS,  // Alias
    "INCLUDE": handleINCLUDE,
}

func handleDEFB(a *Assembler, operands []string) error {
    for _, op := range operands {
        val, err := a.resolveValue(op)
        if err != nil { return err }
        a.EmitByte(byte(val))
    }
    return nil
}

func handleDEFW(a *Assembler, operands []string) error {
    for _, op := range operands {
        val, err := a.resolveValue(op)
        if err != nil { return err }
        a.EmitWord(val)
    }
    return nil
}
```

### Expected Impact
- **+4% success rate** - Data directive support
- **Broader compatibility** - Handle more MinZ output patterns  
- **Standard compliance** - Match other Z80 assemblers

---

## 🎯 Quick Win #4: Enhanced Error Messages (Low Effort, High UX)

### Problem
Current errors are cryptic: "undefined symbol: (HL)" doesn't help users.

### Implementation
```go
type AssemblyError struct {
    Line    int
    Column  int  
    Type    ErrorType
    Message string
    Suggestion string
}

func NewUndefinedSymbolError(line int, symbol string) AssemblyError {
    var suggestion string
    
    // Smart suggestions based on common patterns
    if strings.HasPrefix(symbol, "(") && strings.HasSuffix(symbol, ")") {
        inner := symbol[1:len(symbol)-1]
        if isValidRegister(inner) {
            suggestion = fmt.Sprintf("Register indirect '%s' may not be supported. Try immediate addressing or check instruction syntax.", symbol)
        } else {
            suggestion = fmt.Sprintf("Memory indirect '%s' requires address format like '($8000)'", symbol)
        }
    }
    
    return AssemblyError{
        Line: line,
        Type: UndefinedSymbol,
        Message: fmt.Sprintf("undefined symbol: %s", symbol),
        Suggestion: suggestion,
    }
}
```

### Expected Impact
- **+2% effective success rate** - Users can fix more issues
- **Better developer experience** - Clear guidance on fixes
- **Reduced support burden** - Self-explanatory errors

---

## 🎯 Quick Win #5: Pseudo-Instruction Expansion (Medium Impact)

### Problem
MinZ generates pseudo-instructions that need expansion to real Z80 opcodes.

### Common Patterns
```asm
; MinZ generates these, but Z80 doesn't have them:
LD HL, DE          ; -> LD H, D : LD L, E  
LD BC, HL          ; -> LD B, H : LD C, L
NEG A              ; -> NEG (A is implied)
PUSH AF, BC        ; -> PUSH AF : PUSH BC
```

### Implementation  
```go
var PseudoInstructions = map[string]PseudoExpander{
    "LD": expandPseudoLD,
    "PUSH": expandPseudoPUSH,  
    "POP":  expandPseudoPOP,
}

func expandPseudoLD(operands []string) []Instruction {
    dest, src := operands[0], operands[1]
    
    // 16-bit register to 16-bit register expansion
    if isReg16(dest) && isReg16(src) {
        return []Instruction{
            {Mnemonic: "LD", Operands: []string{highByte(dest), highByte(src)}},
            {Mnemonic: "LD", Operands: []string{lowByte(dest), lowByte(src)}},
        }
    }
    
    return []Instruction{{Mnemonic: "LD", Operands: operands}}
}
```

### Expected Impact
- **+3% success rate** - Handle more MinZ instruction patterns
- **Compatibility boost** - Bridge gap between MinZ and Z80
- **Foundation for advanced** - Prepares for complex instruction support

---

## Implementation Priority & Timeline

### Week 1: High Impact, Low Effort
1. **Enhanced Error Messages** (1-2 days) - Immediate UX improvement
2. **Basic Directive Support** (2-3 days) - DEFB, DEFW, DEFS

### Week 2: High Value Features  
3. **Target Support Framework** (3-4 days) - Platform definitions
4. **Pseudo-Instruction Expansion** (2-3 days) - Common patterns

### Week 3: Output Formats
5. **.TAP Export** (2-3 days) - Spectrum tape files
6. **.SNA Export** (2-3 days) - Spectrum snapshots  

### Week 4: Polish & Integration
7. **Target-specific optimizations** (2-3 days)
8. **Comprehensive testing** (2-3 days)

## Expected Cumulative Impact

### Success Rate Progression
- **Baseline**: 12% (Phase 1 complete)
- **+ Error Messages**: 14% (users fix more issues)
- **+ Directives**: 18% (data directive support)
- **+ Pseudo-instructions**: 21% (instruction expansion)
- **+ Target support**: 26% (platform validation)
- **+ Output formats**: 27% (better format handling)

### **Combined with Phase 2**: 27% + 40-60% = **67-87% target success rate!**

## Implementation Strategy

### Parallel Development
- **Quick wins** can be implemented while Phase 2 table-driven encoder is being built
- **Independent modules** - each quick win is self-contained
- **Incremental testing** - measure impact of each addition
- **Low risk** - fallback to current behavior if issues arise

### Integration Points
- **Share target information** between quick wins and Phase 2 encoder
- **Common error handling** framework across all improvements  
- **Unified output pipeline** for different formats
- **Coordinated testing** to ensure no regressions

This approach gives us **immediate user value** while building toward the **systematic Phase 2 solution**! 🚀