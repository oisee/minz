# MinZ Compilation Results Report
Date: Mon 28 Jul 2025 01:32:52 IST
Compiler: minzc

Testing: arithmetic_demo.minz
  ✅ SUCCESS

Testing: const_only.minz
  ✅ SUCCESS

Testing: debug_bit_field.minz
  ❌ SEMANTIC ERROR
       1. error in function main: undefined identifier: TestBits (analyzeIdentifier)

Testing: debug_const.minz
  ❌ SEMANTIC ERROR
       1. error in function main: undefined identifier: X (analyzeIdentifier)

Testing: debug_scope.minz
  ❌ SEMANTIC ERROR
       1. error in function test: cannot use u8 as value

Testing: debug_tokenize.minz
  💥 PANIC
     panic: runtime error: invalid memory address or nil pointer dereference

Testing: enums.minz
  💥 PANIC
     panic: runtime error: invalid memory address or nil pointer dereference

Testing: fibonacci.minz
  💥 PANIC
     panic: runtime error: invalid memory address or nil pointer dereference

Testing: game_sprite.minz
  💥 PANIC
     panic: runtime error: invalid memory address or nil pointer dereference

Testing: lua_assets.minz
  ❌ PARSE ERROR
     Error: parse error: expected source_file, got ERROR

Testing: lua_metaprogramming.minz
  ❌ SEMANTIC ERROR
       1. error in function main: variable game_state must have either a type or an initial value

Testing: lua_sine_table.minz
  ❌ SEMANTIC ERROR
       1. error in function rotate_point: cannot use i16 as value

Testing: main.minz
  💥 PANIC
     panic: runtime error: invalid memory address or nil pointer dereference

Testing: metaprogramming.minz
  💥 PANIC
     panic: runtime error: invalid memory address or nil pointer dereference

Testing: mnist_complete.minz
  💥 PANIC
     panic: runtime error: invalid memory address or nil pointer dereference

Testing: mnist_simple.minz
  ✅ SUCCESS

Testing: register_test.minz
  ❌ SEMANTIC ERROR
       1. error in function simple_add: undefined identifier: a (analyzeIdentifier)

Testing: screen_color.minz
  ❌ SEMANTIC ERROR
       1. error in function fill_screen: undefined identifier: ink (analyzeIdentifier)

Testing: shadow_registers.minz
  💥 PANIC
     panic: runtime error: invalid memory address or nil pointer dereference

Testing: simple_add.minz
  ❌ SEMANTIC ERROR
       1. error in function add: undefined identifier: a (analyzeIdentifier)

Testing: simple_test.minz
  ✅ SUCCESS

Testing: simple_true_smc.minz
  ❌ SEMANTIC ERROR
       1. error in function add: undefined identifier: x (analyzeIdentifier)

Testing: smc_optimization_simple.minz
  💥 PANIC
     panic: runtime error: invalid memory address or nil pointer dereference

Testing: smc_optimization.minz
  💥 PANIC
     panic: runtime error: invalid memory address or nil pointer dereference

Testing: structs.minz
  💥 PANIC
     panic: runtime error: invalid memory address or nil pointer dereference

Testing: tail_recursive.minz
  💥 PANIC
     panic: runtime error: invalid memory address or nil pointer dereference

Testing: tail_sum.minz
  💥 PANIC
     panic: runtime error: invalid memory address or nil pointer dereference

Testing: test_16bit_arithmetic.minz
  ❌ SEMANTIC ERROR
       1. error in function test_16bit_ops: undefined identifier: b (analyzeIdentifier)

Testing: test_16bit_smc.minz
  ❌ SEMANTIC ERROR
       1. error in function multiply_add: undefined identifier: a (analyzeIdentifier)

Testing: test_abi_comparison.minz
  💥 PANIC
     panic: runtime error: invalid memory address or nil pointer dereference

Testing: test_all_features.minz
  💥 PANIC
     panic: runtime error: invalid memory address or nil pointer dereference

