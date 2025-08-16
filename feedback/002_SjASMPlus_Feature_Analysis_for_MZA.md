# 002: SjASMPlus Feature Analysis for MZA Enhancement

**From**: MZA Verification Colleague  
**To**: MinZ Development Team  
**Date**: 2025-08-16  
**Purpose**: Identify quick wins for MZA/SjASMPlus compatibility

## Executive Summary

SjASMPlus has many features that MZA could adopt. Ranked by **нужность для MinZ** (necessity for MinZ) and implementation speed.

## 🎯 Quick Wins (1-2 days work, HIGH impact)

### 1. **Fake/Pseudo Instructions** ⭐⭐⭐⭐⭐
**Нужность**: CRITICAL - Fixes invalid MinZ codegen immediately

```asm
; SjASMPlus supports these "fake" instructions:
LD HL, DE    →  LD H, D : LD L, E
LD BC, HL    →  LD B, H : LD C, L
LD DE, BC    →  LD D, B : LD E, C

; MZA should add:
LD BC, DE    →  LD B, D : LD C, E
LD SP, HL    →  LD SP, HL  (already valid)
LD IX, BC    →  LD IXH, B : LD IXL, C
```

**Why Critical**: MinZ currently generates these invalid instructions!

### 2. **Multi-Argument PUSH/POP** ⭐⭐⭐⭐⭐
**Нужность**: HIGH - Makes function prologue/epilogue cleaner

```asm
; SjASMPlus allows:
PUSH AF, BC, DE, HL    →  PUSH AF : PUSH BC : PUSH DE : PUSH HL
POP HL, DE, BC, AF     →  POP HL : POP DE : POP BC : POP AF

; MinZ benefits:
; Function entry/exit becomes one line instead of 4
```

### 3. **Local Labels with `.` prefix** ⭐⭐⭐⭐
**Нужность**: HIGH - Solves label collision issues

```asm
main:
.loop:          ; Local to main
    DJNZ .loop  ; References local label
    
other_func:
.loop:          ; Different .loop, no collision!
    JR .loop
```

**Why Important**: MinZ generates many loop/conditional labels that collide

### 4. **String Escape Sequences** ⭐⭐⭐⭐
**Нужность**: HIGH - Fixes string generation issues

```asm
; SjASMPlus supports:
DB "Hello\nWorld"       ; \n → newline
DB "Quote: \"test\""    ; \" → quote
DB 'Single quotes'      ; Alternative quoting
DB "Tab\there"          ; \t → tab
```

**Current Problem**: MinZ uses `\'` which SjASMPlus doesn't recognize

## 🚀 Mid Wins (1 week work, MEDIUM impact)

### 5. **Expression Evaluation** ⭐⭐⭐
**Нужность**: MEDIUM - Enables compile-time calculations

```asm
; SjASMPlus supports:
BUFFER_SIZE EQU 256
LD HL, BUFFER_SIZE * 2    ; Evaluates to 512
LD A, (label + 5)         ; Address arithmetic
```

### 6. **Conditional Assembly** ⭐⭐⭐
**Нужность**: MEDIUM - Platform-specific code

```asm
IFDEF SPECTRUM
    OUT (254), A    ; Spectrum border
ELSE
    ; CP/M version
ENDIF
```

### 7. **Module/Namespace Support** ⭐⭐⭐
**Нужность**: MEDIUM - Better code organization

```asm
MODULE graphics
draw:
    ; graphics.draw
ENDMODULE

    CALL graphics.draw
```

### 8. **Repeat Blocks (DUP/REPT)** ⭐⭐
**Нужность**: LOW-MEDIUM - Data generation

```asm
    DUP 10
    NOP
    EDUP    ; Generates 10 NOPs
```

## 🐌 Slow Wins (2+ weeks, LOW priority for MinZ)

### 9. **Lua Scripting Engine** ⭐
**Нужность**: LOW - Complex but rarely needed

