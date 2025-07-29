# TSMC Reference Implementation Plan

## Current State Analysis

Looking at our generated code, we're still thinking in pointer terms:

```asm
; Current (pointer-thinking):
LD ($F002), HL    ; Store pointer to memory
LD HL, ($F002)    ; Load pointer from memory  
LD A, (HL)        ; Dereference pointer

; TSMC way (reference-thinking):
data$immOP:
    LD A, 0       ; Direct immediate use
data$imm0 EQU data$immOP+1
```

## Implementation Phases

### Phase 1: Immediate Reinterpretation

**Goal**: Make current pointer syntax generate TSMC-friendly code

1. **Identify reference parameters at semantic analysis**
   ```minz
   fun process(data: *u8) -> u8 {  // Mark as reference, not pointer
       return *data;                // This becomes immediate load
   }
   ```

2. **Generate anchors for reference parameters**
   ```asm
   process:
   data$immOP:
       LD A, 0      ; Immediate slot for data value
   data$imm0 EQU data$immOP+1
       RET
   ```

3. **Patch at call sites**
   ```asm
   ; Instead of:
   LD HL, data_address
   CALL process
   
   ; Generate:
   LD A, (data_address)      ; Load actual value
   LD (data$imm0), A        ; Patch into immediate
   CALL process
   ```

### Phase 2: Array/Struct References as Base Addresses

For larger types, references become base address immediates:

```minz
fun clear(buf: *[256]u8) {
    // buf is a constant base address
}
```

Generates:
```asm
clear:
    LD B, 0
.loop:
buf$immOP:
    LD HL, 0     ; Base address patched here
buf$imm0 EQU buf$immOP+1
    ; ... rest of function
```

### Phase 3: Detect Dereference Patterns

Transform dereference operations based on context:

1. **Simple dereference**: `*ptr` → immediate load
2. **Array access**: `ptr[i]` → base + index with immediate base
3. **Field access**: `ptr.field` → immediate base + fixed offset

### Technical Changes Needed

1. **In semantic analyzer**:
   - Flag parameters that should use TSMC references
   - Track which variables are references vs true pointers
   - Generate different IR for reference operations

2. **New IR operations**:
   ```go
   OpTSMCParam     // Mark parameter as TSMC reference
   OpTSMCLoad      // Load from immediate slot
   OpTSMCPatch     // Patch immediate slot
   ```

3. **In code generator**:
   - Emit anchors for TSMC parameters
   - Generate patch sequences at call sites
   - Optimize multiple uses to reuse patched values

### Example Transformation

**Before (pointer style)**:
```minz
fun strlen(str: *u8) -> u16 {
    let mut len = 0;
    let mut p = str;
    while *p != 0 {
        len += 1;
        p += 1;
    }
    return len;
}
```

**After (TSMC reference style)**:
```asm
strlen:
    LD BC, 0        ; len counter
str$immOP:
    LD HL, 0        ; Base address immediate
str$imm0 EQU str$immOP+1
    
.loop:
    LD A, (HL)      ; Direct load from current position
    OR A
    RET Z           ; Return if zero
    INC BC          ; len++
    INC HL          ; Move to next byte
    JR .loop
```

But wait! We can go further with self-modification:

```asm
strlen:
    LD BC, 0        ; len counter
.loop:
str$immOP:
    LD A, (0)       ; This immediate gets modified!
str$imm0 EQU str$immOP+1
    OR A
    RET Z
    INC BC
    ; Self-modify for next iteration
    LD HL, (str$imm0)
    INC HL
    LD (str$imm0), HL
    JR .loop
```

### Benefits Over Current Implementation

1. **No register pressure** - Don't need HL to hold addresses
2. **Faster loops** - Self-modifying addresses avoid ADD HL,DE
3. **True TSMC** - Every parameter is a patch point
4. **Zero abstraction cost** - References compile to optimal code

### Migration Strategy

1. **Keep current syntax** - `*T` remains valid
2. **Reinterpret semantics** - `*T` means "TSMC reference to T"
3. **Add explicit pointer type** - `ptr<T>` for true pointers when needed
4. **Gradual optimization** - Start with simple cases, expand over time

### Key Insight

**In TSMC-MinZ, references aren't addresses pointing to data. They're addresses pointing to code locations where data will be patched.**

This flips traditional thinking:
- Traditional: Code uses registers to find data
- TSMC: Data is injected into code at compile/runtime

The program becomes a template, and function calls become template instantiations.