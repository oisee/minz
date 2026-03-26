/*
 * C23 Digit separators on Z80
 * 1'000 → 1000, 0xFF'FF → 0xFFFF, 0b1010'1010 → 0xAA
 * Preprocessor strips ' between digits. Char literals ('A') unaffected.
 *
 * mzv: echo "" | mzv -H c23_digit_sep.c
 */
#include <stdint.h>

/* Decimal */
uint16_t thousand(void) { return 1'000; }
uint8_t two_fifty_five(void) { return 2'5'5; }

/* Hex */
uint16_t hex_ffff(void) { return 0xFF'FF; }
uint8_t hex_ab(void) { return 0xA'B; }

/* Binary */
uint8_t bin_aa(void) { return 0b1010'1010; }
uint8_t bin_0f(void) { return 0b0000'1111; }

/* Char literals NOT affected */
uint8_t char_a(void) { return 'A'; }
uint8_t char_zero(void) { return '0'; }

/* Mixed: digit separators + char literals in same file */
uint8_t mixed(void) {
    uint16_t big = 1'000;
    uint8_t ch = 'X';
    return (uint8_t)(big / 100) + ch - 'X';
}

// assert thousand() == 1000 via mir2
// assert two_fifty_five() == 255 via mir2
// assert hex_ffff() == 0xFFFF via mir2
// assert hex_ab() == 0xAB via mir2
// assert bin_aa() == 0xAA via mir2
// assert bin_0f() == 0x0F via mir2
// assert char_a() == 65 via mir2
// assert char_zero() == 48 via mir2
// assert mixed() == 10 via mir2
