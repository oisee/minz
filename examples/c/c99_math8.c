/*
 * C99 8-bit arithmetic — edge cases on Z80
 * Overflow, underflow, signed/unsigned, division
 *
 * mzv: echo "" | mzv -H c99_math8.c
 */
#include <stdint.h>
#include <stdbool.h>

uint8_t add8(uint8_t a, uint8_t b) { return a + b; }
uint8_t sub8(uint8_t a, uint8_t b) { return a - b; }
uint8_t mul8(uint8_t a, uint8_t b) { return a * b; }
uint8_t div8(uint8_t a, uint8_t b) { return a / b; }
uint8_t mod8(uint8_t a, uint8_t b) { return a % b; }

uint8_t abs_diff(uint8_t a, uint8_t b) {
    return a > b ? a - b : b - a;
}

uint8_t min8(uint8_t a, uint8_t b) { return a < b ? a : b; }
uint8_t max8(uint8_t a, uint8_t b) { return a > b ? a : b; }

uint8_t clamp8(uint8_t val, uint8_t lo, uint8_t hi) {
    return val < lo ? lo : val > hi ? hi : val;
}

uint8_t gcd8(uint8_t a, uint8_t b) {
    while (b != 0) {
        uint8_t t = b;
        b = a % b;
        a = t;
    }
    return a;
}

bool is_even(uint8_t x) { return (x & 1) == 0; }

uint8_t double_it(uint8_t x) { return x << 1; }
uint8_t halve_it(uint8_t x) { return x >> 1; }

uint8_t saturating_add(uint8_t a, uint8_t b) {
    uint16_t sum = (uint16_t)a + (uint16_t)b;
    return sum > 255 ? 255 : (uint8_t)sum;
}

uint8_t average(uint8_t a, uint8_t b) {
    return ((uint16_t)a + (uint16_t)b) / 2;
}

// assert add8(100, 50) == 150 via mir2
// assert add8(200, 100) == 300 via mir2
// assert sub8(100, 30) == 70 via mir2
// assert mul8(7, 6) == 42 via mir2
// assert mul8(16, 16) == 256 via mir2
// assert div8(100, 10) == 10 via mir2
// assert div8(255, 1) == 255 via mir2
// assert mod8(17, 5) == 2 via mir2
// assert mod8(100, 10) == 0 via mir2
// assert abs_diff(10, 3) == 7 via mir2
// assert abs_diff(3, 10) == 7 via mir2
// assert abs_diff(5, 5) == 0 via mir2
// assert min8(10, 20) == 10 via mir2
// assert min8(20, 10) == 10 via mir2
// assert max8(10, 20) == 20 via mir2
// assert max8(20, 10) == 20 via mir2
// assert clamp8(50, 10, 100) == 50 via mir2
// assert clamp8(5, 10, 100) == 10 via mir2
// assert clamp8(200, 10, 100) == 100 via mir2
// assert gcd8(12, 8) == 4 via mir2
// assert gcd8(7, 13) == 1 via mir2
// assert gcd8(100, 25) == 25 via mir2
// assert is_even(0) == 1 via mir2
// assert is_even(42) == 1 via mir2
// assert is_even(7) == 0 via mir2
// assert double_it(21) == 42 via mir2
// assert halve_it(84) == 42 via mir2
// assert saturating_add(200, 100) == 255 via mir2
// assert saturating_add(100, 50) == 150 via mir2
// assert average(10, 20) == 15 via mir2
// assert average(0, 255) == 127 via mir2
