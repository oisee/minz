# Mini Plan - February 2026

## Final: 68/70 examples (97%)

## Completed Today
- [x] `conway` - Fixed `for` → `while true`
- [x] `error_propagation_demo` - Fixed @error metafunction
- [x] `zero_cost_test` - Fixed lambda `=> Type` syntax
- [x] `math_functions` - Fixed abs(u8) overload
- [x] `performance_tricks` - Fixed `@extern` → `extern`
- [x] `nested_loops` - Fixed pointer dereference precedence
- [x] `pointer_arithmetic` - Fixed pointer dereference precedence
- [x] `cpm_hello` - Fixed module import path resolution + pub functions
- [x] `mos_hello` - Created Agon Light 2 hello world example

## Remaining (2 examples)

### Complex Shaders (need glsl stdlib)
- [ ] `lunar_scene` - 21+ errors
- [ ] `sphere_raymarch` - 17+ errors

## Features Implemented
- [x] `@error(EnumValue)` metafunction with enum lookup
- [x] Lambda `=> Type` syntax in parser
- [x] `asm fun` syntax added to parser
- [x] Pointer arithmetic type preservation
- [x] bdos.minz converted to asm blocks + all functions exported as `pub`
- [x] Module import short alias (e.g., `import cpm.bdos` → `bdos.putchar()`)
- [x] Flexible stdlib path search (MINZ_STDLIB env, executable-relative)
- [x] Fixed duplicate function emission in module imports
- [x] `extern fun name() at addr;` syntax for RST optimization

## Platform Support
- [x] CP/M: `cpm_hello.minz` compiles and assembles to COM file
- [x] Agon Light 2: `mos_hello.minz` compiles with RST 0x10 optimization
- [x] Tested with built-in MZE emulator (CP/M mode)
- [x] Copied to fab-agon-emulator SD card for testing

## Session Progress
- Start: 59/69 (85%)
- End: 68/70 (97%)
- Gained: +9 examples (+12%)
