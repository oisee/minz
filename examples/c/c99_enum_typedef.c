/*
 * C99 Enums, typedefs, static locals — Z80 testing
 *
 * mzv: echo "" | mzv -H c99_enum_typedef.c
 */
#include <stdint.h>
#include <stdbool.h>

/* Enum with explicit values */
enum Direction { NORTH = 0, EAST = 1, SOUTH = 2, WEST = 3 };

uint8_t opposite(enum Direction d) {
    return (d + 2) % 4;
}

uint8_t is_vertical(enum Direction d) {
    return d == NORTH || d == SOUTH;
}

/* Anonymous enum */
enum { OFF = 0, ON = 1, BLINK = 2 };

uint8_t next_state(uint8_t s) {
    return (s + 1) % 3;
}

/* Typedef for clarity */
typedef uint8_t byte;
typedef uint16_t word;

byte byte_add(byte a, byte b) { return a + b; }
byte word_lo(word w) { return (byte)(w & 0xFF); }
byte word_hi(word w) { return (byte)(w >> 8); }

/* Static local — retains value across calls */
uint8_t counter(void) {
    static uint8_t n = 0;
    n++;
    return n;
}

uint8_t test_counter(void) {
    counter();    /* n = 1 */
    counter();    /* n = 2 */
    return counter();  /* n = 3 */
}

/* Sizeof various types */
uint8_t size_u8(void) { return sizeof(uint8_t); }
uint8_t size_u16(void) { return sizeof(uint16_t); }
uint8_t size_ptr(void) { return sizeof(void*); }
uint8_t size_enum(void) { return sizeof(enum Direction); }

// assert opposite(0) == 2 via mir2
// assert opposite(1) == 3 via mir2
// assert opposite(2) == 0 via mir2
// assert is_vertical(0) == 1 via mir2
// assert is_vertical(1) == 0 via mir2
// assert next_state(0) == 1 via mir2
// assert next_state(2) == 0 via mir2
// assert byte_add(100, 42) == 142 via mir2
// assert word_lo(0x1234) == 0x34 via mir2
// assert word_hi(0x1234) == 0x12 via mir2
// assert test_counter() == 3 via mir2
// assert size_u8() == 1 via mir2
// assert size_u16() == 2 via mir2
// assert size_ptr() == 2 via mir2
// assert size_enum() == 2 via mir2
