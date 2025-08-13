# The Self-Hosting Dream: Can MinZ Compile Itself on Z80?

*August 13, 2025*

## Introduction: The Ultimate Compiler Achievement

Self-hosting - when a compiler can compile itself - represents the ultimate validation of a programming language. For MinZ, targeting Z80 processors with 64KB of memory, this presents unique challenges and opportunities. This article explores the feasibility of implementing both a self-hosted MinZ compiler and a MIR VM interpreter in MinZ itself.

## Part 1: Self-Hosted MinZ Compiler Feasibility

### The Dream Architecture

Imagine running this on a ZX Spectrum:
```minz
// minzc.minz - The MinZ compiler written in MinZ
import std.io;
import compiler.lexer;
import compiler.parser;
import compiler.codegen;

fun main() -> void {
    let source = io.read_file("program.minz");
    let tokens = lexer.tokenize(source);
    let ast = parser.parse(tokens);
    let mir = compiler.generate_mir(ast);
    let asm = codegen.emit_z80(mir);
    io.write_file("program.a80", asm);
}
```

### Memory Constraints Analysis

#### Current MinZ Compiler Size (Go Implementation)
- Binary size: ~9MB (with ANTLR parser)
- Runtime memory: 50-100MB typical
- AST nodes: ~200 bytes per node
- Symbol tables: ~1KB per 100 symbols

#### Z80 Memory Available
- Total RAM: 48KB usable (ZX Spectrum)
- After system: ~40KB available
- With banking: up to 128KB (Spectrum +2/+3)

### Feasibility Assessment: EXTREMELY CHALLENGING

#### Major Obstacles

1. **Parser Complexity**
   - ANTLR grammar: ~1300 lines
   - Parse tree construction needs significant memory
   - Recursive descent would exhaust stack quickly
   - **Verdict**: Need simplified grammar for self-hosting

2. **AST Memory Requirements**
   ```minz
   // Even a simple function creates multiple nodes
   fun add(a: u8, b: u8) -> u8 {  // FunctionDecl node (32 bytes)
       return a + b;               // ReturnStmt + BinaryExpr (48 bytes)
   }                              // Total: ~80 bytes for trivial function
   ```
   - Medium program: 500+ AST nodes = 40KB+ memory
   - **Verdict**: Need streaming/incremental compilation

3. **Symbol Table Management**
   - Each symbol: ~40 bytes (name, type, scope, attributes)
   - 100 symbols = 4KB
   - Nested scopes multiply requirements
   - **Verdict**: Need compact symbol representation

4. **Code Generation Buffers**
   - MIR intermediate: 2-3x source size
   - Assembly output: 5-10x source size
   - Can't fit all in memory simultaneously
   - **Verdict**: Need file-based intermediate storage

### Proposed Solution: Minimal Self-Hosting Subset

#### Phase 1: TinyMinZ Compiler
A dramatically simplified MinZ subset that can compile itself:

```minz
// TinyMinZ - A self-compilable subset
// Only supports:
// - u8/u16 types
// - Functions (no lambdas)
// - Basic expressions
// - No metaprogramming
// - No modules

struct Token {
    type: u8,      // TokenType enum
    value: u16,    // Symbol table index
}

struct ASTNode {
    type: u8,      // NodeType enum  
    data: u16,     // Packed data
    child: u16,    // First child index
    next: u16,     // Next sibling index
}

fun compile_file(input: FileHandle, output: FileHandle) -> bool {
    let tokenizer = create_tokenizer(input);
    let parser = create_parser(tokenizer);
    
    // Single-pass compilation
    while !parser.at_end() {
        let stmt = parser.parse_statement();
        let asm = codegen_statement(stmt);
        write_assembly(output, asm);
        free_ast(stmt);  // Immediately free memory
    }
    
    return true;
}
```

#### Phase 2: Memory-Mapped Compilation
Using banking or disk overlays:

```minz
// Bank 0: Lexer (16KB)
// Bank 1: Parser (16KB)
// Bank 2: Code Generator (16KB)
// Bank 3: Symbol Table (16KB)

@target("spectrum128")
fun compile_with_banking() -> void {
    set_bank(0);  // Load lexer
    let tokens = tokenize_to_file("temp.tok");
    
    set_bank(1);  // Load parser
    let ast = parse_to_file("temp.ast");
    
    set_bank(2);  // Load codegen
    generate_assembly("output.a80");
}
```

### Realistic Timeline

1. **Year 1**: TinyMinZ subset compiler (10% of features)
2. **Year 2**: Add control flow, basic types (25% of features)
3. **Year 3**: Add structs, arrays (40% of features)
4. **Year 4**: Memory banking support (60% of features)
5. **Year 5**: Near-complete self-hosting (80% of features)

## Part 2: MIR VM Implementation in MinZ

### The MIR Virtual Machine Concept

MIR (MinZ Intermediate Representation) could run as a bytecode interpreter:

