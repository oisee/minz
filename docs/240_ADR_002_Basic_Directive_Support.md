# ADR-002: Basic Directive Support (DEFB, DEFW, DEFS)

## Status
**ACCEPTED** - Implementation in progress

## Context

### Current Problem
MinZ generates assembly with common data directives that MZA doesn't support:
```asm
; These fail in MZA but are essential for MinZ output:
    DEFB $01, $02, $03        ; Define bytes - data tables
    DEFW $1234, $5678         ; Define words - address tables  
    DEFS 100, $00             ; Define space - buffers
    DB "Hello", 0             ; String constants
    DW sprite_data, sound_ptr ; Pointer arrays
```

### Impact Analysis
Data directives appear in **~30% of MinZ-generated files**, particularly:
- **Game data** - Sprite tables, sound data, level maps
- **System constants** - Memory addresses, configuration values
- **Buffer allocation** - Screen buffers, temporary storage
- **String literals** - Text messages, file names

### Success Rate Impact
Current **12%** could improve to **~16%** (+4%) by supporting these fundamental directives.

## Decision

### Implement Core Data Directive Support
Add support for the most common data directives used by MinZ:

1. **DEFB/DB** - Define byte(s) 
2. **DEFW/DW** - Define word(s) 
3. **DEFS/DS** - Define space (fill)
4. **String literals** - Quoted text with null termination

### Directive Syntax Support
```asm
; Byte definitions
DEFB $12                    ; Single byte
DEFB $01, $02, $03         ; Multiple bytes
DB 65, 66, 67              ; Decimal values
DB 'A', 'B', 'C'          ; Character literals

; Word definitions  
DEFW $1234                 ; Single word (little-endian)
DEFW $1234, $5678          ; Multiple words
DW main, loop, end         ; Label addresses

; Space definitions
DEFS 100                   ; 100 bytes of zero
DEFS 50, $FF               ; 50 bytes of $FF
DS 1024, 0                 ; 1KB buffer

; String literals
DB "Hello World", 0        ; Null-terminated string
DEFB "Menu", 13, 10, 0     ; String with CRLF
```

## Implementation

### 1. Directive Detection and Parsing
```go
// Add to instruction processing pipeline
func (a *Assembler) processLine(line *Line) error {
    // Check if it's a directive
    if isDirective(line.Mnemonic) {
        return a.processDirective(line)
    }
    
    // Regular instruction processing...
    return a.processInstruction(line)
}

func isDirective(mnemonic string) bool {
    upper := strings.ToUpper(mnemonic)
    directives := []string{"DEFB", "DB", "DEFW", "DW", "DEFS", "DS"}
    for _, dir := range directives {
        if upper == dir {
            return true
        }
    }
    return false
}
```

### 2. Directive Processing Engine
```go
func (a *Assembler) processDirective(line *Line) error {
    directive := strings.ToUpper(line.Mnemonic)
    
    switch directive {
    case "DEFB", "DB":
        return a.processDEFB(line.Operands)
    case "DEFW", "DW": 
        return a.processDEFW(line.Operands)
    case "DEFS", "DS":
        return a.processDEFS(line.Operands)
    default:
        return fmt.Errorf("unsupported directive: %s", directive)
    }
}
```

### 3. DEFB/DB Implementation
```go
func (a *Assembler) processDEFB(operands []string) error {
    for _, operand := range operands {
        operand = strings.TrimSpace(operand)
        
        // Handle string literals
        if isStringLiteral(operand) {
            bytes, err := parseStringLiteral(operand)
            if err != nil {
                return err
            }
            for _, b := range bytes {
                a.EmitByte(b)
            }
            continue
        }
        
        // Handle numeric values
        value, err := a.resolveValue(operand)
        if err != nil {
            return err
        }
        
        if value > 255 {
            return fmt.Errorf("DEFB value %d exceeds 8-bit range", value)
        }
        
        a.EmitByte(byte(value))
    }
    return nil
}

func isStringLiteral(operand string) bool {
    return (strings.HasPrefix(operand, "\"") && strings.HasSuffix(operand, "\"")) ||
           (strings.HasPrefix(operand, "'") && strings.HasSuffix(operand, "'"))
}

func parseStringLiteral(operand string) ([]byte, error) {
    if len(operand) < 2 {
        return nil, fmt.Errorf("invalid string literal: %s", operand)
    }
    
    // Remove quotes
    content := operand[1 : len(operand)-1]
    
    // Process escape sequences
    var result []byte
    for i := 0; i < len(content); i++ {
        if content[i] == '\\' && i+1 < len(content) {
            switch content[i+1] {
            case 'n':
                result = append(result, '\n')
            case 'r':
                result = append(result, '\r')
            case 't':
                result = append(result, '\t')
            case '\\':
                result = append(result, '\\')
            case '"':
                result = append(result, '"')
            case '\'':
                result = append(result, '\'')
            default:
                result = append(result, content[i+1])
            }
            i++ // Skip next character
        } else {
            result = append(result, content[i])
        }
    }
    
    return result, nil
}
```

