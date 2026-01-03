# ADR-0004: Character Literals in Assembly

## Status
Accepted

## Context
Writing Z80 assembly code with character constants was cumbersome and error-prone:

```asm
; Old way - had to use ASCII codes
LD A, 72    ; 'H'
LD A, 101   ; 'e'
LD A, 108   ; 'l'
LD A, 108   ; 'l'
LD A, 111   ; 'o'
```

This made assembly code:
- Hard to read and maintain
- Error-prone (easy to use wrong ASCII value)
- Tedious to write
- Difficult to review

Additionally, the `DB` directive couldn't handle strings with commas:
```asm
DB "Hello, World"  ; ERROR: comma interpreted as operand separator
```

## Decision
Extend the MinZ assembler (mza) to support character literals and improve string handling:

1. **Support character literals** in instructions: `LD A, 'H'` and `LD A, "H"`
2. **Support escape sequences**: `LD A, '\n'`, `LD A, '\t'`, etc.
3. **Fix DB directive parsing** to handle quoted strings with commas
4. **Allow mixed operands** in DB: `DB "Hello", 13, 10, 0`

## Implementation

### Character Literal Parsing
```go
// In parseNumber function
if (strings.HasPrefix(s, "'") && strings.HasSuffix(s, "'")) ||
   (strings.HasPrefix(s, "\"") && strings.HasSuffix(s, "\"")) {
    char := s[1 : len(s)-1]
    
    // Handle escape sequences
    switch char {
    case "\\n": return 10  // newline
    case "\\r": return 13  // carriage return
    case "\\t": return 9   // tab
    case "\\\\": return 92 // backslash
    // ... etc
    }
    
    // Regular character
    if len(char) == 1 {
        return int(char[0])
    }
}
```

### Quote-Aware DB Parsing
```go
// Track quote state when splitting operands
inQuotes := false
for _, char := range line {
    if char == '"' || char == '\'' {
        inQuotes = !inQuotes
    }
    if char == ',' && !inQuotes {
        // Split here
    }
}
```

## Consequences

### Positive
- **Readability**: Assembly code is self-documenting
- **Maintainability**: Easy to see what characters are being used
- **Compatibility**: Works with both single and double quotes
- **Modern feel**: Brings assembly closer to high-level languages
- **Error reduction**: No more ASCII table lookups

### Negative
- **Parser complexity**: More complex parsing logic required
- **Edge cases**: Must handle escaped quotes, special characters
- **Documentation**: Need to document supported escape sequences

### Neutral
- Generated binary unchanged (literals converted at assembly time)
- Assembly time slightly increased (negligible)

## Examples

### Before and After
```asm
; Before
LD A, 72        ; What character is this?
DB 72, 101, 108, 108, 111

; After  
LD A, 'H'       ; Clear and obvious!
DB "Hello"
```

### Complex Example
```asm
; Print "Hello, World!\n"
DB "Hello, World!", 13, 10, 0

; Control characters
LD A, '\n'      ; Newline
LD A, '\t'      ; Tab
LD A, '\\'      ; Backslash
LD A, '\''      ; Single quote
```

## Alternatives Considered

### Preprocessor macros
- **Pros**: Keeps assembler simple
- **Cons**: Additional build step, not integrated
- **Rejected because**: Poor developer experience

### Only support single quotes
- **Pros**: Simpler parser
- **Cons**: Inconsistent with MinZ string literals (double quotes)
- **Rejected because**: Confusing to have different quote rules

### ASCII function syntax
- **Pros**: Explicit about conversion
- **Cons**: Verbose, unlike other assemblers
- **Rejected because**: Character literals are industry standard

## References
- [NASM Character Constants](https://www.nasm.us/doc/nasmdoc3.html#section-3.4.2)
- [Z80 Assembler Comparison](http://www.z80.info/z80sasm.htm)
- Standard C character literal specification

## Related ADRs
- ADR-0003: Platform Independence (uses character literals in examples)

## Date
2025-08-09