```asm
    LUA
    for i = 1, 10 do
        _pc("DB " .. i)
    end
    ENDLUA
```

### 10. **Device Emulation Modes** ⭐
**Нужность**: LOW - Platform-specific

```asm
DEVICE ZXSPECTRUM48
    ; Memory mapping for Spectrum
```

### 11. **Snapshot Generation** ⭐
**Нужность**: LOW - Not compiler's job

```asm
SAVESNA "output.sna", start
SAVETAP "output.tap", start
```

## 📊 Implementation Priority Matrix

| Feature | Нужность | Effort | Priority | Why |
|---------|----------|--------|----------|-----|
| Fake instructions | ⭐⭐⭐⭐⭐ | 1 day | **#1** | Fixes invalid MinZ output |
| Multi-arg PUSH/POP | ⭐⭐⭐⭐⭐ | 2 hours | **#2** | Clean function frames |
| Local labels | ⭐⭐⭐⭐ | 1 day | **#3** | Prevents label collisions |
| String escapes | ⭐⭐⭐⭐ | 4 hours | **#4** | Fixes string generation |
| Expression eval | ⭐⭐⭐ | 3 days | #5 | Compile-time math |
| Conditional asm | ⭐⭐⭐ | 2 days | #6 | Platform targeting |
| Modules | ⭐⭐⭐ | 1 week | #7 | Code organization |
| Repeat blocks | ⭐⭐ | 1 day | #8 | Data tables |
| Lua scripting | ⭐ | 2 weeks | #9 | Overkill for MinZ |
| Device modes | ⭐ | 1 week | #10 | Platform-specific |

## 🎯 Recommended MZA Roadmap

### Phase 1: Critical Fixes (This Week)
```go
// Add to MZA assembler
type PseudoInstruction struct {
    Pattern  string   // "LD HL, DE"
    Expand   []string // ["LD H, D", "LD L, E"]
}

var pseudoInstructions = []PseudoInstruction{
    {"LD HL, DE", []string{"LD H, D", "LD L, E"}},
    {"LD BC, HL", []string{"LD B, H", "LD C, L"}},
    // ... etc
}
```

### Phase 2: Enhanced Syntax (Next Week)
- Multi-argument PUSH/POP
- Local label support
- String escape sequences

### Phase 3: Advanced Features (Month 2)
- Expression evaluation
- Conditional assembly
- Module support

## 💡 MinZ Code Generation Improvements

Based on SjASMPlus features, MinZ should:

1. **Use local labels** for loops/conditions:
```asm
main:
.if_1:      ; Instead of global "if_1"
.loop_1:    ; Instead of global "loop_1"
```

2. **Use multi-arg PUSH/POP**:
```asm
; Instead of:
PUSH AF
PUSH BC
PUSH DE

; Generate:
PUSH AF, BC, DE
```

3. **Use proper string escapes**:
```asm
DB "Line 1\nLine 2"    ; Not "Line 1\\nLine 2"
DB "Say \"Hello\""     ; Not "Say \\"Hello\\""
```

## Test Case for Compatibility

```asm
; test_sjasmplus_compat.asm
    ORG $8000

; Test pseudo-instructions
    LD HL, DE           ; Must work
    LD BC, HL           ; Must work
    
; Test multi-arg
    PUSH AF, BC, DE     ; Must work
    
; Test local labels
main:
.loop:
    DJNZ .loop          ; Must work
    
; Test strings
    DB "Hello\nWorld"   ; Must work
    DB "Quote: \"hi\""  ; Must work
    
    END
```

Success = Both MZA and SjASMPlus assemble identically!

## Conclusion

**Quick wins** (fake instructions, multi-arg PUSH/POP, local labels, string escapes) would solve **80% of MinZ's compatibility issues** with just 2-3 days of work. These are the features MinZ actually needs and generates.

The advanced features (Lua, device modes) are nice-to-have but not essential for a compiler's assembler output.

**Recommendation**: Implement Phase 1 immediately to unblock MinZ development!