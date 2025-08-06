# MIR vs LLVM IR: A Comprehensive Comparison

*A deep dive into intermediate representations in modern compilers*

---

## 1. Introduction: What Are Intermediate Representations?

Intermediate Representations (IRs) are the backbone of modern compilers, serving as the bridge between high-level source code and machine code. This article compares three significant IRs:

- **LLVM IR**: The universal backend used by Clang, Rust, Swift, and many others
- **Rust's MIR**: Mid-level IR specific to the Rust compiler
- **MinZ's MIR**: A unique hardware-specific IR for Z80 systems

Each serves different purposes and embodies different design philosophies.

---

## 2. LLVM IR: The Universal Backend

### 2.1 Core Characteristics
- **Purpose**: Hardware-agnostic code generation for multiple architectures
- **Form**: Static Single Assignment (SSA) with infinite virtual registers
- **Type System**: Rich, including integers of any bit width, floats, vectors, structs, arrays
- **Memory Model**: Abstract with explicit load/store operations

### 2.2 Example LLVM IR
```llvm
define i32 @add(i32 %a, i32 %b) {
entry:
  %sum = add i32 %a, %b
  ret i32 %sum
}
```

### 2.3 Key Features
- Platform-independent (x86, ARM, RISC-V, WebAssembly, etc.)
- Extensive optimization passes (100+ transformations)
- Well-defined semantics for undefined behavior
- Intrinsics for platform-specific operations
- Debug information preservation

---

## 3. Rust's MIR: The Safety Analyzer

### 3.1 Core Characteristics
- **Purpose**: Simplify Rust's complex features for analysis and optimization
- **Position**: Between HIR (High-level IR) and LLVM IR
- **Focus**: Borrow checking, lifetime analysis, pattern matching lowering
- **Form**: Control Flow Graph with typed temporaries

### 3.2 Example Rust MIR (Simplified)
```mir
fn add(_1: i32, _2: i32) -> i32 {
    bb0: {
        _0 = Add(_1, _2);
        return;
    }
}
```

### 3.3 Key Features
- Explicit drop points for RAII
- Simplified pattern matching (no nested patterns)
- Pre-monomorphization (generic)
- Borrow checker operates here
- Const evaluation happens at MIR level

---

## 4. MinZ's MIR: The Hardware-Aware Optimizer

### 4.1 Core Characteristics
- **Purpose**: Z80-specific optimization with self-modifying code support
- **User-Facing**: Programmers can write MIR directly via `@mir` blocks
- **Hardware-Mapped**: Direct correspondence to Z80 registers and operations
- **SMC-Aware**: First-class support for self-modifying code patterns

### 4.2 Example MinZ MIR
```mir
; MinZ uses length-prefixed strings (not null-terminated!)
; String = 1-byte length prefix (max 255 chars)
; LString = 2-byte length prefix (max 65535 chars)

Function get_string_len(str: String) -> u8
  Instructions:
    0: r1 = load str      ; String pointer
    1: r2 = load.u8 [r1]  ; First byte is the length!
    2: return r2          ; O(1) - just return it!

Function concat_strings(s1: String, s2: String, dest: *u8) -> u8
  @smc  ; Enable self-modifying code optimization
  Instructions:
    0: r1 = load s1          ; First string pointer
    1: r2 = load s2          ; Second string pointer
    2: r3 = load dest        ; Destination pointer
    3: r4 = load.u8 [r1]     ; Length of s1
    4: r5 = load.u8 [r2]     ; Length of s2
    5: r6 = r4 + r5          ; Total length
    6: store.u8 [r3], r6     ; Write new length prefix
    7: r3 = r3 + 1           ; Move past length byte
    8: r1 = r1 + 1           ; Skip s1 length prefix
    9: ; Copy first string
    10: copy_loop1:
    11: jump_if_zero r4, copy_s2
    12: r7 = load.u8 [r1]
    13: store.u8 [r3], r7
    14: r1 = r1 + 1
    15: r3 = r3 + 1
    16: r4 = r4 - 1
    17: jump copy_loop1
    18: copy_s2:
    19: r2 = r2 + 1          ; Skip s2 length prefix
    20: copy_loop2:
    21: jump_if_zero r5, done
    22: r7 = load.u8 [r2]
    23: store.u8 [r3], r7
    24: r2 = r2 + 1
    25: r3 = r3 + 1
    26: r5 = r5 - 1
    27: jump copy_loop2
    28: done:
    29: return r6            ; Return total length
```

### 4.3 Unique Features
- Z80 register tracking (A, B, C, D, E, H, L, IX, IY)
- Self-modifying code operations (`OpSMCLoadConst`, `OpTrueSMCPatch`)
- User-programmable via `@mir {}` blocks
- Cycle-accurate cost modeling
- Metaprogramming interpreter

