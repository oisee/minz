/*
 * C99 stdbool.h — bool/true/false on Z80
 *
 * mzv: echo "" | mzv -H c99_stdbool.c
 * CP/M: mz c99_stdbool.c --target=cpm -o out.a80
 */
#include <stdbool.h>
#include <stdint.h>

bool is_even(uint8_t n) {
    return (n & 1) == 0;
}

bool is_positive(int16_t x) {
    return x > 0;
}

uint8_t bool_to_int(bool b) {
    return b ? 42 : 0;
}

bool both_true(bool a, bool b) {
    return a && b;
}

bool either_true(bool a, bool b) {
    return a || b;
}

bool negate(bool b) {
    return !b;
}

// assert is_even(4) == 1 via mir2
// assert is_even(7) == 0 via mir2
// assert is_positive(10) == 1 via mir2
// assert is_positive(0) == 0 via mir2
// assert bool_to_int(1) == 42 via mir2
// assert bool_to_int(0) == 0 via mir2
// assert both_true(1, 1) == 1 via mir2
// assert both_true(1, 0) == 0 via mir2
// assert either_true(0, 1) == 1 via mir2
// assert either_true(0, 0) == 0 via mir2
// assert negate(0) == 1 via mir2
// assert negate(1) == 0 via mir2