Testing: test_array_access.minz
  💥 PANIC
     panic: runtime error: invalid memory address or nil pointer dereference

Testing: test_array_syntax.minz
  💥 PANIC
     panic: runtime error: invalid memory address or nil pointer dereference

Testing: test_asm_final.minz
  ✅ SUCCESS

Testing: test_asm_simple.minz
  💥 PANIC
     panic: runtime error: invalid memory address or nil pointer dereference

Testing: test_asm.minz
  💥 PANIC
     panic: runtime error: invalid memory address or nil pointer dereference

Testing: test_assignment.minz
  ✅ SUCCESS

Testing: test_bit_field_access.minz
  ❌ SEMANTIC ERROR
       1. error in function main: undefined identifier: ScreenAttr (analyzeIdentifier)

Testing: test_bit_field_proper.minz
  ❌ SEMANTIC ERROR
       1. error in function main: undefined identifier: Nibbles (analyzeIdentifier)

Testing: test_bit_structs.minz
  ❌ SEMANTIC ERROR
       1. error in function set_screen_attr: unsupported expression type: <nil>

Testing: test_cast.minz
  ❌ SEMANTIC ERROR
       1. error in function main: undefined identifier: Nibbles (analyzeIdentifier)

Testing: test_complete_iterator.minz
  💥 PANIC
     panic: runtime error: invalid memory address or nil pointer dereference

Testing: test_const_debug.minz
  ❌ SEMANTIC ERROR
       1. error in function main: undefined identifier: MAGIC (analyzeIdentifier)

Testing: test_const_simple_fix.minz
  ❌ SEMANTIC ERROR
       1. error in function main: undefined identifier: X (analyzeIdentifier)

Testing: test_const.minz
  ❌ SEMANTIC ERROR
       1. error in function main: undefined identifier: MAGIC (analyzeIdentifier)

Testing: test_const2.minz
  ❌ SEMANTIC ERROR
       1. error in function main: undefined identifier: MAGIC (analyzeIdentifier)

Testing: test_direct_return.minz
  ❌ SEMANTIC ERROR
       1. error in function calculate_once: unsupported expression type: <nil>

Testing: test_global_init.minz
  ✅ SUCCESS

Testing: test_implicit_return.minz
  ✅ SUCCESS

Testing: test_imports_debug.minz
  ✅ SUCCESS

Testing: test_imports.minz
  ❌ SEMANTIC ERROR
       1. error in function main: indirect function calls not yet supported

Testing: test_inline_asm_expr.minz
  ✅ SUCCESS

Testing: test_let_access.minz
  ✅ SUCCESS

Testing: test_loop_debug.minz
  💥 PANIC
     panic: runtime error: invalid memory address or nil pointer dereference

Testing: test_loop_indexed.minz
  💥 PANIC
     panic: runtime error: invalid memory address or nil pointer dereference

Testing: test_loop_into.minz
  💥 PANIC
     panic: runtime error: invalid memory address or nil pointer dereference

Testing: test_loop_minimal.minz
  💥 PANIC
     panic: runtime error: invalid memory address or nil pointer dereference

Testing: test_loop_no_init.minz
  💥 PANIC
     panic: runtime error: invalid memory address or nil pointer dereference

Testing: test_loop_ref.minz
  💥 PANIC
     panic: runtime error: invalid memory address or nil pointer dereference

Testing: test_loop_simple.minz
  💥 PANIC
     panic: runtime error: invalid memory address or nil pointer dereference

Testing: test_loop_simple2.minz
  💥 PANIC
     panic: runtime error: invalid memory address or nil pointer dereference

Testing: test_minimal.minz
  ✅ SUCCESS

Testing: test_multiline.minz
  ✅ SUCCESS

Testing: test_old_array.minz
  ✅ SUCCESS

Testing: test_oneline.minz
  ❌ SEMANTIC ERROR
       1. error in function main: undefined identifier: Nibbles (analyzeIdentifier)

Testing: test_param_reuse_simple.minz
  ❌ SEMANTIC ERROR
       1. error in function calculate: undefined identifier: x (analyzeIdentifier)

