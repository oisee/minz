/*
 * C23 <stdbit.h> — Bit manipulation on Z80
 * popcount, clz, ctz, bit_width, has_single_bit, bit_ceil, bit_floor
 *
 * mzv: echo "" | mzv -H c23_stdbit.c
 * CP/M: mz c23_stdbit.c --target=cpm -o out.a80
 */
#include <stdint.h>
#include <stdbit.h>

/* Population count — number of 1-bits */
uint8_t popcount_0(void) { return stdc_count_ones_uc(0); }
uint8_t popcount_1(void) { return stdc_count_ones_uc(1); }
uint8_t popcount_ff(void) { return stdc_count_ones_uc(0xFF); }
uint8_t popcount_aa(void) { return stdc_count_ones_uc(0xAA); }
uint8_t popcount_55(void) { return stdc_count_ones_uc(0x55); }

/* Count leading zeros */
uint8_t clz_0(void) { return stdc_leading_zeros_uc(0); }
uint8_t clz_1(void) { return stdc_leading_zeros_uc(1); }
uint8_t clz_80(void) { return stdc_leading_zeros_uc(0x80); }
uint8_t clz_0f(void) { return stdc_leading_zeros_uc(0x0F); }
uint8_t clz_40(void) { return stdc_leading_zeros_uc(0x40); }

/* Count trailing zeros */
uint8_t ctz_0(void) { return stdc_trailing_zeros_uc(0); }
uint8_t ctz_1(void) { return stdc_trailing_zeros_uc(1); }
uint8_t ctz_80(void) { return stdc_trailing_zeros_uc(0x80); }
uint8_t ctz_10(void) { return stdc_trailing_zeros_uc(0x10); }

/* Has single bit (power of 2) */
uint8_t hsb_0(void) { return stdc_has_single_bit_uc(0); }
uint8_t hsb_1(void) { return stdc_has_single_bit_uc(1); }
uint8_t hsb_64(void) { return stdc_has_single_bit_uc(64); }
uint8_t hsb_3(void) { return stdc_has_single_bit_uc(3); }

/* Bit width — floor(log2) + 1 */
uint8_t bw_0(void) { return stdc_bit_width_uc(0); }
uint8_t bw_1(void) { return stdc_bit_width_uc(1); }
uint8_t bw_7(void) { return stdc_bit_width_uc(7); }
uint8_t bw_8(void) { return stdc_bit_width_uc(8); }
uint8_t bw_ff(void) { return stdc_bit_width_uc(255); }

/* Bit ceil — smallest power of 2 >= value */
uint8_t bc_0(void) { return stdc_bit_ceil_uc(0); }
uint8_t bc_1(void) { return stdc_bit_ceil_uc(1); }
uint8_t bc_3(void) { return stdc_bit_ceil_uc(3); }
uint8_t bc_5(void) { return stdc_bit_ceil_uc(5); }

/* Bit floor — largest power of 2 <= value */
uint8_t bf_0(void) { return stdc_bit_floor_uc(0); }
uint8_t bf_1(void) { return stdc_bit_floor_uc(1); }
uint8_t bf_5(void) { return stdc_bit_floor_uc(5); }
uint8_t bf_7(void) { return stdc_bit_floor_uc(7); }

/* Count zeros */
uint8_t cz_0(void) { return stdc_count_zeros_uc(0); }
uint8_t cz_ff(void) { return stdc_count_zeros_uc(0xFF); }
uint8_t cz_aa(void) { return stdc_count_zeros_uc(0xAA); }

// popcount
// assert popcount_0() == 0 via mir2
// assert popcount_1() == 1 via mir2
// assert popcount_ff() == 8 via mir2
// assert popcount_aa() == 4 via mir2
// assert popcount_55() == 4 via mir2

// clz
// assert clz_0() == 8 via mir2
// assert clz_1() == 7 via mir2
// assert clz_80() == 0 via mir2
// assert clz_0f() == 4 via mir2
// assert clz_40() == 1 via mir2

// ctz
// assert ctz_0() == 8 via mir2
// assert ctz_1() == 0 via mir2
// assert ctz_80() == 7 via mir2
// assert ctz_10() == 4 via mir2

// has_single_bit
// assert hsb_0() == 0 via mir2
// assert hsb_1() == 1 via mir2
// assert hsb_64() == 1 via mir2
// assert hsb_3() == 0 via mir2

// bit_width
// assert bw_0() == 0 via mir2
// assert bw_1() == 1 via mir2
// assert bw_7() == 3 via mir2
// assert bw_8() == 4 via mir2
// assert bw_ff() == 8 via mir2

// bit_ceil
// assert bc_0() == 1 via mir2
// assert bc_1() == 1 via mir2
// assert bc_3() == 4 via mir2
// assert bc_5() == 8 via mir2

// bit_floor
// assert bf_0() == 0 via mir2
// assert bf_1() == 1 via mir2
// assert bf_5() == 4 via mir2
// assert bf_7() == 4 via mir2

// count_zeros
// assert cz_0() == 8 via mir2
// assert cz_ff() == 0 via mir2
// assert cz_aa() == 4 via mir2
