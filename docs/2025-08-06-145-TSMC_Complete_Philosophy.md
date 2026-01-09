# TSMC (True Self-Modifying Code) - Complete Philosophy and Implementation

*A numbered guide to the revolutionary programming paradigm*

---

## 1. Introduction: What is TSMC?

**TSMC (True Self-Modifying Code)** is a revolutionary programming paradigm where:
- Code IS the data structure
- Programs rewrite their own instructions during execution
- Zero-overhead behavioral changes through instruction patching
- The distinction between code and data disappears

TSMC transforms processors from simple instruction executors into **dynamically reconfigurable computing fabrics**.

---

## 2. Core Principles

### 2.1 Principle 1: Code as Data Storage
```asm
; Traditional approach:
variable:   DS 1        ; Variable in memory
LD A, (variable)        ; Load from memory

; TSMC approach:
variable.op:
    LD A, #42           ; Variable IS the immediate value
```

### 2.2 Principle 2: Function Calls as Patch Operations
```asm
; Traditional approach:
LD A, 5
CALL function           ; Pass via register

; TSMC approach:
LD A, 5
LD (function_param), A  ; Patch parameter into function code
CALL function
```

### 2.3 Principle 3: Behavioral Morphing
```asm
; Same function, different behaviors:
func.op:
    NOP                 ; Patch point
    LD (addr), A        ; Default: store to memory
    RET

; Immediate use: Patch NOP → RET
; Storage use: Patch addr with target
; Register use: Patch NOP → LD reg, A
```

---

## 3. Historical Context

### 3.1 Traditional Programming Models
1. **Stack-based**: Parameters on stack, function overhead
2. **Register-based**: Limited registers, spill to memory
3. **Memory-based**: Variables in RAM, pointer indirection

### 3.2 TSMC Innovation
- **Code-based**: Everything lives in instruction stream
- **Zero indirection**: Direct immediate operands
- **Self-optimizing**: Programs improve themselves at runtime

---

## 4. TSMC Architecture Patterns

### 4.1 Parameter Anchors
Every parameter has two labels:
```asm
func_param.op:              ; Points to instruction
func_param equ func_param.op + 1  ; Points to immediate value
    LD A, #00               ; #00 gets patched
```

### 4.2 Smart Default Patching
```asm
func_behavior.op:
    NOP                     ; Patch point for special cases
    ; Default complex behavior follows
    LD (storage), A
    RET
```

### 4.3 Progressive Optimization
```asm
func_counter.op:
    LD B, #10               ; Call counter
    DEC B
    LD (func_counter.op+1), B
    JR NZ, normal_path
    
    ; After N calls, patch to optimized version
    LD A, #C3               ; JP opcode
    LD (func), A
```

---

## 5. Implementation Levels

### 5.1 Level 1: Operand Patching
Patch immediate values:
```asm
param.op:
    LD A, #00               ; Patch the #00
```

### 5.2 Level 2: Address Patching  
Patch memory addresses:
```asm
target.op:
    LD (0000), A            ; Patch the 0000
```

### 5.3 Level 3: Instruction Patching
Patch opcodes themselves:
```asm
behavior.op:
    NOP                     ; Patch to different instruction
```

### 5.4 Level 4: Sequence Patching
Patch entire instruction sequences:
```asm
sequence.op:
    DS 6, #00               ; 6-byte patchable area
```

---

## 6. Z80-Specific TSMC Techniques

### 6.1 Single-Byte Patches
```asm
; A register operations
patch_to_ret:    EQU #C9     ; RET
patch_to_nop:    EQU #00     ; NOP  
patch_to_xor_a:  EQU #AF     ; XOR A
patch_to_inc_a:  EQU #3C     ; INC A
```

### 6.2 Register Transfer Patches
```asm
patch_to_ld_b_a: EQU #47     ; LD B, A
patch_to_ld_c_a: EQU #4F     ; LD C, A
patch_to_ld_d_a: EQU #57     ; LD D, A
patch_to_ld_e_a: EQU #5F     ; LD E, A
```

