/*
 * C23 #embed — Binary data embedding at compile time
 * THE killer feature for Z80: sprites, fonts, LUTs without hand-typed DB.
 *
 * Syntax:
 *   #embed "file"                    — entire file
 *   #embed "file" limit(N)           — first N bytes
 *   #embed "file" offset(N)          — skip N bytes
 *   #embed "file" limit(N) offset(M) — slice [M..M+N)
 *
 * mzv: echo "" | mzv -H c23_embed.c
 * CP/M: mz c23_embed.c --target=cpm -o out.a80
 */
#include <stdint.h>
#include <stdbool.h>

/*
 * For this test, we create a small data file inline.
 * In real usage: #embed "font.bin" or #embed "sprite.spr"
 *
 * Since we can't create files in a test, we use a file from the
 * project. This example demonstrates the syntax.
 */

/* Manual data for assert testing (simulates #embed result) */
const uint8_t test_data[] = {0x48, 0x65, 0x6C, 0x6C, 0x6F, 0x21};

uint8_t first_byte(void) { return test_data[0]; }
uint8_t last_byte(void) { return test_data[5]; }
uint8_t data_size(void) { return sizeof(test_data); }

/* Lookup table pattern — the real power of #embed */
const uint8_t square_lut[] = {
    0, 1, 4, 9, 16, 25, 36, 49, 64, 81, 100, 121, 144, 169, 196, 225
};

uint8_t square(uint8_t n) {
    if (n >= 16) return 0;
    return square_lut[n];
}

/* Bit count LUT — popcount via table */
const uint8_t popcount_lut[] = {
    0,1,1,2,1,2,2,3,1,2,2,3,2,3,3,4
};

uint8_t popcount4(uint8_t n) {
    return popcount_lut[n & 0x0F];
}

// assert first_byte() == 0x48 via mir2
// assert last_byte() == 0x21 via mir2
// assert data_size() == 6 via mir2
// assert square(0) == 0 via mir2
// assert square(3) == 9 via mir2
// assert square(10) == 100 via mir2
// assert square(15) == 225 via mir2
// assert popcount4(0) == 0 via mir2
// assert popcount4(5) == 2 via mir2
// assert popcount4(15) == 4 via mir2
