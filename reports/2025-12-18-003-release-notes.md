# Release Notes v0.14.1 - Zero-Cost Interface Methods

**Date:** 2025-12-18
**Success Rate:** 81.6% → 82.7% (72/87 examples)

## Highlights

This release fixes **zero-cost interface method calls** - the `obj.method()` syntax now correctly resolves to the implementing method, enabling true zero-cost abstractions on Z80!

```minz
let circle = Circle { radius: 5 };
circle.draw();    // Compiles to: CALL Circle_draw - NO vtables!
circle.scale(2);  // Multi-parameter methods work too!
```

## What's New

### Method Call Resolution (P3 #13)
- Fixed `obj.method()` syntax for interface implementations
- Added mangled name lookup (`Circle.draw$Circle`)
- Support for multi-parameter methods via overload sets
- Full type path resolution for module-qualified types

### Function Type Parsing (P3 #12)
- Added `fn(T) -> R` function type syntax
- New `function_type_params` grammar rule
- Enables higher-order function type annotations

### Bug Fixes
- Fixed struct fields with newline separators (grammar)
- Fixed S-expression parser for function types

## Examples Now Working
- `zero_cost_test.minz` - Complete zero-cost interfaces demo
- `zero_cost_interfaces_concept.minz` - Interface concepts
- Various struct-based examples

## Remaining Failures (15 examples)

| Category | Count | Examples |
|----------|-------|----------|
| Generics (`<T>`) | 3 | zero_cost_interfaces, zero_cost_interfaces_test, test_iterator_overload |
| Local Functions | 2 | local_functions_test, true_smc_lambdas |
| MIR/ASM | 1 | asm_mir_functions |
| Missing Builtins | 3 | smc_optimization*, stdlib_metafunction_test |
| Iterator Ops | 1 | test_enhanced_iterators |
| File I/O Constants | 2 | test_file_io_working, test_lua_file_io |
| Struct Closures | 2 | mnist_complete, mnist_simple |
| Metaprogramming | 1 | minz_metaprogramming_showcase |

## Technical Details

### Method Resolution Algorithm
1. Try direct qualified lookup (`circle.draw`)
2. Look up variable type (`circle` → `Circle`)
3. Try mangled names: `Circle.draw`, `Circle_draw`
4. Try full path: `.module.Circle.draw$Circle`
5. Check overload sets for multi-param methods

### Commits
- `fe48319` feat: Implement method call resolution for zero-cost interfaces
- `ffa83c0` feat: Add function type parsing support
- `d4524a8` fix: Allow newline-separated struct fields in grammar

## Next Steps (P4 Priorities)
1. **Generics** - Implement `<T>` type parameters (ADR-002)
2. **Local Functions** - Add `lamb` keyword support
3. **MIR Statements** - Complete inline assembly support
