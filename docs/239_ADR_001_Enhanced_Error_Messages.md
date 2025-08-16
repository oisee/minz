# ADR-001: Enhanced Error Messages with Contextual Suggestions

## Status
**ACCEPTED** - Implementation in progress

## Context

### Current Problem
MZA error messages are cryptic and unhelpful for debugging:
```
Assembly errors:
  line 4: undefined symbol: (HL)
  line 13: undefined symbol: ($8100)
  line 14: unsupported indirect addressing: ($8100), A
```

Users have no guidance on how to fix these issues, leading to:
- **Poor developer experience** - Cryptic errors frustrate users
- **Reduced effective success rate** - Users can't fix solvable issues  
- **Support burden** - Same questions asked repeatedly
- **Abandoned adoption** - Users give up due to unclear errors

### Success Rate Impact
Current **12%** success rate could improve to **~14%** just by helping users fix common issues.

## Decision

### Implement Contextual Error System
Create a structured error system that provides:
1. **Clear error descriptions** - What went wrong
2. **Contextual suggestions** - How to fix it
3. **Pattern-based guidance** - Common solutions for common problems
4. **Progressive disclosure** - Basic message + detailed help

### Error Categories
1. **Undefined Symbol Errors** - Most common issue
2. **Unsupported Instruction Errors** - Feature gaps
3. **Syntax Errors** - Format problems
4. **Address/Memory Errors** - Range and mapping issues

## Implementation

### 1. Structured Error Types
```go
type AssemblyError struct {
    Line        int
    Column      int
    Type        ErrorType
    Context     string      // Problematic text
    Message     string      // Clear description
    Suggestion  string      // How to fix
    Examples    []string    // Valid alternatives
    HelpURL     string      // Link to docs
}

type ErrorType int
const (
    UndefinedSymbol ErrorType = iota
    UnsupportedInstruction
    InvalidSyntax
    AddressOutOfRange
    UnknownDirective
)
```

### 2. Smart Suggestion Engine
```go
func NewUndefinedSymbolError(line int, symbol string) AssemblyError {
    var suggestion string
    var examples []string
    
    // Pattern-based suggestions
    switch {
    case isRegisterIndirect(symbol): // (HL), (BC), etc.
        suggestion = "Register indirect addressing may not be supported for this instruction"
        examples = []string{
            "Use immediate addressing: LD A, $12",
            "Check if instruction supports (HL): LD A, (HL)",
        }
        
    case isMemoryIndirect(symbol): // ($8100), etc.
        suggestion = "Memory indirect addressing requires proper address format"
        examples = []string{
            "Ensure hex format: LD HL, ($8000)",
            "Use defined symbols: LD A, (buffer_addr)",
        }
        
    case isLikelyTypo(symbol): // Common misspellings
        suggestion = fmt.Sprintf("Symbol '%s' not defined. Did you mean '%s'?", 
                                 symbol, suggestCorrection(symbol))
        examples = []string{
            "Check spelling and case sensitivity",
            "Ensure symbol is defined before use",
        }
        
    default:
        suggestion = "Symbol not defined in current scope"
        examples = []string{
            "Define symbol with: label:",
            "Check for typos in symbol name",
        }
    }
    
    return AssemblyError{
        Line: line,
        Type: UndefinedSymbol,
        Context: symbol,
        Message: fmt.Sprintf("undefined symbol: %s", symbol),
        Suggestion: suggestion,
        Examples: examples,
        HelpURL: "https://docs.minz.dev/mza/symbols",
    }
}
```

### 3. Pattern Recognition
```go
func isRegisterIndirect(symbol string) bool {
    if !strings.HasPrefix(symbol, "(") || !strings.HasSuffix(symbol, ")") {
        return false
    }
    inner := symbol[1:len(symbol)-1]
    return inner == "HL" || inner == "BC" || inner == "DE" || inner == "SP"
}

func isMemoryIndirect(symbol string) bool {
    if !strings.HasPrefix(symbol, "(") || !strings.HasSuffix(symbol, ")") {
        return false
    }
    inner := symbol[1:len(symbol)-1]
    // Check if it looks like a memory address
    return strings.HasPrefix(inner, "$") || 
           strings.HasPrefix(inner, "0x") ||
           isAllDigits(inner)
}

func isLikelyTypo(symbol string) bool {
    // Check against common symbols and suggest corrections
    commonSymbols := []string{"main", "loop", "end", "start", "buffer"}
    return findClosestMatch(symbol, commonSymbols) != ""
}
```