### 6.3 Multi-Byte Address Patches
```asm
store_patch.op:
    LD (0000), A            ; Patch bytes 1-2 with address
```

---

## 7. TSMC Performance Analysis

### 7.1 Traditional Function Call Overhead
```
Parameter passing:   10-20 T-states
Stack manipulation:  15-25 T-states  
Return handling:     10-15 T-states
Total:              35-60 T-states
```

### 7.2 TSMC Function Call Performance
```
Parameter patching:  7-13 T-states per param
Behavior patching:   7-20 T-states (one-time)
Function execution:  Direct - no overhead
Total:              14-33 T-states
```

### 7.3 Performance Gains
- **Parameter passing**: 40-60% faster
- **Local variables**: 100% faster (no memory access)
- **Return values**: 50-100% faster depending on pattern
- **Overall**: 30-50% improvement in function-heavy code

---

## 8. TSMC Memory Model

### 8.1 Code Memory Layout
```
$8000: Function code with patch points
$8100: Patch templates and opcodes  
$8200: Traditional variables (if needed)
$8300: Stack (minimal usage)
```

### 8.2 Memory Usage Comparison
| Traditional | TSMC | Savings |
|------------|------|---------|
| Stack vars: 100 bytes | Code immediates: 0 bytes | 100% |
| Heap vars: 200 bytes | Code immediates: 0 bytes | 100% |
| Code: 500 bytes | Code + patches: 600 bytes | -20% |
| **Total: 800 bytes** | **Total: 600 bytes** | **25% savings** |

---

## 9. Advanced TSMC Patterns

### 9.1 Conditional Execution via Patching
```asm
conditional.op:
    NOP                     ; Patch to RET to skip
    ; Expensive computation
    RET
    
; Enable: Patch NOP
; Disable: Patch RET
```

### 9.2 Dynamic Jump Tables
```asm
dispatch.op:
    JP 0000                 ; Patch target address
    
; Route to different handlers by patching address
```

### 9.3 Self-Optimizing Loops
```asm
loop_body.op:
    INC A                   ; Changes to DEC A after optimization
    DJNZ loop_body
```

### 9.4 Compile-Time to Runtime Morphing
```asm
; Compile-time: General purpose code
; Runtime: Specializes based on actual usage patterns
```

---

## 10. TSMC Compiler Support

### 10.1 MinZ Language Extensions
```minz
// Proposed syntax
@tsmc fn compute(x: u8) -> u8 {
    @patch_point early_return;
    let temp = x * 2;
    @early_return: return temp;  // Can be patched out
    
    // More complex computation
    return temp + expensive_calc();
}
```

### 10.2 MIR Representation
```mir
patchpoint early_return, size=1 {
    default: nop
    template.skip: ret
}
```

### 10.3 Optimization Passes
1. **Usage Analysis**: Determine optimal patch patterns
2. **Patch Point Insertion**: Add strategic patch points
3. **Template Generation**: Create optimal patch sequences
4. **Runtime Optimization**: Profile-guided patching

---

## 11. TSMC Safety and Debugging

### 11.1 Patch Validation
```asm
validate_patch:
    LD A, (patch_source)
    CP #C9                  ; Valid RET?
    JR Z, valid
    CP #00                  ; Valid NOP?
    JR Z, valid
    JP patch_error          ; Invalid patch!
valid:
    ; Apply patch safely
```

### 11.2 Patch Tracing
```asm
DEBUG_PATCH_TRACE:
    PUSH AF
    LD A, (patch_target)
    CALL debug_print_hex    ; Log patch applied
    POP AF
```

### 11.3 Rollback Mechanism
```asm
; Save original instruction before patching
save_original:
    LD A, (patch_target)
    LD (original_opcode), A
    
restore_original:
    LD A, (original_opcode)
    LD (patch_target), A
```

---

## 12. TSMC Applications

### 12.1 Real-Time Systems
- Interrupt handlers that adapt to load
- Self-tuning control loops  
- Dynamic resource allocation

