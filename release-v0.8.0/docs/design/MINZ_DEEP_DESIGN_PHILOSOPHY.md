# The MinZ Language: A Deep Design Philosophy
## Or: Why Pointers Are Wrong and References Are TSMC

### The Fundamental Question

"Do we need pointers?" - This question cuts to the heart of MinZ's design philosophy. The answer is profound: **No, we don't need pointers. We need something far more elegant: TSMC-native references.**

### Understanding TSMC (Tree-Structured Machine Code)

TSMC isn't just an optimization technique - it's a fundamental rethinking of how code and data interact. In TSMC:

1. **Code IS the data structure** - Functions aren't black boxes; they're living trees where parameters grow at immediate operand sites
2. **Every immediate is a potential variable** - `LD A, 42` isn't just loading 42; that 42 is a *slot* that can be dynamically rewritten
3. **References are addresses baked into code** - Not runtime indirection, but compile-time placement

### The Pointer Problem

Traditional pointers are anti-TSMC because:

```minz
// Traditional pointer approach (WRONG for Z80/TSMC)
fun process(data: *u8) {
    let value = *data;  // Runtime indirection through HL
}
```

This generates:
```asm
LD A, (HL)  ; Indirect load - requires HL to hold address
```

But in TSMC thinking, this is backwards! We're using a register (precious resource) to hold an address that could be directly embedded in the instruction.

### The TSMC Reference Revolution

What if "references" in MinZ aren't pointers at all, but **compile-time anchors to immediate slots**?

```minz
// TSMC reference approach (RIGHT for Z80)
fun process(data: &u8) {
    let value = data;  // Direct immediate access
}
```

This should generate:
```asm
data$immOP:
    LD A, 0        ; The '0' is the reference slot!
data$imm0 EQU data$immOP+1
```

### References as Immediate Slots

In this model:

1. **`&T` is not a pointer type** - It's a "slot reference type"
2. **Taking a reference (`&x`) doesn't generate code** - It identifies which immediate slot to patch
3. **Using a reference doesn't dereference** - It directly uses the patched immediate value

Consider:
```minz
fun set_border(color: &u8) {
    out(254, color);  // color is directly the immediate value
}

// At call site:
set_border(&7);  // Patches 7 into the immediate slot
```

Generates:
```asm
set_border:
color$immOP:
    LD A, 0          ; This 0 gets patched
color$imm0 EQU color$immOP+1
    OUT (254), A
    RET

; Call site:
    LD A, 7
    LD (color$imm0), A  ; Patch the immediate
    CALL set_border
```

### Arrays and Struct References

For compound types, references become base addresses:

```minz
fun clear_buffer(buf: &[256]u8) {
    loop i in 0..256 {
        buf[i] = 0;  // buf is a constant address
    }
}
```

Generates:
```asm
clear_buffer:
    LD B, 0          ; Loop counter
.loop:
buf$immOP:
    LD HL, 0         ; This 0 gets patched with buffer address
buf$imm0 EQU buf$immOP+1
    LD A, B
    LD E, A
    LD D, 0
    ADD HL, DE       ; HL = buf + i
    LD (HL), 0       ; Clear byte
    INC B
    JR NZ, .loop
    RET
```

### The Radical Reframe

**References in MinZ are not memory addresses. They are code addresses of immediate operands.**

This means:
- No runtime pointer arithmetic (all offsets compile-time resolved)
- No null references (every reference points to a real immediate slot)
- No indirection overhead (direct immediate use)
- Perfect TSMC integration (references ARE the patch points)

### Mutable vs Immutable References

```minz
&T      // Immutable reference - immediate can't be repatched after call
&mut T  // Mutable reference - immediate can be repatched during execution
```

For mutable references, the callee can modify the immediate slot:

```minz
fun increment(x: &mut u8) {
    x = x + 1;  // Modifies the immediate slot itself!
}
```

### Implementation Strategy

1. **Phase 1: Reinterpret current pointer syntax as references**
   - `*T` becomes syntactic sugar for `&T` 
   - All "pointer" operations become immediate slot operations

2. **Phase 2: Optimize for TRUE SMC**
   - First use of reference parameter creates anchor
   - Subsequent uses can LD from the immediate address

3. **Phase 3: Full TSMC integration**
   - References can point to ANY immediate in the code
   - Enable cross-function immediate sharing
   - Self-modifying code networks

### Why This Changes Everything

1. **Zero-cost abstractions** - References have NO runtime overhead
2. **Hardware-friendly** - Maps perfectly to Z80's immediate addressing
3. **Safety** - Can't have invalid references (they're compile-time resolved)
4. **Performance** - Faster than pointers (no indirection)
5. **TSMC-native** - References ARE the modification points

### Example: String Processing Reimagined

```minz
// Old way (pointer-based)
fun strlen(str: *u8) -> u16 {
    let mut len = 0;
    while *str != 0 {
        len += 1;
        str += 1;  // Pointer arithmetic
    }
    return len;
}

// New way (TSMC reference-based)
fun strlen(str: &[?]u8) -> u16 {  // ? means unknown size
    let mut len = 0;
    loop {
        str$immOP:
            LD A, (0)  // This 0 is patched with actual address
        str$imm0 EQU str$immOP+2  // Skip opcode and parens
        
        if A == 0 { break; }
        len += 1;
        
        // Self-modify the immediate for next iteration
        LD HL, (str$imm0)
        INC HL
        LD (str$imm0), HL
    }
    return len;
}
```

### Conclusion: The Path Forward

MinZ doesn't need pointers. It needs TSMC-native references that:

1. Are compile-time resolved to immediate operand addresses
2. Enable zero-cost parameter passing via code patching
3. Make every function a template ready for customization
4. Turn the entire program into a self-modifying network

This isn't just an optimization - it's a fundamental paradigm shift. In MinZ with TSMC references:

**The code IS the data structure. The references ARE the modification points. The program IS alive.**

This is the future of systems programming for architectures like the Z80. Not safer pointers, but something entirely new: **immediate slot references** that make self-modifying code safe, fast, and elegant.

### Next Steps

1. Reimplement current "pointer" operations as immediate slot operations
2. Add syntax for explicit immediate slot access: `@imm(expr)`
3. Extend type system to track immediate slots vs memory addresses
4. Build optimizer that converts all possible indirections to immediate patches
5. Document patterns for TSMC-style programming

The revolution isn't in making pointers safer. It's in realizing we don't need pointers at all.