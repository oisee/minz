# tail_sum

## Files
- **tail_sum.minz** - Original MinZ source code
- **tail_sum.mir** - MinZ Intermediate Representation (shows optimization passes)
- **tail_sum.a80** - Generated Z80 assembly with SMC
- **tail_sum_opt.mir** - Optimized MIR showing tail recursion optimization
- **tail_sum_opt.a80** - Optimized assembly with tail recursion converted to jump

## Features Demonstrated
- Self-Modifying Code (SMC) functions
- Recursive functions
- **Tail Recursion Optimization** (with -O flag)

## SMC Parameters
```asm
sum_tail_param_n EQU sum_tail + 1
sum_tail_param_acc EQU sum_tail + 4
```

## Optimization Example

From tail_sum_opt.mir:
```
sum_tail_start:
...
jump sum_tail_start ; Tail recursion optimized
```

The tail recursive call is converted to a jump, eliminating call/return overhead!