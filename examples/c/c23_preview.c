/*
 * C23 preview features on Z80
 * bool/true/false as keywords (no #include needed)
 *
 * mzv: echo "" | mzv -H c23_preview.c
 * CP/M: mz c23_preview.c --target=cpm -o out.a80
 */
#include <stdint.h>

/* C23: bool, true, false are keywords — no stdbool.h needed */
bool is_zero(uint8_t n) {
    return n == 0;
}

bool xor_bool(bool a, bool b) {
    return (a && !b) || (!a && b);
}

/* C23: typeof (if parser supports it) */
/* typeof(uint8_t) x = 42; */

/* Ternary chains */
uint8_t clamp(uint8_t val, uint8_t lo, uint8_t hi) {
    return val < lo ? lo : val > hi ? hi : val;
}

/* Nested ternary */
uint8_t sign(int16_t x) {
    return x > 0 ? 1 : x < 0 ? 2 : 0;
}

// assert is_zero(0) == 1 via mir2
// assert is_zero(5) == 0 via mir2
// assert xor_bool(1, 0) == 1 via mir2
// assert xor_bool(1, 1) == 0 via mir2
// assert xor_bool(0, 0) == 0 via mir2
// assert clamp(50, 10, 100) == 50 via mir2
// assert clamp(5, 10, 100) == 10 via mir2
// assert clamp(200, 10, 100) == 100 via mir2
// assert sign(42) == 1 via mir2
// assert sign(0) == 0 via mir2
