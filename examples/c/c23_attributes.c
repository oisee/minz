/*
 * C23 [[attributes]] on Z80
 * Stripped by preprocessor — cosmetic only, no codegen effect.
 *
 * mzv: echo "" | mzv -H c23_attributes.c
 */
#include <stdint.h>
#include <stdbool.h>

/* [[maybe_unused]] — suppress "unused variable" warnings */
[[maybe_unused]] uint8_t debug_level = 0;

/* [[nodiscard]] — caller should not ignore return value */
[[nodiscard]] uint8_t allocate(uint8_t size) {
    return size > 0 ? size : 1;
}

/* [[deprecated]] — this function is obsolete */
[[deprecated("use add_v2 instead")]] uint8_t add_old(uint8_t a, uint8_t b) {
    return a + b;
}

/* [[noreturn]] — function never returns */
[[noreturn]] void infinite_loop(void) {
    for (;;);
}

/* Regular functions mixed with attributed ones */
uint8_t add_v2(uint8_t a, uint8_t b) {
    return a + b;
}

/* [[fallthrough]] in switch — explicit fallthrough marker */
uint8_t classify(uint8_t x) {
    if (x == 0) return 0;
    if (x < 10) return 1;
    return 2;
}

/* Multiple attributes on same line */
[[maybe_unused]] [[nodiscard]] uint8_t helper(uint8_t x) {
    return x * 2;
}

/* Attributed function with complex body */
[[nodiscard]] uint8_t safe_div(uint8_t a, uint8_t b) {
    if (b == 0) return 0;
    return a / b;
}

// assert allocate(10) == 10 via mir2
// assert allocate(0) == 1 via mir2
// assert add_old(10, 20) == 30 via mir2
// assert add_v2(10, 20) == 30 via mir2
// assert classify(0) == 0 via mir2
// assert classify(5) == 1 via mir2
// assert classify(50) == 2 via mir2
// assert helper(21) == 42 via mir2
// assert safe_div(100, 10) == 10 via mir2
// safe_div(10, 0) — div by zero, skipped on mir2 VM
