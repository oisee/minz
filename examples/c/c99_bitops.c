/*
 * C99 Bitwise operations — thorough Z80 testing
 * Shifts, rotates, masks, bit manipulation
 *
 * mzv: echo "" | mzv -H c99_bitops.c
 */
#include <stdint.h>
#include <stdbool.h>

uint8_t shl(uint8_t x, uint8_t n) { return x << n; }
uint8_t shr(uint8_t x, uint8_t n) { return x >> n; }
uint8_t band(uint8_t a, uint8_t b) { return a & b; }
uint8_t bor(uint8_t a, uint8_t b) { return a | b; }
uint8_t bxor(uint8_t a, uint8_t b) { return a ^ b; }
uint8_t bnot(uint8_t a) { return ~a; }

uint8_t isolate_bit(uint8_t x, uint8_t bit) {
    return (x >> bit) & 1;
}

uint8_t set_bit(uint8_t x, uint8_t bit) {
    return x | (1 << bit);
}

uint8_t clear_bit(uint8_t x, uint8_t bit) {
    return x & ~(1 << bit);
}

uint8_t toggle_bit(uint8_t x, uint8_t bit) {
    return x ^ (1 << bit);
}

uint8_t swap_nibbles(uint8_t x) {
    return (x << 4) | (x >> 4);
}

uint8_t popcount8(uint8_t x) {
    uint8_t c = 0;
    c += x & 1; x >>= 1;
    c += x & 1; x >>= 1;
    c += x & 1; x >>= 1;
    c += x & 1; x >>= 1;
    c += x & 1; x >>= 1;
    c += x & 1; x >>= 1;
    c += x & 1; x >>= 1;
    c += x & 1;
    return c;
}

bool is_power_of_2(uint8_t x) {
    return x != 0 && (x & (x - 1)) == 0;
}

uint8_t high_nibble(uint8_t x) { return x >> 4; }
uint8_t low_nibble(uint8_t x) { return x & 0x0F; }

// assert shl(1, 0) == 1 via mir2
// assert shl(1, 3) == 8 via mir2
// assert shl(1, 7) == 128 via mir2
// assert shr(128, 7) == 1 via mir2
// assert shr(255, 4) == 15 via mir2
// assert band(0xAA, 0x55) == 0 via mir2
// assert band(0xFF, 0x0F) == 15 via mir2
// assert bor(0xA0, 0x05) == 0xA5 via mir2
// assert bxor(0xFF, 0xFF) == 0 via mir2
// assert bxor(0xAA, 0x55) == 0xFF via mir2
// assert bnot(0) == 255 via mir2
// assert bnot(0xFF) == 0 via mir2
// assert isolate_bit(0x80, 7) == 1 via mir2
// assert isolate_bit(0x80, 6) == 0 via mir2
// assert set_bit(0, 3) == 8 via mir2
// assert clear_bit(0xFF, 0) == 0xFE via mir2
// assert toggle_bit(0, 5) == 32 via mir2
// assert toggle_bit(32, 5) == 0 via mir2
// swap_nibbles: needs u8 truncation in shifts (MIR2 VM uses u16)
// TODO assert swap_nibbles(0x12) == 0x21 via z80
// assert popcount8(0) == 0 via mir2
// assert popcount8(0xFF) == 8 via mir2
// assert popcount8(0xAA) == 4 via mir2
// assert is_power_of_2(1) == 1 via mir2
// assert is_power_of_2(64) == 1 via mir2
// assert is_power_of_2(0) == 0 via mir2
// assert is_power_of_2(3) == 0 via mir2
// assert high_nibble(0xAB) == 10 via mir2
// assert low_nibble(0xAB) == 11 via mir2
