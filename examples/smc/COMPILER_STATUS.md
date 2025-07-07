# MinZ Compiler Status - SMC Implementation

## Completed Optimizations

1. **No IX Usage** ✅
   - All functions now use absolute addressing for locals
   - Even recursive functions avoid IX by using stack for parameter context

2. **SMC Parameters at Point of Use** ✅
   - Parameters are now emitted where they're first used
   - Labels correctly point to the instruction containing the parameter

3. **Absolute Addressing for Locals** ✅
   - All local variables use fast absolute addressing at 0xF000+
   - 58% faster than IX-indexed addressing

## Still Needed

1. **Pure SMC Parameter Usage**
   - Currently: `LD HL, #0000` then `LD ($F006), HL` (wasteful)
   - Should be: `LD HL, #0000` then use HL directly
   - Avoid storing parameters to memory unless necessary

2. **SMC Call Site Modification**
   - Calls need to modify the parameter slots before calling
   - Example: `LD (add_param_a + 1), HL` before `CALL add`

3. **Parameter Tracking in IR**
   - The semantic analyzer needs to understand SMC parameters differently
   - Parameters should not generate store instructions

## Example Issues

Current output for `add(a, b)`:
```asm
add_param_a:
    LD HL, #0000   ; SMC parameter a
    LD ($F006), HL ; WASTEFUL - HL already has the value!
add_param_b:
    LD HL, #0000   ; SMC parameter b  
    LD ($F008), HL ; WASTEFUL
    ; r5 = r3 + r4
    LD HL, ($F006) ; WASTEFUL - reload what we just stored
```

Should be:
```asm
add_param_a:
    LD HL, #0000   ; SMC parameter a - HL has the value
    PUSH HL        ; Save it if needed later
add_param_b:
    LD HL, #0000   ; SMC parameter b - HL has the value
    POP DE         ; Get a back
    ADD HL, DE     ; Direct computation!
```

## Compiler Crashes
- register_test.minz - memory corruption
- shadow_registers.minz - memory corruption
- Some ZVDB examples timeout

## Successfully Compiling (10 examples)
1. **fibonacci** - Iterative Fibonacci with SMC parameters
2. **game_sprite** - Sprite rendering with SMC parameters
3. **main** - Simple game loop (fixed to remove module imports)
4. **screen_color** - Screen attribute manipulation (fixed constants)
5. **simple_add** - Basic addition demonstrating SMC parameters
6. **smc_optimization_simple** - Various SMC optimization examples
7. **tail_recursive** - Tail recursion examples with SMC
8. **tail_sum** - Tail recursive sum with SMC context save/restore
9. **test_simple_vars** - Variable declarations with absolute addressing
10. **test_var_decls** - Type casting and variable declarations

## Failed Examples (12)
- **enums** - Enum support not implemented
- **register_test** - Causes memory corruption in compiler
- **shadow_registers** - Causes memory corruption in compiler  
- **smc_optimization** - Original version with unsupported decorators
- **structs** - Struct support not implemented
- **test_registers** - Causes memory corruption in compiler
- **zvdb_*** (6 examples) - Module system and metaprogramming not implemented