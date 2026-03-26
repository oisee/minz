/* MinZ stdbit.h for Z80 targets (C23) */
/* Bit manipulation utilities — inline implementations */
#ifndef _MINZ_STDBIT_H
#define _MINZ_STDBIT_H

#include <stdint.h>

/* Population count — number of 1-bits */
static unsigned int stdc_count_ones_uc(unsigned char value) {
    unsigned char v = value;
    unsigned char c = 0;
    while (v) { c += v & 1; v >>= 1; }
    return c;
}

static unsigned int stdc_count_ones_us(unsigned int value) {
    unsigned int v = value;
    unsigned int c = 0;
    while (v) { c += v & 1; v >>= 1; }
    return c;
}

/* Count leading zeros */
static unsigned int stdc_leading_zeros_uc(unsigned char value) {
    if (value == 0) return 8;
    unsigned int n = 0;
    if ((value & 0xF0) == 0) { n += 4; value <<= 4; }
    if ((value & 0xC0) == 0) { n += 2; value <<= 2; }
    if ((value & 0x80) == 0) { n += 1; }
    return n;
}

static unsigned int stdc_leading_zeros_us(unsigned int value) {
    if (value == 0) return 16;
    unsigned int n = 0;
    if ((value & 0xFF00) == 0) { n += 8; value <<= 8; }
    if ((value & 0xF000) == 0) { n += 4; value <<= 4; }
    if ((value & 0xC000) == 0) { n += 2; value <<= 2; }
    if ((value & 0x8000) == 0) { n += 1; }
    return n;
}

/* Count trailing zeros */
static unsigned int stdc_trailing_zeros_uc(unsigned char value) {
    if (value == 0) return 8;
    unsigned int n = 0;
    if ((value & 0x0F) == 0) { n += 4; value >>= 4; }
    if ((value & 0x03) == 0) { n += 2; value >>= 2; }
    if ((value & 0x01) == 0) { n += 1; }
    return n;
}

static unsigned int stdc_trailing_zeros_us(unsigned int value) {
    if (value == 0) return 16;
    unsigned int n = 0;
    if ((value & 0x00FF) == 0) { n += 8; value >>= 8; }
    if ((value & 0x000F) == 0) { n += 4; value >>= 4; }
    if ((value & 0x0003) == 0) { n += 2; value >>= 2; }
    if ((value & 0x0001) == 0) { n += 1; }
    return n;
}

/* Count zeros = width - count_ones */
static unsigned int stdc_count_zeros_uc(unsigned char value) {
    return 8 - stdc_count_ones_uc(value);
}

static unsigned int stdc_count_zeros_us(unsigned int value) {
    return 16 - stdc_count_ones_us(value);
}

/* Leading ones */
static unsigned int stdc_leading_ones_uc(unsigned char value) {
    return stdc_leading_zeros_uc(~value);
}

/* Trailing ones */
static unsigned int stdc_trailing_ones_uc(unsigned char value) {
    return stdc_trailing_zeros_uc(~value);
}

/* Has single bit — is power of 2 (and nonzero) */
static int stdc_has_single_bit_uc(unsigned char value) {
    return value != 0 && (value & (value - 1)) == 0;
}

static int stdc_has_single_bit_us(unsigned int value) {
    return value != 0 && (value & (value - 1)) == 0;
}

/* Bit width — floor(log2(x)) + 1, or 0 for x==0 */
static unsigned int stdc_bit_width_uc(unsigned char value) {
    if (value == 0) return 0;
    return 8 - stdc_leading_zeros_uc(value);
}

static unsigned int stdc_bit_width_us(unsigned int value) {
    if (value == 0) return 0;
    return 16 - stdc_leading_zeros_us(value);
}

/* Bit ceil — smallest power of 2 >= value */
static unsigned char stdc_bit_ceil_uc(unsigned char value) {
    if (value <= 1) return 1;
    unsigned int w = stdc_bit_width_uc(value - 1);
    return 1 << w;
}

/* Bit floor — largest power of 2 <= value, or 0 */
static unsigned char stdc_bit_floor_uc(unsigned char value) {
    if (value == 0) return 0;
    unsigned int w = stdc_bit_width_uc(value);
    return 1 << (w - 1);
}

/* Type-generic macros (C23 style) — route to _uc for uint8_t, _us for uint16_t */
#define stdc_count_ones(x) \
    (sizeof(x) <= 1 ? stdc_count_ones_uc(x) : stdc_count_ones_us(x))
#define stdc_count_zeros(x) \
    (sizeof(x) <= 1 ? stdc_count_zeros_uc(x) : stdc_count_zeros_us(x))
#define stdc_leading_zeros(x) \
    (sizeof(x) <= 1 ? stdc_leading_zeros_uc(x) : stdc_leading_zeros_us(x))
#define stdc_trailing_zeros(x) \
    (sizeof(x) <= 1 ? stdc_trailing_zeros_uc(x) : stdc_trailing_zeros_us(x))
#define stdc_has_single_bit(x) \
    (sizeof(x) <= 1 ? stdc_has_single_bit_uc(x) : stdc_has_single_bit_us(x))
#define stdc_bit_width(x) \
    (sizeof(x) <= 1 ? stdc_bit_width_uc(x) : stdc_bit_width_us(x))

#endif /* _MINZ_STDBIT_H */