### 4. Enhanced Error Formatting
```go
func (e AssemblyError) Format() string {
    var buf strings.Builder
    
    // Main error message
    fmt.Fprintf(&buf, "Line %d: %s\n", e.Line, e.Message)
    
    // Context highlighting
    if e.Context != "" {
        fmt.Fprintf(&buf, "  Problem: '%s'\n", e.Context)
    }
    
    // Suggestion
    if e.Suggestion != "" {
        fmt.Fprintf(&buf, "  💡 %s\n", e.Suggestion)
    }
    
    // Examples
    if len(e.Examples) > 0 {
        fmt.Fprintf(&buf, "  Examples:\n")
        for _, example := range e.Examples {
            fmt.Fprintf(&buf, "    • %s\n", example)
        }
    }
    
    // Help link
    if e.HelpURL != "" {
        fmt.Fprintf(&buf, "  📖 Help: %s\n", e.HelpURL)
    }
    
    return buf.String()
}
```

## Expected Outcomes

### Before (Current)
```
Assembly errors:
  line 4: undefined symbol: (HL)
  line 13: undefined symbol: ($8100)
```

### After (Enhanced)
```
Line 4: undefined symbol: (HL)
  Problem: '(HL)'
  💡 Register indirect addressing may not be supported for this instruction
  Examples:
    • Use immediate addressing: LD A, $12
    • Check if instruction supports (HL): LD A, (HL)
  📖 Help: https://docs.minz.dev/mza/addressing

Line 13: undefined symbol: ($8100)
  Problem: '($8100)'
  💡 Memory indirect addressing requires proper address format
  Examples:
    • Ensure hex format: LD HL, ($8000)
    • Use defined symbols: LD A, (buffer_addr)
  📖 Help: https://docs.minz.dev/mza/memory
```

## Benefits

### Immediate Impact
- **+2% success rate** - Users can fix common issues
- **Better UX** - Clear guidance instead of cryptic errors
- **Reduced support** - Self-explanatory error messages
- **Learning tool** - Helps users understand Z80 assembly

### Long-term Benefits
- **Foundation for tooling** - IDEs can use structured errors
- **Analytics potential** - Track common error patterns
- **Documentation driver** - Errors point to relevant docs
- **Ecosystem growth** - Better developer experience drives adoption

## Implementation Plan

### Phase 1: Core Infrastructure (1 day)
- [ ] Define `AssemblyError` struct and error types
- [ ] Implement basic formatting and suggestion engine
- [ ] Add pattern recognition for common cases

### Phase 2: Smart Suggestions (1 day)  
- [ ] Implement undefined symbol pattern detection
- [ ] Add context-aware suggestions for addressing modes
- [ ] Create typo detection and correction suggestions

### Phase 3: Integration (1 day)
- [ ] Replace existing error handling with structured errors
- [ ] Update all error generation sites
- [ ] Test error message quality and coverage

## Risks and Mitigations

### Performance Impact
- **Risk**: Error formatting adds overhead
- **Mitigation**: Only format errors when they occur (rare case)

### Maintenance Burden
- **Risk**: Error messages need updates as features change
- **Mitigation**: Centralized error generation, pattern-based suggestions

### User Confusion  
- **Risk**: Too much information overwhelms users
- **Mitigation**: Progressive disclosure, optional verbose mode

## Success Metrics

### Quantitative
- **Error clarity score** - Survey users on error helpfulness
- **Resolution rate** - % of errors users can fix themselves  
- **Support ticket reduction** - Fewer questions about cryptic errors

### Qualitative  
- **User feedback** - "Much clearer error messages!"
- **Adoption metrics** - Users try more fixes before giving up
- **Community growth** - Better experience drives word-of-mouth

## Future Enhancements

### Advanced Features
- **Error recovery suggestions** - Auto-fix common patterns
- **IDE integration** - Structured errors for language servers
- **Internationalization** - Error messages in multiple languages
- **Analytics dashboard** - Track error patterns across users

This ADR establishes the foundation for dramatically improved developer experience in MZA! 🚀