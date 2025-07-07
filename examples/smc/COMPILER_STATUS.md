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
- fibonacci
- game_sprite  
- main
- screen_color
- simple_add
- smc_optimization_simple
- tail_recursive
- tail_sum
- test_simple_vars
- test_var_decls