---

## 5. Detailed Comparison Table

| Aspect | LLVM IR | Rust MIR | MinZ MIR |
|--------|---------|----------|----------|
| **1. Purpose** | Universal codegen | Rust-specific analysis | Z80 optimization |
| **2. Abstraction Level** | Low (close to assembly) | Mid (between HIR and LLVM) | Low (structured assembly) |
| **3. User Visibility** | Rarely seen/written | Internal only | User-programmable |
| **4. Type System** | Rich, flexible | Rust types | Simple (u8, u16, pointers) |
| **5. Memory Model** | Abstract load/store | Place-based | Z80 memory map |
| **6. Register Model** | Infinite virtual | Abstract temporaries | Physical Z80 registers |
| **7. Control Flow** | SSA form | CFG with basic blocks | Basic blocks with jumps |
| **8. Optimization Focus** | General purpose | Rust semantics | Z80/SMC specific |
| **9. Platform Support** | 15+ architectures | N/A (lowered to LLVM) | Z80 only |
| **10. Metaprogramming** | Limited | None | Full interpreter |

---

## 6. Common Ground: What They Share

### 6.1 Basic Block Structure
All three use basic blocks as fundamental units:
```
block_label:
    instruction1
    instruction2
    terminator (jump/return/branch)
```

### 6.2 SSA-Like Properties
- LLVM IR: Pure SSA with phi nodes
- Rust MIR: SSA-inspired with explicit assignments
- MinZ MIR: Register-based but with SSA-like temporaries

### 6.3 Function-Based Organization
All three organize code into functions with:
- Parameters
- Return types
- Local variables/registers
- Control flow between blocks

### 6.4 Type Information
Each maintains type information, though at different granularities:
- LLVM: Extensive type system
- Rust MIR: Full Rust type information
- MinZ MIR: Basic types sufficient for Z80

---

## 7. Key Differences: What Sets Them Apart

### 7.1 Hardware Abstraction
**LLVM IR**: Complete abstraction
```llvm
%result = call i32 @llvm.ctpop.i32(i32 %value)  ; Population count
```

**Rust MIR**: Language abstraction
```mir
_1 = move _2;  ; Move semantics explicit
drop(_1);      ; Destructor calls explicit
```

**MinZ MIR**: Hardware-specific
```mir
r1 = load A      ; Z80 accumulator
r2 = load BC     ; Z80 register pair
```

### 7.2 Self-Modifying Code Support
- **LLVM IR**: No support (undefined behavior)
- **Rust MIR**: No support (unsafe in Rust)
- **MinZ MIR**: First-class support with special operations

### 7.3 User Interaction Model
- **LLVM IR**: Write C/C++/Rust → Get LLVM IR
- **Rust MIR**: Invisible to users
- **MinZ MIR**: Direct programming via `@mir` blocks

---

## 8. Use Cases: When to Use What

### 8.1 LLVM IR Use Cases
- Cross-platform compilation
- JIT compilation
- Whole-program optimization
- Language implementation backends

### 8.2 Rust MIR Use Cases
- Borrow checking
- Const evaluation
- MIR-only optimizations (pre-monomorphization)
- Incremental compilation

### 8.3 MinZ MIR Use Cases
- Z80 system programming
- Self-modifying code generation
- Cycle-critical optimizations
- Teaching/learning assembly concepts

---

## 9. Optimization Strategies

### 9.1 LLVM IR Optimizations
- Inlining, vectorization, loop unrolling
- Dead code elimination, constant propagation
- Alias analysis, escape analysis
- Target-specific lowering

### 9.2 Rust MIR Optimizations
- Drop elaboration
- Generator state machines
- Match lowering
- Const propagation (MIR-level)

### 9.3 MinZ MIR Optimizations
- Register pair utilization (BC, DE, HL)
- Self-modifying code generation
- Z80-specific peephole optimizations
- Cycle counting and optimization

---

## 10. Evolution and Future Directions

### 10.1 LLVM IR Trends
- Better support for parallelism (MLIR integration)
- Improved debug information
- New architectures (RISC-V, quantum)
- Convergence with MLIR for ML workloads

### 10.2 Rust MIR Evolution
- Polonius (next-gen borrow checker)
- Better const evaluation
- Improved async/await lowering
- Potential exposure to macros

### 10.3 MinZ MIR Potential
- Complete TAS debugger integration
- Visual programming interface
- More SMC patterns
- Cross-compilation to modern ISAs

---

## 11. Practical Examples: Same Program, Three IRs

### 11.1 Simple Addition Function