Testing: test_param_reuse.minz
  ❌ SEMANTIC ERROR
       1. error in function calculate: undefined identifier: x (analyzeIdentifier)

Testing: test_parse_only.minz
  ✅ SUCCESS

Testing: test_patch.minz
  ❌ SEMANTIC ERROR
       1. error in function add: undefined identifier: x (analyzeIdentifier)

Testing: test_physical_registers.minz
  ✅ SUCCESS

Testing: test_register_params.minz
  💥 PANIC
     panic: runtime error: invalid memory address or nil pointer dereference

Testing: test_registers.minz
  💥 PANIC
     panic: runtime error: invalid memory address or nil pointer dereference

Testing: test_scope.minz
  ✅ SUCCESS

Testing: test_simple_array.minz
  ✅ SUCCESS

Testing: test_simple_call.minz
  ❌ SEMANTIC ERROR
       1. error in function add: undefined identifier: a (analyzeIdentifier)

Testing: test_simple_scope.minz
  ❌ SEMANTIC ERROR
       1. error in function test_variables: cannot use u8 as value

Testing: test_simple_stack.minz
  ✅ SUCCESS

Testing: test_simple_vars.minz
  ✅ SUCCESS

Testing: test_smc_patching.minz
  ❌ SEMANTIC ERROR
       1. error in function add_nums: undefined identifier: x (analyzeIdentifier)

Testing: test_smc_recursive.minz
  💥 PANIC
     panic: runtime error: invalid memory address or nil pointer dereference

Testing: test_stack_locals.minz
  💥 PANIC
     panic: runtime error: invalid memory address or nil pointer dereference

Testing: test_string_lengths.minz
  ❌ SEMANTIC ERROR
       1. error in function main: variable short must have either a type or an initial value

Testing: test_strings.minz
  💥 PANIC
     panic: runtime error: invalid memory address or nil pointer dereference

Testing: test_struct.minz
  ❌ SEMANTIC ERROR
       1. error in function test_struct: variable p must have either a type or an initial value

Testing: test_three_vars.minz
  ✅ SUCCESS

Testing: test_true_smc_call.minz
  ❌ SEMANTIC ERROR
       1. error in function add: undefined identifier: x (analyzeIdentifier)

Testing: test_true_smc_calls.minz
  ❌ SEMANTIC ERROR
       1. error in function add: undefined identifier: x (analyzeIdentifier)

Testing: test_true_smc_simple.minz
  ❌ SEMANTIC ERROR
       1. error in function identity: undefined identifier: x (analyzeIdentifier)

Testing: test_two_vars.minz
  ✅ SUCCESS

Testing: test_u8_type.minz
  ✅ SUCCESS

Testing: test_var_decls.minz
  ❌ SEMANTIC ERROR
       1. error in function test_var_declarations: unsupported expression type: <nil>

Testing: test_var_lookup.minz
  ✅ SUCCESS

Testing: test1_basic_8bit.minz
  ❌ SEMANTIC ERROR
       1. error in function add: undefined identifier: x (analyzeIdentifier)

Testing: test2_16bit_params.minz
  ❌ SEMANTIC ERROR
       1. error in function multiply_add: undefined identifier: a (analyzeIdentifier)

Testing: test3_param_reuse.minz
  ❌ SEMANTIC ERROR
       1. error in function process: undefined identifier: value (analyzeIdentifier)

Testing: test4_mixed_types.minz
  ❌ SEMANTIC ERROR
       1. error in function calculate: undefined identifier: x (analyzeIdentifier)

Testing: test5_recursive.minz
  💥 PANIC
     panic: runtime error: invalid memory address or nil pointer dereference

Testing: true_smc_test.minz
  ❌ SEMANTIC ERROR
       1. error in function main: undefined function: print

Testing: working_demo.minz
  ✅ SUCCESS

Testing: zvdb_code_search.minz
  💥 PANIC
     panic: runtime error: invalid memory address or nil pointer dereference

