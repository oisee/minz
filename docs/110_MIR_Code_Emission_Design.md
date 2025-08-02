# MIR Code Emission in MinZ

## Overview

This document proposes adding MIR (Machine-Independent Representation) code emission capabilities to MinZ, allowing developers to write optimization passes and low-level code generation directly in MinZ using metafunctions.

## Motivation

1. **Machine Independence**: Abstract away Z80-specific details for portability
2. **Metaprogramming Power**: Leverage Lua metafunctions for sophisticated optimizations
3. **User-Defined Optimizations**: Allow developers to write custom optimization passes
4. **Future Portability**: Enable targeting 6502, 68000, and other architectures

## Proposed Syntax

### Basic MIR Block

```minz
mir {
    r1 = load_const 42
    r2 = load_var "x"
    r3 = add r1, r2
    store_var "result", r3
    ret r3
}
```

### With Metafunctions

```minz
@lua {
    function emit_inc_sequence(reg, count)
        for i = 1, count do
            emit_mir("inc " .. reg)
        end
    end
}

fun increment_by(x: u8, n: u8) -> u8 {
    mir {
        r1 = load_param 0    // x
        r2 = load_param 1    // n
        @lua { emit_inc_sequence("r1", n) }
        ret r1
    }
}
```

### Abstract MIR (AMIR)

For truly machine-independent code:

```minz
amir {
    // Abstract operations that map to different instructions
    val = atomic_increment(counter)
    
    // Platform-specific selection at compile time
    @platform("z80") {
        // Maps to INC (HL)
    }
    @platform("6502") {
        // Maps to INC $addr
    }
}
```

## MIR Instruction Set

### Data Movement
- `load_const <reg>, <value>` - Load immediate value
- `load_var <reg>, <name>` - Load from variable
- `store_var <name>, <reg>` - Store to variable
- `load_param <reg>, <index>` - Load function parameter
- `move <dest>, <src>` - Register to register

### Arithmetic
- `add <dest>, <src1>, <src2>` - Addition
- `sub <dest>, <src1>, <src2>` - Subtraction
- `mul <dest>, <src1>, <src2>` - Multiplication
- `inc <reg>` - Increment
- `dec <reg>` - Decrement

### Control Flow
- `label <name>` - Define label
- `jump <label>` - Unconditional jump
- `jump_if <reg>, <label>` - Conditional jump
- `call <func>` - Function call
- `ret [<reg>]` - Return

### Memory
- `load_mem <reg>, <addr>` - Load from memory
- `store_mem <addr>, <reg>` - Store to memory
- `load_indirect <reg>, <ptr>` - Load through pointer
- `store_indirect <ptr>, <reg>` - Store through pointer

### Special
- `smc_patch <anchor>, <value>` - SMC patching
- `push <reg>` - Push to stack
- `pop <reg>` - Pop from stack

## Integration with Optimization Pipeline

### Pre-MIR Optimizations

```minz
// User-defined pre-MIR optimization
@optimize("pre_mir")
fun fold_increments(ast: ASTNode) -> ASTNode {
    // Pattern: x = x + 2
    if ast.is_assignment() && ast.rhs.is_add() {
        if ast.lhs == ast.rhs.left && ast.rhs.right.is_const() {
            return mir {
                r1 = load_var @{ast.lhs.name}
                @lua { 
                    local n = ast.rhs.right.value
                    if should_use_inc_dec(r1, n) then
                        emit_inc_sequence("r1", n)
                    else
                        emit_mir("r2 = load_const " .. n)
                        emit_mir("r1 = add r1, r2")
                    end
                }
                store_var @{ast.lhs.name}, r1
            }
        }
    }
    return ast
}
```

### MIR-Level Optimizations

```minz
@optimize("mir")
fun peephole_optimizer(instructions: [MIRInstruction]) -> [MIRInstruction] {
    mir {
        @lua {
            -- Pattern matching on MIR instructions
            for i = 1, #instructions - 1 do
                local inst1 = instructions[i]
                local inst2 = instructions[i + 1]
                
                -- Redundant load/store elimination
                if inst1.op == "store_var" and 
                   inst2.op == "load_var" and
                   inst1.var == inst2.var and
                   inst1.reg == inst2.reg then
                    -- Eliminate the load
                    instructions[i + 1] = nil
                end
            end
        }
    }
}
```

## Use Cases

### 1. Custom Calling Conventions

```minz
@calling_convention("fastcall")
mir {
    // First 3 params in registers
    r1 = load_reg A
    r2 = load_reg B  
    r3 = load_reg C
    // Rest on stack
    r4 = load_stack 0
}
```

### 2. Inline Assembly Alternative

Instead of:
```minz
@asm {
    LD A, 42
    ADD A, B
}
```

Use:
```minz
mir {
    r1 = load_const 42
    r2 = load_reg B
    r1 = add r1, r2
    store_reg A, r1
}
```

### 3. Platform-Specific Optimizations

```minz
@lua {
    function emit_optimal_multiply(dest, src, n)
        if is_power_of_two(n) then
            local shifts = log2(n)
            emit_mir(dest .. " = " .. src)
            for i = 1, shifts do
                emit_mir(dest .. " = shl " .. dest .. ", 1")
            end
        else
            emit_mir("r_temp = load_const " .. n)
            emit_mir(dest .. " = mul " .. src .. ", r_temp")
        end
    end
}
```

### 4. Advanced SMC Patterns

```minz
mir {
    // Self-modifying loop
    label "loop_start"
    r1 = load_smc_param "counter"
    dec r1
    smc_patch "counter", r1
    jump_if_not_zero r1, "loop_start"
}
```

## Benefits

1. **Abstraction**: Write optimizations without Z80 assembly knowledge
2. **Portability**: Same MIR code can target different architectures
3. **Power**: Full Lua metaprogramming for code generation
4. **Debugging**: Easier to debug MIR than assembly
5. **Education**: Learn compiler internals by writing MIR

## Implementation Phases

### Phase 1: Basic MIR Emission
- Parse `mir { }` blocks
- Generate MIR instructions
- Feed into existing optimizer

### Phase 2: Metafunction Integration
- Allow Lua code in MIR blocks
- Dynamic MIR generation
- Pattern matching helpers

### Phase 3: Optimization Hooks
- `@optimize` decorator
- User-defined passes
- Pass ordering control

### Phase 4: Abstract MIR
- Platform-independent operations
- Architecture selection
- Cross-platform libraries

## Example: Complete INC/DEC Optimizer in MinZ

```minz
@optimize("pre_mir", priority: 100)
fun optimize_increments(ast: ASTNode) -> ASTNode {
    @pattern("$var = $var + $const")
    fun handle_increment(var: Identifier, const: Number) -> MIR {
        mir {
            r1 = load_var @{var.name}
            @lua {
                local n = const.value
                local reg_type = infer_register_type(var)
                
                if reg_type == "A" and n == 1 then
                    emit_mir("inc r1")
                elseif reg_type ~= "A" and n <= 3 then
                    for i = 1, n do
                        emit_mir("inc r1")
                    end
                else
                    emit_mir("r2 = load_const " .. n)
                    emit_mir("r1 = add r1, r2")
                end
            }
            store_var @{var.name}, r1
        }
    }
    
    return ast.transform(handle_increment)
}
```

## Conclusion

MIR code emission in MinZ would provide a powerful abstraction layer for optimization and code generation. Combined with metafunctions, it enables sophisticated compile-time transformations while maintaining the language's philosophy of zero-cost abstractions. This feature would make MinZ not just a language for Z80, but a platform for exploring compiler optimizations across multiple architectures.