```minz
// mir_vm.minz - A MIR interpreter in MinZ
struct MIRInstruction {
    opcode: u8,
    dst: u8,
    src1: u8,
    src2: u8,
}

struct MIRVM {
    memory: [u8; 16384],     // 16KB VM memory
    registers: [u16; 256],   // 256 virtual registers
    pc: u16,                 // Program counter
    sp: u16,                 // Stack pointer
}

fun execute_mir(vm: *mut MIRVM, program: *MIRInstruction) -> void {
    loop {
        let inst = program[vm.pc];
        
        case inst.opcode {
            OP_MOVE => vm.registers[inst.dst] = vm.registers[inst.src1],
            OP_ADD => vm.registers[inst.dst] = 
                      vm.registers[inst.src1] + vm.registers[inst.src2],
            OP_LOAD => vm.registers[inst.dst] = 
                      vm.memory[vm.registers[inst.src1]],
            OP_STORE => vm.memory[vm.registers[inst.dst]] = 
                       vm.registers[inst.src1],
            OP_JUMP => vm.pc = inst.src1 as u16,
            OP_HALT => break,
            _ => error("Unknown opcode"),
        }
        
        vm.pc = vm.pc + 1;
    }
}
```

### MIR VM Feasibility: HIGHLY FEASIBLE

#### Advantages

1. **Simple Instruction Set**
   - ~50 opcodes total
   - Fixed instruction format
   - No complex addressing modes
   - Easy to decode and execute

2. **Manageable Memory**
   ```minz
   // Compact MIR program representation
   struct CompactMIR {
       code: [u8; 8192],      // 8KB code segment
       data: [u8; 4096],      // 4KB data segment
       stack: [u8; 2048],     // 2KB stack
       heap: [u8; 2048],      // 2KB heap
   }
   ```

3. **Performance Acceptable**
   - 10-20x slower than native
   - Still usable for development
   - Can optimize hot paths

### Practical MIR VM Implementation

#### Phase 1: Basic Interpreter (2-3 months)
```minz
// Basic MIR interpreter with core opcodes
const OP_MOVE: u8 = 0x01;
const OP_ADD: u8 = 0x10;
const OP_SUB: u8 = 0x11;
const OP_MUL: u8 = 0x12;
const OP_LOAD: u8 = 0x20;
const OP_STORE: u8 = 0x21;
const OP_JUMP: u8 = 0x30;
const OP_JUMP_IF: u8 = 0x31;
const OP_CALL: u8 = 0x40;
const OP_RET: u8 = 0x41;
const OP_HALT: u8 = 0xFF;

fun interpret_mir(bytecode: *u8, size: u16) -> u8 {
    let vm = MIRVM {
        memory: [0; 16384],
        registers: [0; 256],
        pc: 0,
        sp: 0x3FFF,  // Stack grows down
    };
    
    // Load bytecode into VM memory
    memcpy(vm.memory, bytecode, size);
    
    // Main interpretation loop
    while vm.pc < size {
        let result = execute_instruction(&mut vm);
        if result == HALT {
            break;
        }
    }
    
    return vm.registers[0] as u8;  // Return value in r0
}
```

#### Phase 2: Optimized Dispatcher (1-2 months)
```minz
// Threaded code interpreter for better performance
fun execute_threaded(vm: *mut MIRVM) -> void {
    // Jump table for fast dispatch
    let handlers: [fn(*mut MIRVM); 256] = [
        handle_move,    // 0x01
        handle_add,     // 0x10
        handle_sub,     // 0x11
        // ... more handlers
    ];
    
    loop {
        let opcode = vm.memory[vm.pc];
        handlers[opcode](vm);
        
        if vm.halted {
            break;
        }
    }
}

@inline
fun handle_add(vm: *mut MIRVM) -> void {
    let inst = decode_instruction(vm.memory + vm.pc);
    vm.registers[inst.dst] = 
        vm.registers[inst.src1] + vm.registers[inst.src2];
    vm.pc = vm.pc + 4;
}
```

#### Phase 3: JIT Compilation (6-12 months)
```minz
// Simple JIT for hot code paths
struct JITCache {
    entries: [JITEntry; 32],  // Cache 32 hot functions
    code_buffer: [u8; 4096],  // 4KB for generated code
    next_offset: u16,
}

fun jit_compile_block(mir: *MIRInstruction, count: u16) -> *u8 {
    let code = allocate_code_buffer(count * 4);
    
    for i in 0..count {
        case mir[i].opcode {
            OP_MOVE => {
                // Generate: LD A, register[src1]
                emit_byte(code, 0x3A);  // LD A, (addr)
                emit_word(code, &vm.registers[mir[i].src1]);
                // Generate: LD (register[dst]), A
                emit_byte(code, 0x32);  // LD (addr), A
                emit_word(code, &vm.registers[mir[i].dst]);
            },
            OP_ADD => {
                // Generate native Z80 ADD instruction
                emit_byte(code, 0x86);  // ADD A, (HL)
            },
            // ... more cases
        }
    }
    
    return code;
}
```

### Memory Requirements Comparison

