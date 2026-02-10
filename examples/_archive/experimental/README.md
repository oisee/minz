# Experimental Examples

These examples require language features that are **not yet implemented** or are **experimental**.

## Feature Requirements

| Example | Feature Needed | Status |
|---------|---------------|--------|
| `zero_cost_interfaces.minz` | Generics `<T>` | ADR-002: Parked |
| `zero_cost_interfaces_test.minz` | Generics `<T>` | ADR-002: Parked |
| `test_iterator_overload.minz` | Generic type inference | Parked |
| `local_functions_test.minz` | `lamb` keyword | Not in grammar |
| `true_smc_lambdas.minz` | `lamb` keyword | Not in grammar |
| `asm_mir_functions.minz` | MIR statements | Incomplete |
| `mnist_complete.minz` | Struct closures | Not implemented |
| `mnist_simple.minz` | Struct closures | Not implemented |
| `test_enhanced_iterators.minz` | Advanced iterators | Incomplete |
| `minz_metaprogramming_showcase.minz` | `@minz` edge cases | Incomplete |
| `smc_optimization.minz` | Constant expressions | Bug |
| `smc_optimization_simple.minz` | Expression parsing | Bug |
| `stdlib_metafunction_test.minz` | `pad` builtin | Not implemented |
| `test_file_io_working.minz` | Lua constants | Bug |
| `test_lua_file_io.minz` | Lua constants | Bug |

## Moving to Working

Once a feature is implemented:
1. Test the example compiles: `./minzc/main examples/experimental/example.minz -o /tmp/out.a80`
2. If successful, move to `examples/`: `mv examples/experimental/example.minz examples/`
3. Update this README

## See Also

- [ADR-002: Generics Decision](../../docs/ADR-002-Generics-Decision.md)
- [STABILITY_ROADMAP.md](../../STABILITY_ROADMAP.md)
