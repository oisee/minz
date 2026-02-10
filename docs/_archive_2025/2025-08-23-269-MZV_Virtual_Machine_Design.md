# MZV: MinZ Virtual Machine - The MIR Executor

## Revolutionary Concept
**MZV is NOT a visualization tool - it's a Virtual Machine for MIR!**

MIR (MinZ Intermediate Representation) is unconstrained by physical CPU limitations:
- Unlimited registers (r0, r1, r2... r999)
- Any operations we want
- Perfect for compile-time execution
- No Z80 restrictions!

## Why This Is Brilliant

### MIR is Already Simple!
```mir
; MIR example
r0 = 10
r1 = 20  
r2 = add r0, r1
r3 = mul r2, r2
ret r3
```

### No CPU Limitations
- **Z80**: 7 registers, complex addressing
- **MIR**: Unlimited registers, simple operations
- **Result**: MUCH easier to execute!

## MZV Architecture

```go
type MZV struct {
    registers map[string]interface{}  // r0, r1, r2... unlimited!
    memory    map[uint32]byte        // Virtual memory
    stack     []interface{}           // Unlimited stack
    pc        int                     // Program counter
}

func (vm *MZV) Execute(mir []MIRInstruction) interface{} {
    for vm.pc < len(mir) {
        inst := mir[vm.pc]
        switch inst.Op {
        case "mov":
            vm.registers[inst.Dest] = vm.registers[inst.Src]
        case "add":
            vm.registers[inst.Dest] = vm.getVal(inst.Arg1) + vm.getVal(inst.Arg2)
        case "mul":
            vm.registers[inst.Dest] = vm.getVal(inst.Arg1) * vm.getVal(inst.Arg2)
        case "call":
            result := vm.ExecuteFunction(inst.Func)
            vm.registers[inst.Dest] = result
        case "ret":
            return vm.getVal(inst.Arg1)
        }
        vm.pc++
    }
}
```

## MIR Instruction Set (Simple!)

### Data Movement
```mir
r1 = 42           ; Load immediate
r2 = r1           ; Register copy
r3 = [r1]         ; Load from memory
[r1] = r2         ; Store to memory
```

### Arithmetic (Unlimited Precision!)
```mir
r1 = add r2, r3   ; No overflow!
r1 = mul r2, 1000000  ; Big numbers!
r1 = div r2, r3   ; Real division!
r1 = mod r2, r3   ; Modulo
```

### Control Flow
```mir
jmp label
jz r1, label      ; Jump if zero
jnz r1, label     ; Jump if not zero
call function
ret r1
```

### Advanced Operations (Not Possible on Z80!)
```mir
r1 = concat s1, s2     ; String operations!
r2 = len str           ; String length
r3 = slice arr, 0, 10  ; Array slicing
r4 = map func, list    ; Functional operations!
```

## Use Cases

### 1. Compile-Time Execution (CTIE)
```minz
@minz[[[
    let table = generate_sine_table(256);
    @emit_table(table);
]]]
```

Executes in MZV:
```mir
r0 = 256
r1 = call generate_sine_table, r0
; r1 now contains full array!
emit_table r1
```

### 2. Constant Folding
```minz
const X = fibonacci(20);  // Computed at compile time!
```

### 3. Macro Expansion
```minz
@define("REPEAT", count, body) {
    for i in 0..count {
        @emit(body);
    }
}
```

### 4. Type Checking
MZV can run type inference algorithms during compilation!

## Why MZV is Superior to Z80 Emulation for CTIE

| Aspect | Z80 Emulation | MZV (MIR VM) |
|--------|--------------|--------------|
| Registers | 7 registers | Unlimited |
| Memory | 64KB limit | Unlimited |
| Data Types | Bytes only | Any type (strings, arrays, objects) |
| Operations | CPU instructions | Any operation we want |
| Speed | Cycle-accurate simulation | Direct execution |
| Complexity | Must handle flags, addressing modes | Simple register machine |

## Implementation Simplicity

### MIR is Already a Simple VM Language!
```mir
; Fibonacci in MIR
func fib(n):
    r0 = cmp n, 2
    jl r0, base_case
    r1 = sub n, 1
    r2 = call fib, r1
    r3 = sub n, 2
    r4 = call fib, r3
    r5 = add r2, r4
    ret r5
base_case:
    ret n
```

### MZV Executes It Directly!
No compilation to Z80 needed - just interpret the MIR!

## Integration with MinZ Compiler

```go
// During compilation
case "@minz":
    mir := CompileToMIR(metafunctionAST)
    vm := mzv.New()
    result := vm.Execute(mir)
    // Use result in code generation!
```

## Advantages

1. **No CPU Restrictions** - Design MIR however we want
2. **Simple Implementation** - MIR is already VM-like
3. **Powerful Operations** - Strings, arrays, maps at compile time
4. **Fast** - No CPU simulation overhead
5. **Debuggable** - Can trace MIR execution easily

## Success Metrics
- Execute any MIR code at compile time
- Support operations impossible on Z80
- Enable true metaprogramming
- Simplify CTIE implementation

## Revolutionary Impact

MZV turns MIR from just an intermediate representation into a **powerful compile-time execution environment**. This enables:

- True metaprogramming
- Compile-time computation without limits
- Type checking and inference
- Macro expansion
- Code generation

**"MIR is not just intermediate - it's a superpower!"**