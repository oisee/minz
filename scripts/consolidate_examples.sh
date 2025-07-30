#!/bin/bash
# Script to consolidate and organize MinZ examples to exactly 100

set -e

echo "=== Consolidating MinZ Examples to 100 ==="

# Create backup directory
mkdir -p examples_backup
echo "Backing up current examples..."
cp -r examples/* examples_backup/ 2>/dev/null || true

cd examples

# 1. Move important mnist examples to main folder
echo "Moving important mnist examples..."
mv mnist/mnist_editor_standalone.minz editor_standalone.minz 2>/dev/null || true
mv mnist/editor_working.minz editor_demo.minz 2>/dev/null || true
mv mnist/test_basic.minz basic_functions.minz 2>/dev/null || true
mv mnist/test_field_assignment.minz field_assignment.minz 2>/dev/null || true

# 2. Remove duplicate test files (keep only the best ones)
echo "Removing duplicate test files..."
# Keep primary test files, remove numbered variants
rm -f test1_*.minz test2_*.minz test3_*.minz test4_*.minz test5_*.minz
rm -f test_*2.minz  # Remove test_const2.minz etc
rm -f test_*_simple.minz test_*_simple2.minz  # Remove simple variants
rm -f test_*_debug.minz debug_*.minz  # Remove debug files
rm -f test_minimal.minz test_oneline.minz test_parse_only.minz
rm -f test_simple_*.minz single_*.minz

# 3. Remove duplicate editor files from mnist
rm -rf mnist/editor*.minz
rm -rf mnist/mnist_editor*.minz
rm -rf mnist/test_*.minz

# 4. Rename test files to more descriptive names
echo "Renaming test files to descriptive names..."
mv test_16bit_arithmetic.minz arithmetic_16bit.minz 2>/dev/null || true
mv test_array_syntax.minz arrays.minz 2>/dev/null || true
mv test_bit_structs.minz bit_fields.minz 2>/dev/null || true
mv test_complete_iterator.minz iterators.minz 2>/dev/null || true
mv test_global_init.minz global_variables.minz 2>/dev/null || true
mv test_implicit_return.minz implicit_returns.minz 2>/dev/null || true
mv test_inline_asm_expr.minz inline_assembly.minz 2>/dev/null || true
mv test_loop_indexed.minz loops_indexed.minz 2>/dev/null || true
mv test_physical_registers.minz register_allocation.minz 2>/dev/null || true
mv test_smc_recursive.minz smc_recursion.minz 2>/dev/null || true
mv test_stack_locals.minz stack_variables.minz 2>/dev/null || true
mv test_true_smc_calls.minz true_smc_demo.minz 2>/dev/null || true

# 5. Keep only essential test files
rm -f test_var_*.minz test_two_vars.minz test_three_vars.minz
rm -f test_direct_return.minz test_explicit_return.minz
rm -f test_old_array.minz test_multiline.minz
rm -f test_param_reuse*.minz test_patch.minz
rm -f test_scope.minz test_let_access.minz
rm -f test_u8_type.minz test_void_main.minz

# 6. Create new category-based examples if needed
cat > control_flow.minz << 'EOF'
// Control Flow Examples - if, while, loop
fun test_if(x: u8) -> u8 {
    if x > 10 {
        return x * 2;
    } else if x > 5 {
        return x + 10;
    } else {
        return x;
    }
}

fun test_while(n: u8) -> u8 {
    let sum: u8 = 0;
    let i: u8 = 0;
    while i < n {
        sum = sum + i;
        i = i + 1;
    }
    return sum;
}

fun main() {
    let result1 = test_if(15);
    let result2 = test_while(5);
}
EOF

cat > types_demo.minz << 'EOF'
// Type System Demonstration
fun test_u8(a: u8, b: u8) -> u8 {
    return a + b;
}

fun test_u16(x: u16, y: u16) -> u16 {
    return x * y;
}

fun test_i8(a: i8, b: i8) -> i8 {
    return a - b;
}

fun test_bool(x: u8) -> bool {
    return x > 10;
}

fun test_pointers(ptr: *u8, value: u8) -> void {
    *ptr = value;
}

fun main() {
    let byte_result = test_u8(10, 20);
    let word_result = test_u16(100, 200);
    let signed_result = test_i8(-5, 10);
    let bool_result = test_bool(15);
}
EOF

# 7. Clean up empty directories
rmdir mnist 2>/dev/null || true

# 8. List current count
echo -e "\n=== Current Example Count ==="
CURRENT_COUNT=$(find . -name "*.minz" -type f | wc -l)
echo "Current examples: $CURRENT_COUNT"

# 9. If we have more than 100, remove additional test files
if [ $CURRENT_COUNT -gt 100 ]; then
    echo "Removing excess test files..."
    # Remove in priority order
    rm -f test_assignment.minz test_cast.minz test_const.minz
    rm -f test_loop_*.minz test_string*.minz
    rm -f test_register*.minz test_asm.minz
fi

# 10. If we have less than 100, create additional useful examples
FINAL_COUNT=$(find . -name "*.minz" -type f | wc -l)
if [ $FINAL_COUNT -lt 100 ]; then
    NEEDED=$((100 - FINAL_COUNT))
    echo "Need $NEEDED more examples to reach 100..."
    
    # Add more @abi examples
    cat > abi_rom_integration.minz << 'EOF'
// ROM Integration via @abi
@abi("register: A=char")
@extern
fun rom_print_char(c: u8) -> void;

@abi("register: HL=str")
@extern
fun rom_print_string(str: *u8) -> void;

fun main() {
    rom_print_char(72);  // 'H'
    rom_print_char(105); // 'i'
    rom_print_string("Hello from MinZ!");
}
EOF

    cat > abi_hardware_drivers.minz << 'EOF'
// Hardware Driver Integration via @abi
@abi("register: A=value")
@extern
fun out_fe_port(value: u8) -> void;

@abi("register: A=reg, C=value")
@extern
fun ay_write(reg: u8, value: u8) -> void;

fun set_border(color: u8) {
    out_fe_port(color);
}

fun play_tone(frequency: u16) {
    let freq_lo = (frequency & 0xFF) as u8;
    let freq_hi = ((frequency >> 8) & 0x0F) as u8;
    ay_write(0, freq_lo);
    ay_write(1, freq_hi);
    ay_write(8, 15);
}

fun main() {
    set_border(2);      // Red border
    play_tone(440);     // A note
}
EOF
fi

# Final count and list
echo -e "\n=== Final Organization ==="
FINAL_COUNT=$(find . -name "*.minz" -type f | wc -l)
echo "Total examples: $FINAL_COUNT"

# Categorize examples
echo -e "\n=== Examples by Category ==="
echo "Core Language Features:"
ls -1 *.minz 2>/dev/null | grep -E "^(arithmetic|arrays|basic|control|enums|fibonacci|functions|global|structs|types)" | sort

echo -e "\n@abi & Assembly Integration:"
ls -1 *.minz 2>/dev/null | grep -E "^(abi_|asm_|inline_assembly)" | sort

echo -e "\nOptimizations & Advanced:"
ls -1 *.minz 2>/dev/null | grep -E "^(smc_|true_smc|tail_|shadow|register)" | sort

echo -e "\nApplications & Demos:"
ls -1 *.minz 2>/dev/null | grep -E "^(editor|game|mnist|zx_)" | sort

echo -e "\nOther Examples:"
ls -1 *.minz 2>/dev/null | grep -v -E "^(arithmetic|arrays|basic|control|enums|fibonacci|functions|global|structs|types|abi_|asm_|inline_assembly|smc_|true_smc|tail_|shadow|register|editor|game|mnist|zx_)" | sort

echo -e "\n=== Consolidation Complete! ==="