### 4. DEFW/DW Implementation
```go
func (a *Assembler) processDEFW(operands []string) error {
    for _, operand := range operands {
        operand = strings.TrimSpace(operand)
        
        value, err := a.resolveValue(operand)
        if err != nil {
            return err
        }
        
        // Emit little-endian word
        a.EmitByte(byte(value & 0xFF))         // Low byte
        a.EmitByte(byte((value >> 8) & 0xFF))  // High byte
    }
    return nil
}
```

### 5. DEFS/DS Implementation  
```go
func (a *Assembler) processDEFS(operands []string) error {
    if len(operands) == 0 {
        return fmt.Errorf("DEFS requires at least one operand (size)")
    }
    
    // Parse size
    sizeStr := strings.TrimSpace(operands[0])
    size, err := a.resolveValue(sizeStr)
    if err != nil {
        return err
    }
    
    // Parse fill value (default 0)
    var fillValue byte = 0
    if len(operands) > 1 {
        fillStr := strings.TrimSpace(operands[1])
        fill, err := a.resolveValue(fillStr)
        if err != nil {
            return err
        }
        if fill > 255 {
            return fmt.Errorf("DEFS fill value %d exceeds 8-bit range", fill)
        }
        fillValue = byte(fill)
    }
    
    // Emit the specified number of bytes
    for i := uint16(0); i < size; i++ {
        a.EmitByte(fillValue)
    }
    
    return nil
}
```

### 6. Assembler Integration
```go
// Add to Assembler struct
func (a *Assembler) EmitByte(b byte) {
    a.output = append(a.output, b)
    a.currentAddr++
}

func (a *Assembler) EmitWord(w uint16) {
    a.EmitByte(byte(w & 0xFF))       // Low byte
    a.EmitByte(byte((w >> 8) & 0xFF)) // High byte
}
```

## Expected Outcomes

### Before (Current)
```asm
; This fails with "unknown instruction: DEFB"
game_data:
    DEFB $01, $02, $03
    DEFW sprite_addr, sound_addr  
    DEFS 100, $00
```

### After (Enhanced)
```asm
; This assembles correctly:
game_data:
    DEFB $01, $02, $03           ; → 01 02 03
    DEFW sprite_addr, sound_addr ; → [addr_low addr_high addr_low addr_high]
    DEFS 100, $00                ; → 100 bytes of $00
```

## Benefits

### Immediate Impact
- **+4% success rate** - Handle data directive patterns
- **Game compatibility** - Enable sprite data, level maps, sound tables
- **System programming** - Support memory buffers, constant tables
- **MinZ compatibility** - Handle more generated assembly patterns

### Long-term Benefits
- **Foundation for complex data** - Arrays, structures, lookup tables
- **Debugging support** - Data can be labeled and referenced
- **Performance optimization** - Pre-computed tables vs runtime calculation
- **Ecosystem growth** - Enable more sophisticated MinZ programs

## Test Coverage

### Unit Tests
```go
func TestDEFB(t *testing.T) {
    tests := []struct {
        input    []string
        expected []byte
    }{
        {[]string{"$01", "$02", "$03"}, []byte{0x01, 0x02, 0x03}},
        {[]string{"65", "66", "67"}, []byte{65, 66, 67}},
        {[]string{"\"Hi\"", "0"}, []byte{'H', 'i', 0}},
    }
    
    for _, test := range tests {
        a := NewAssembler()
        err := a.processDEFB(test.input)
        assert.NoError(t, err)
        assert.Equal(t, test.expected, a.output)
    }
}
```

### Integration Tests
```asm
; Test file: test_directives.a80
    ORG $8000
    
data_section:
    DEFB $01, $02, $03
    DEFW $1234, $5678  
    DB "Hello", 0
    DEFS 10, $FF
    
code_section:
    LD A, (data_section)
    RET
    END
```

## Implementation Plan

### Phase 1: Core Directives (1 day)
- [ ] Implement DEFB/DB with numeric values
- [ ] Implement DEFW/DW with numeric values  
- [ ] Implement DEFS/DS with size and fill
- [ ] Add directive detection and routing

### Phase 2: String Support (0.5 day)
- [ ] Add string literal parsing for DEFB/DB
- [ ] Implement escape sequence handling
- [ ] Test with MinZ-generated string constants

### Phase 3: Integration (0.5 day)
- [ ] Update instruction processing pipeline
- [ ] Add enhanced error messages for directive failures
- [ ] Test against corpus for success rate improvement

## Success Metrics

### Quantitative
- **Success rate improvement**: 12% → 16% (+4%)
- **Directive coverage**: 100% of DEFB, DEFW, DEFS patterns
- **String literal support**: Basic escape sequences
- **Performance**: <1ms per directive processing

### Qualitative
- **MinZ compatibility**: Handle more generated assembly patterns
- **Developer experience**: Clear errors for directive issues
- **Feature completeness**: Standard Z80 assembler directive set

## Future Enhancements

### Advanced Features
- **INCLUDE directive** - File inclusion support
- **EQU/SET directives** - Symbol definition
- **MACRO support** - Code generation macros
- **Conditional assembly** - IF/ELSE/ENDIF blocks

This ADR establishes essential data directive support that will significantly improve MZA's ability to handle MinZ-generated assembly! 🚀