**C Source**:
```c
int add(int a, int b) {
    return a + b;
}
```

**LLVM IR**:
```llvm
define i32 @add(i32 %a, i32 %b) {
entry:
  %sum = add nsw i32 %a, %b
  ret i32 %sum
}
```

**Rust MIR** (from equivalent Rust):
```mir
fn add(_1: i32, _2: i32) -> i32 {
    let mut _0: i32;  // return place
    bb0: {
        _0 = Add(move _1, move _2);
        return;
    }
}
```

**MinZ MIR** (8-bit version):
```mir
Function add(a: u8, b: u8) -> u8
  Instructions:
    0: r1 = load a
    1: r2 = load b
    2: r3 = r1 + r2
    3: return r3
```

---

## 12. Tooling and Ecosystem

### 12.1 LLVM IR Tools
- `opt`: Optimization driver
- `llc`: Code generator
- `lli`: Interpreter
- `llvm-dis/as`: Assembler/disassembler
- `llvm-link`: Linker

### 12.2 Rust MIR Tools
- `cargo rustc -- -Z dump-mir`: Dump MIR
- `MIRI`: MIR interpreter
- `rustc` internal passes

### 12.3 MinZ MIR Tools
- MIR visualizer (Graphviz output)
- MIR interpreter (metaprogramming)
- `@mir` blocks in source
- TAS debugger (in development)

---

## 13. Performance Characteristics

### 13.1 Compilation Speed
- **LLVM IR**: Slower (many optimization passes)
- **Rust MIR**: Medium (pre-LLVM optimizations)
- **MinZ MIR**: Fast (simpler, targeted)

### 13.2 Runtime Performance
- **LLVM IR**: Excellent (mature optimizations)
- **Rust MIR**: N/A (compiled to LLVM)
- **MinZ MIR**: Optimal for Z80 (hardware-aware)

### 13.3 Code Size
- **LLVM IR**: Variable (optimization trade-offs)
- **Rust MIR**: N/A
- **MinZ MIR**: Minimal (Z80 constraints)

---

## 14. Learning Curve

### 14.1 Difficulty to Learn
1. **MinZ MIR**: Easiest (close to assembly, simpler)
2. **LLVM IR**: Medium (well-documented, but complex)
3. **Rust MIR**: Hardest (requires Rust knowledge, internal)

### 14.2 Documentation Quality
1. **LLVM IR**: Excellent (extensive docs, tutorials)
2. **MinZ MIR**: Good (growing, user-focused)
3. **Rust MIR**: Limited (internal, developer-focused)

---

## 15. Conclusion: Different Tools for Different Jobs

### 15.1 Summary
- **LLVM IR**: The Swiss Army knife of compiler backends
- **Rust MIR**: The specialized tool for Rust's unique features
- **MinZ MIR**: The precision instrument for Z80 optimization

### 15.2 Key Takeaways
1. **Not all IRs are created equal** - each serves specific needs
2. **Hardware awareness varies** - from abstract (LLVM) to specific (MinZ)
3. **User visibility differs** - from hidden (Rust) to programmable (MinZ)
4. **Optimization goals diverge** - general vs. language vs. hardware
5. **Evolution continues** - all three are actively developed

### 15.3 The Future
As compilers evolve, we're seeing:
- More domain-specific IRs (like MLIR for ML)
- Better debugging and visualization tools
- Increased user control over optimization
- Hardware-software co-design influence

The diversity of IRs reflects the diversity of computing needs - from cloud servers running LLVM-compiled code, to embedded Rust systems checked by MIR, to retro Z80 computers optimized by MinZ MIR.

---

## 16. References and Further Reading

### LLVM IR
- [LLVM Language Reference Manual](https://llvm.org/docs/LangRef.html)
- [LLVM Programmer's Manual](https://llvm.org/docs/ProgrammersManual.html)
- "Getting Started with LLVM Core Libraries" by Bruno Cardoso Lopes

### Rust MIR
- [Rust Compiler Development Guide - MIR](https://rustc-dev-guide.rust-lang.org/mir/index.html)
- [MIR RFC](https://github.com/rust-lang/rfcs/blob/master/text/1211-mir.md)
- "Rust MIR: A Mid-level IR for the Rust Compiler" - Rust Blog

### MinZ MIR
- MinZ MIR Documentation (docs/110_MIR_Code_Emission_Design.md)
- MinZ MIR Interpreter Design (docs/126_MIR_Interpreter_Design.md)
- MinZ MIR Visualization Guide (docs/MIR_VISUALIZATION_GUIDE.md)

---

*This comparison reflects the state of these IRs as of August 2025. Compiler technology evolves rapidly, and features may change.*