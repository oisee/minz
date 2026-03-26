/*
 * C23 constexpr + enum underlying type on Z80
 *
 * constexpr: compile-time constant (mapped to const on Z80)
 * enum E : uint8_t — explicit underlying type, saves bytes on Z80
 *
 * mzv: echo "" | mzv -H c23_constexpr_enum.c
 */
#include <stdint.h>
#include <stdbool.h>

/* constexpr — compile-time constants */
constexpr int BUFFER_SIZE = 32;
constexpr int MAX_PLAYERS = 4;
constexpr int TILE_SIZE = 8;

uint8_t get_buffer_size(void) { return BUFFER_SIZE; }
uint8_t get_max_players(void) { return MAX_PLAYERS; }
uint8_t get_tile_pixels(void) { return TILE_SIZE * TILE_SIZE; }

/* enum with underlying type (C23) — uint8_t saves 1 byte per value on Z80 */
enum Color : uint8_t { RED = 0, GREEN = 1, BLUE = 2, YELLOW = 3 };

uint8_t color_value(enum Color c) { return c; }
uint8_t next_color(enum Color c) { return (c + 1) % 4; }
uint8_t is_primary(enum Color c) { return c <= BLUE; }

/* enum with int (default 16-bit on Z80) */
enum Status { OK = 0, ERR_TIMEOUT = 1, ERR_OVERFLOW = 2, ERR_IO = 100 };

uint8_t status_is_error(enum Status s) { return s != OK; }

/* constexpr with enum — compile-time lookup */
constexpr uint8_t COLOR_COUNT = 4;
constexpr uint8_t PRIMARY_COUNT = 3;

uint8_t palette_total(void) { return COLOR_COUNT; }
uint8_t primary_count(void) { return PRIMARY_COUNT; }

/* Sized enum for flags — each flag is 1 byte */
enum Flags : uint8_t {
    FLAG_NONE    = 0x00,
    FLAG_VISIBLE = 0x01,
    FLAG_ACTIVE  = 0x02,
    FLAG_SOLID   = 0x04,
    FLAG_ALL     = 0x07
};

uint8_t has_flag(enum Flags f, enum Flags mask) {
    return (f & mask) != 0;
}

uint8_t combine_flags(void) {
    return FLAG_VISIBLE | FLAG_SOLID;
}

/* sizeof enum — uint8_t enum should be 1 byte */
uint8_t size_color(void) { return sizeof(enum Color); }
uint8_t size_flags(void) { return sizeof(enum Flags); }
uint8_t size_status(void) { return sizeof(enum Status); }

// constexpr
// assert get_buffer_size() == 32 via mir2
// assert get_max_players() == 4 via mir2
// assert get_tile_pixels() == 64 via mir2

// enum Color : uint8_t
// assert color_value(0) == 0 via mir2
// assert color_value(2) == 2 via mir2
// assert next_color(0) == 1 via mir2
// assert next_color(3) == 0 via mir2
// assert is_primary(0) == 1 via mir2
// assert is_primary(3) == 0 via mir2

// enum Status (default int)
// assert status_is_error(0) == 0 via mir2
// assert status_is_error(1) == 1 via mir2
// assert status_is_error(100) == 1 via mir2

// constexpr + enum
// assert palette_total() == 4 via mir2
// assert primary_count() == 3 via mir2

// flags enum
// assert has_flag(5, 1) == 1 via mir2
// assert has_flag(5, 2) == 0 via mir2
// assert combine_flags() == 5 via mir2

// sizeof — uint8_t enum = 1 byte on Z80 (C23 sized enum)
// assert size_color() == 1 via mir2
// assert size_flags() == 1 via mir2
// assert size_status() == 2 via mir2