Testing: zvdb_demo.minz
  💥 PANIC
     panic: runtime error: invalid memory address or nil pointer dereference

Testing: zvdb_minimal.minz
  💥 PANIC
     panic: runtime error: invalid memory address or nil pointer dereference

Testing: zvdb_optimized.minz
  💥 PANIC
     panic: runtime error: invalid memory address or nil pointer dereference

Testing: zvdb_paged.minz
  💥 PANIC
     panic: runtime error: invalid memory address or nil pointer dereference

Testing: zvdb_scorpion_optimized.minz
  💥 PANIC
     panic: runtime error: invalid memory address or nil pointer dereference

## Summary

Total files:      105
- ✅ Success: 25
- ❌ Parse errors: 1
- ❌ Semantic errors: 39
- 💥 Panics: 40
- ❓ Other errors: 0

## Successful Compilations (25)
- arithmetic_demo.minz
- const_only.minz
- mnist_simple.minz
- simple_test.minz
- test_asm_final.minz
- test_assignment.minz
- test_global_init.minz
- test_implicit_return.minz
- test_imports_debug.minz
- test_inline_asm_expr.minz
- test_let_access.minz
- test_minimal.minz
- test_multiline.minz
- test_old_array.minz
- test_parse_only.minz
- test_physical_registers.minz
- test_scope.minz
- test_simple_array.minz
- test_simple_stack.minz
- test_simple_vars.minz
- test_three_vars.minz
- test_two_vars.minz
- test_u8_type.minz
- test_var_lookup.minz
- working_demo.minz

## Parse Errors (1)
- lua_assets.minz

## Semantic Errors (39)
- debug_bit_field.minz
- debug_const.minz
- debug_scope.minz
- lua_metaprogramming.minz
- lua_sine_table.minz
- register_test.minz
- screen_color.minz
- simple_add.minz
- simple_true_smc.minz
- test_16bit_arithmetic.minz
- test_16bit_smc.minz
- test_bit_field_access.minz
- test_bit_field_proper.minz
- test_bit_structs.minz
- test_cast.minz
- test_const_debug.minz
- test_const_simple_fix.minz
- test_const.minz
- test_const2.minz
- test_direct_return.minz
- test_imports.minz
- test_oneline.minz
- test_param_reuse_simple.minz
- test_param_reuse.minz
- test_patch.minz
- test_simple_call.minz
- test_simple_scope.minz
- test_smc_patching.minz
- test_string_lengths.minz
- test_struct.minz
- test_true_smc_call.minz
- test_true_smc_calls.minz
- test_true_smc_simple.minz
- test_var_decls.minz
- test1_basic_8bit.minz
- test2_16bit_params.minz
- test3_param_reuse.minz
- test4_mixed_types.minz
- true_smc_test.minz

## Panic Errors (40)
- debug_tokenize.minz
- enums.minz
- fibonacci.minz
- game_sprite.minz
- main.minz
- metaprogramming.minz
- mnist_complete.minz
- shadow_registers.minz
- smc_optimization_simple.minz
- smc_optimization.minz
- structs.minz
- tail_recursive.minz
- tail_sum.minz
- test_abi_comparison.minz
- test_all_features.minz
- test_array_access.minz
- test_array_syntax.minz
- test_asm_simple.minz
- test_asm.minz
- test_complete_iterator.minz
- test_loop_debug.minz
- test_loop_indexed.minz
- test_loop_into.minz
- test_loop_minimal.minz
- test_loop_no_init.minz
- test_loop_ref.minz
- test_loop_simple.minz
- test_loop_simple2.minz
- test_register_params.minz
- test_registers.minz
- test_smc_recursive.minz
- test_stack_locals.minz
- test_strings.minz
- test5_recursive.minz
- zvdb_code_search.minz
- zvdb_demo.minz
- zvdb_minimal.minz
- zvdb_optimized.minz
- zvdb_paged.minz
- zvdb_scorpion_optimized.minz

## Other Errors (0)