| Component | Self-Hosted Compiler | MIR VM |
|-----------|---------------------|---------|
| Code Size | 30-40KB | 8-12KB |
| Data Size | 20-30KB | 4-8KB |
| Runtime Memory | 40-50KB | 16-20KB |
| **Total** | **90-120KB** | **28-40KB** |
| **Feasible on 48KB?** | ❌ No | ✅ Yes |
| **Feasible on 128KB?** | ⚠️ Maybe | ✅ Definitely |

## Part 3: Hybrid Approach - The Practical Path

### MIR VM as Bootstrap Environment

Instead of full self-hosting, use MIR VM to bootstrap:

```minz
// bootminz.minz - A bootstrap MinZ compiler
// Compiles MinZ -> MIR (not native Z80)
// Runs on MIR VM, produces MIR bytecode

fun compile_to_mir(source: String) -> *MIRProgram {
    // Simpler target - MIR instead of Z80
    let tokens = tokenize(source);
    let ast = parse_minimal(tokens);
    let mir = generate_mir(ast);
    return mir;
}

fun main() -> void {
    // Stage 1: Compile MinZ to MIR using MIR VM
    let mir_compiler = load_mir("bootminz.mir");
    let source = read_file("program.minz");
    let mir_output = run_mir_vm(mir_compiler, source);
    
    // Stage 2: Optionally compile MIR to native
    if has_native_compiler() {
        let native = mir_to_z80(mir_output);
        write_file("program.a80", native);
    } else {
        // Just run in VM
        run_mir_vm(mir_output, program_args);
    }
}
```

### Development Roadmap

#### Near Term (3-6 months)
1. Implement basic MIR VM in MinZ
2. Create minimal MIR assembler
3. Hand-compile core libraries to MIR

#### Medium Term (6-12 months)
1. Bootstrap minimal compiler in MIR
2. Add optimization passes
3. Implement debugging support

#### Long Term (1-2 years)
1. Extended language support
2. Banking/overlay system
3. Native code generation from VM

## Part 4: Real-World Constraints

### What Would Actually Run on Z80

#### Realistic MIR VM
- ✅ 50 opcodes
- ✅ 64 virtual registers
- ✅ 8KB code space
- ✅ 4KB data space
- ✅ Basic I/O support
- ✅ 10-20x performance penalty

#### Realistic Self-Hosted Compiler
- ✅ Subset language (TinyMinZ)
- ✅ Single-pass compilation
- ✅ File-based intermediate storage
- ✅ 128KB minimum RAM
- ❌ Full MinZ feature set
- ❌ Optimization passes
- ❌ Error recovery

### Performance Projections

```minz
// Compiling "Hello World" on Z80 @ 3.5MHz
// Native MinZ compiler (hypothetical): 5-10 seconds
// MIR VM interpreter: 30-60 seconds
// Self-hosted on 48KB: Not possible
// Self-hosted on 128KB: 60-120 seconds

// For comparison:
// Modern PC: 0.01 seconds
// Raspberry Pi: 0.1 seconds
```

## Part 5: The Pragmatic Approach

### Recommended Implementation Strategy

1. **Start with MIR VM** (Highly Feasible)
   - Implement in current MinZ
   - Test on real hardware
   - Use as development platform

2. **Create MIR Assembler** (Feasible)
   - Hand-write critical functions
   - Build standard library
   - Bootstrap basic tools

3. **Minimal Compiler Subset** (Challenging)
   - Target MIR, not native code
   - Focus on core features
   - Use VM for execution

4. **Gradual Enhancement** (Long-term)
   - Add features incrementally
   - Optimize hot paths
   - Consider banking/overlays

## Conclusion: Dreams vs Reality

### The Dream
A fully self-hosted MinZ compiler running on a 48KB ZX Spectrum, compiling itself in seconds, supporting all language features.

### The Reality
- **MIR VM**: ✅ Absolutely feasible, practical, useful
- **Minimal Compiler**: ⚠️ Possible with major restrictions  
- **Full Self-Hosting**: ❌ Not realistic on period hardware

### The Opportunity
The MIR VM represents the sweet spot - feasible to implement, useful for development, and a stepping stone toward greater ambitions. It would allow MinZ programs to run on any platform with a MIR VM implementation, provide a debugging environment, and serve as a bootstrap platform for compiler development.

### The Path Forward

```minz
// The journey of a thousand miles begins with a single step
fun main() -> void {
    let dream = "Self-hosted MinZ compiler";
    let reality = "MIR VM interpreter";
    let first_step = implement_mir_vm();
    
    print("Starting with: ", reality);
    print("Working toward: ", dream);
    print("First step: ", first_step);
    
    // Begin the journey
    while !goal_reached {
        take_next_step();
        celebrate_progress();
    }
}
```

The self-hosting dream may be distant, but the MIR VM is within reach. And sometimes, the journey itself teaches us more than the destination ever could.

---

*Next: Implementing a MIR VM in MinZ - A practical guide to building your own bytecode interpreter on Z80 hardware.*