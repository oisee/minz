/*
 * C23 _BitInt(N) on Z80
 * Maps to nearest Z80 native type:
 *   _BitInt(1)...(8)   → uint8_t
 *   _BitInt(9)...(16)  → uint16_t
 *   _BitInt(17)...(32) → uint32_t
 *   _BitInt(>32)       → error (too wide for Z80)
 *
 * mzv: echo "" | mzv -H c23_bitint.c
 */
#include <stdint.h>
#include <stdbool.h>

/* 1-bit: boolean flag */
_BitInt(1) get_flag(void) { return 1; }

/* 4-bit: nibble */
_BitInt(4) get_nibble(void) { return 0x0F; }

/* 8-bit: byte — maps directly to uint8_t */
_BitInt(8) add_bytes(_BitInt(8) a, _BitInt(8) b) { return a + b; }

/* 16-bit: word */
_BitInt(16) get_word(void) { return 1'000; }

/* Mixed: _BitInt in expressions */
uint8_t nibble_add(void) {
    _BitInt(4) a = 5;
    _BitInt(4) b = 3;
    return a + b;
}

/* sizeof — verify mapping */
uint8_t size_bit1(void) { return sizeof(_BitInt(1)); }
uint8_t size_bit4(void) { return sizeof(_BitInt(4)); }
uint8_t size_bit8(void) { return sizeof(_BitInt(8)); }
uint8_t size_bit16(void) { return sizeof(_BitInt(16)); }

/* Practical: pixel color (5-6-5 RGB fields stored in native types) */
_BitInt(5) get_red(uint16_t rgb565) {
    return (rgb565 >> 11) & 0x1F;
}

_BitInt(6) get_green(uint16_t rgb565) {
    return (rgb565 >> 5) & 0x3F;
}

_BitInt(5) get_blue(uint16_t rgb565) {
    return rgb565 & 0x1F;
}

// basic types
// assert get_flag() == 1 via mir2
// assert get_nibble() == 15 via mir2
// assert add_bytes(100, 42) == 142 via mir2
// assert get_word() == 1000 via mir2
// assert nibble_add() == 8 via mir2

// sizeof
// assert size_bit1() == 1 via mir2
// assert size_bit4() == 1 via mir2
// assert size_bit8() == 1 via mir2
// assert size_bit16() == 2 via mir2

// rgb565 extraction
// assert get_red(0xF800) == 31 via mir2
// assert get_green(0x07E0) == 63 via mir2
// assert get_blue(0x001F) == 31 via mir2