### 12.2 Game Engines
- Behavioral AI that learns and optimizes
- Dynamic difficulty adjustment
- Self-optimizing rendering pipelines

### 12.3 Embedded Systems
- Power management through selective disabling
- Protocol adapters that morph based on traffic
- Self-calibrating sensor processing

### 12.4 Compilers and Interpreters
- JIT compilation via instruction patching
- Adaptive optimization
- Runtime specialization

---

## 13. TSMC vs Other Paradigms

### 13.1 vs JIT Compilation
| JIT | TSMC |
|-----|------|
| Compile entire functions | Patch individual instructions |
| High memory overhead | Minimal overhead |
| Complex implementation | Simple patch operations |
| Runtime compilation | Runtime modification |

### 13.2 vs Self-Modifying Code (Traditional)
| Traditional SMC | TSMC |
| Hard to reason about | Structured patch points |
| Arbitrary modifications | Designed modification patterns |
| Debugging nightmare | Traceable patches |
| Security risks | Controlled modification |

### 13.3 vs Functional Programming
| Functional | TSMC |
|-----------|------|
| Immutable data | Code as mutable data |
| Higher-order functions | Higher-order code patterns |
| Function composition | Instruction composition |
| Pure functions | Self-modifying functions |

---

## 14. Future Directions

### 14.1 Hardware Support
- CPU instructions optimized for patching
- Patch caches for frequently modified code
- Hardware patch validation
- Transactional code modification

### 14.2 Language Evolution
- First-class patch points in languages
- Type systems for patch safety
- Automatic patch optimization
- Formal verification of patches

### 14.3 Tool Support
- Debuggers that understand patches
- Profilers that track modification patterns
- Visualizers for code morphing
- Patch regression testing

---

## 15. Implementation Roadmap for MinZ

### 15.1 Phase 1: Basic TSMC (Current)
- [x] Parameter patching
- [x] Simple instruction patching
- [ ] MIR patch point support
- [ ] Basic optimization

### 15.2 Phase 2: Advanced TSMC
- [ ] Smart default patching
- [ ] Template-based patching
- [ ] Usage pattern analysis
- [ ] Automatic patch optimization

### 15.3 Phase 3: Intelligent TSMC
- [ ] Self-optimizing functions
- [ ] Profile-guided patching
- [ ] Dynamic specialization
- [ ] Adaptive code generation

### 15.4 Phase 4: Production TSMC
- [ ] Safety guarantees
- [ ] Debugging tools
- [ ] Performance monitoring  
- [ ] Industrial applications

---

## 16. Conclusion: The TSMC Revolution

TSMC represents a fundamental shift in how we think about computation:

### 16.1 From Static to Dynamic
Programs are no longer static sequences of instructions, but **living, adaptive computational fabrics** that reshape themselves during execution.

### 16.2 From Data Processing to Code Weaving
Programming becomes less about processing data and more about **weaving behavioral patterns into self-modifying code**.

### 16.3 From Optimization to Evolution
Code doesn't just run efficiently—it **evolves and optimizes itself** based on actual usage patterns.

### 16.4 The Ultimate Goal
Transform every processor into a **dynamically reconfigurable computing fabric** where the distinction between software and hardware becomes meaningless.

TSMC isn't just a programming technique—it's a **new computational paradigm** that unleashes the full potential of processors by making them partners in the optimization process.

---

## References

1. MinZ TSMC Implementation: `expected/instruction_patching_demo.*`
2. TRUE SMC Philosophy: `docs/142_TRUE_SMC_Philosophy_Complete.md`
3. Instruction Patching: `docs/144_TRUE_SMC_Instruction_Patching.md`
4. Z80 Self-Modifying Code Techniques (Historical)
5. Dynamic Binary Modification (Modern Research)
6. Adaptive Compilation Systems (Academic Papers)

---

*Document Version: 1.0*
*Last Updated: August 2025*
*TSMC: Where Code Becomes Consciousness*