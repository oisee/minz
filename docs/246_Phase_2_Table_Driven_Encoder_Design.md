# Phase 2: Table-Driven Encoder Design

## Current Status: 12% → Target: 40-60%

**Phase 1 Results**: ✅ **12% success rate achieved** (6x improvement from 2% baseline)

**Phase 2 Goal**: Implement systematic table-driven encoder as recommended by o4-mini AI colleague

## Design Philosophy: From Ad-Hoc to Systematic

### Current Problem (Ad-Hoc Approach)
```go
// Current: Manual routing with gaps
if isRegisterIndirect(dest) || isRegisterIndirect(src) {
    return encodeLDIndirect(a, dest, src)  // Only handles some cases
}
if isMemoryIndirect(dest) || isMemoryIndirect(src) {
    return encodeLDMemory(a, dest, src)    // Missing many patterns
}
```

### New Approach (Table-Driven)
```go
// Proposed: Pattern-based instruction definitions
type InstructionPattern struct {
    Pattern     string           // "LD r8, (nn)"
    Template    []byte          // [0x3A, IMM16_LO, IMM16_HI]
    Operands    []OperandDef    // Structured operand parsing
    Validator   func(...) bool  // Pattern validation
}
```

## Implementation Plan

### 1. Operand AST System
Replace raw string operands with structured types:

```go
type Operand struct {
    Type     OperandType    // REG8, REG16, IMM8, ADDR16, etc.
    Value    interface{}    // Typed value (Register, uint16, etc.)
    Raw      string        // Original text for debugging
}

type OperandDef struct {
    Name        string           // "r8", "nn", "d"
    Type        OperandType      // What this operand represents  
    Constraints []Constraint    // Additional validation rules
}
```

### 2. Instruction Pattern Database
Comprehensive Z80 instruction definitions:

```go
var InstructionPatterns = []InstructionPattern{
    // Basic 8-bit loads
    {
        Pattern: "LD r8, r8",
        Template: []byte{0x40}, // Base + reg encoding
        Operands: []OperandDef{
            {Name: "dest", Type: REG8},
            {Name: "src", Type: REG8},
        },
        Encoder: encodeRegToReg,
    },
    
    // Memory loads - the ones we need to fix!
    {
        Pattern: "LD r8, (nn)",
        Template: []byte{0x3A, IMM16_LO, IMM16_HI}, 
        Operands: []OperandDef{
            {Name: "reg", Type: REG8, Constraints: []Constraint{MustBeA}},
            {Name: "addr", Type: ADDR16},
        },
        Encoder: encodeMemoryLoad8,
    },
    
    {
        Pattern: "LD r16, (nn)",
        Template: []byte{0x2A, IMM16_LO, IMM16_HI},
        Operands: []OperandDef{
            {Name: "reg", Type: REG16, Constraints: []Constraint{MustBeHL}},
            {Name: "addr", Type: ADDR16},
        },
        Encoder: encodeMemoryLoad16,
    },
    
    // More patterns...
}
```

### 3. Pattern Matching Engine
```go
func MatchInstruction(mnemonic string, operands []Operand) *InstructionPattern {
    for _, pattern := range InstructionPatterns[mnemonic] {
        if pattern.Matches(operands) {
            return &pattern
        }
    }
    return nil
}

func (p *InstructionPattern) Matches(operands []Operand) bool {
    if len(operands) != len(p.Operands) {
        return false
    }
    
    for i, opDef := range p.Operands {
        if !opDef.Matches(operands[i]) {
            return false
        }
    }
    return true
}
```

### 4. Template-Based Code Generation
```go
func (p *InstructionPattern) Encode(operands []Operand) ([]byte, error) {
    code := make([]byte, len(p.Template))
    copy(code, p.Template)
    
    // Fill in template placeholders
    for i, b := range code {
        switch b {
        case IMM8_VAL:
            code[i] = operands[findOperand("imm8")].Value.(uint8)
        case IMM16_LO:
            val := operands[findOperand("addr")].Value.(uint16)
            code[i] = uint8(val & 0xFF)
        case IMM16_HI:
            val := operands[findOperand("addr")].Value.(uint16)
            code[i] = uint8(val >> 8)
        case REG_ENCODING:
            reg := operands[findOperand("reg")].Value.(Register)
            code[i] |= encodeRegister(reg)
        }
    }
    
    return code, nil
}
```

## Expected Benefits

### Instruction Coverage
- **Systematic coverage** of all Z80 instruction patterns
- **No gaps** - every valid combination explicitly defined
- **Easy additions** - new instructions just add patterns
- **Validation** - automatic pattern conflict detection

### Error Handling
- **Clear error messages** - "No pattern matches LD A, (unknown)"
- **Suggestion system** - "Did you mean LD A, (HL)?"
- **Pattern debugging** - Show which patterns were considered

### Maintainability
- **Self-documenting** - patterns show exactly what's supported
- **Testable** - each pattern can be unit tested
- **Extensible** - undocumented instructions just add patterns
- **Debuggable** - clear separation of matching vs encoding

## Implementation Timeline

### Week 1: Foundation
- [ ] Define Operand AST types
- [ ] Implement pattern matching engine
- [ ] Create basic pattern database (20-30 core patterns)
- [ ] Test with simple instructions (LD r8, r8)

### Week 2: Memory Instructions
- [ ] Add all memory LD patterns that are currently broken
- [ ] Implement template-based code generation
- [ ] Test memory indirect addressing (our Phase 1 target)
- [ ] Validate against supertest_z80.a80

### Week 3: Complete Coverage
- [ ] Add all remaining Z80 instruction patterns
- [ ] Implement undocumented instruction support
- [ ] Optimize pattern matching performance
- [ ] Full corpus testing

### Week 4: Polish & Optimization
- [ ] Error message improvements
- [ ] Performance profiling and optimization
- [ ] Documentation and examples
- [ ] Regression test suite

## Success Metrics

### Technical Targets
- **40-60% corpus success rate** (vs current 12%)
- **Zero pattern gaps** for standard Z80 instructions
- **<1ms encoding time** per instruction
- **100% pattern test coverage**

### Quality Targets
- **Clear error messages** for all failure cases
- **Complete documentation** of supported patterns
- **Zero regressions** in currently working functionality
- **Maintainable codebase** for future extensions

## Risk Mitigation

### Performance Concerns
- **Solution**: Pattern pre-indexing by mnemonic and operand count
- **Fallback**: Keep current encoder for hot paths during transition

### Compatibility Risks  
- **Solution**: Parallel implementation with feature flag
- **Testing**: Extensive regression testing during transition
- **Rollback**: Keep old encoder until new one is proven

### Complexity Management
- **Solution**: Start with minimal pattern set, expand incrementally
- **Documentation**: Each pattern thoroughly documented with examples
- **Testing**: Pattern-specific unit tests for validation

## Implementation Files

### New Files to Create
```
minzc/pkg/z80asm/
├── patterns/
│   ├── operand.go           # Operand AST types
│   ├── pattern.go           # Pattern matching engine  
│   ├── database.go          # Instruction pattern database
│   └── encoder.go           # Template-based encoding
├── table_encoder.go         # Main table-driven encoder
└── pattern_test.go         # Pattern validation tests
```

### Modified Files
```
minzc/pkg/z80asm/
├── assembler.go            # Integration with new encoder
├── instructions.go         # Gradual migration to patterns
└── encoder.go             # Legacy encoder (deprecated)
```

This table-driven approach should systematically solve the instruction encoding gaps and get us to the 40-60% success rate target for Phase 2! 🎯