/* C99+ quick wins: typedef, enum, _Bool, _Static_assert, for-init-decl */

typedef unsigned char u8;
typedef unsigned short u16;

/* _Static_assert — validated at parse time by cc/v4 */
_Static_assert(sizeof(u8) == 1, "u8 must be 1 byte");
_Static_assert(sizeof(u16) == 2, "u16 must be 2 bytes");

u8 double_it(u8 x) {
    return x + x;
}

u16 widen(u8 x) {
    return (u16)x * 3;
}

/* enum with typedef */
typedef enum { RED = 0, GREEN = 1, BLUE = 2 } Color;

u8 color_val(Color c) {
    switch (c) {
    case RED: return 0;
    case GREEN: return 1;
    case BLUE: return 2;
    default: return 255;
    }
}

/* plain enum */
enum Direction { NORTH = 0, EAST = 1, SOUTH = 2, WEST = 3 };

u8 dir_opposite(enum Direction d) {
    switch (d) {
    case NORTH: return SOUTH;
    case SOUTH: return NORTH;
    case EAST: return WEST;
    case WEST: return EAST;
    default: return 255;
    }
}

/* _Bool */
_Bool is_positive(u8 x) {
    return x > 0;
}

/* C99 for-init declaration */
u8 sum_range(u8 lo, u8 hi) {
    u8 s = 0;
    for (u8 i = lo; i < hi; i = i + 1) {
        s = s + i;
    }
    return s;
}

/* nested typedef */
typedef u8 byte;
byte byte_add(byte a, byte b) {
    return a + b;
}

// assert double_it(5) == 10 via mir2
// assert double_it(0) == 0 via mir2
// assert widen(10) == 30 via mir2
// assert widen(0) == 0 via mir2
// assert color_val(0) == 0 via mir2
// assert color_val(1) == 1 via mir2
// assert color_val(2) == 2 via mir2
// assert color_val(3) == 255 via mir2
// assert dir_opposite(0) == 2 via mir2
// assert dir_opposite(2) == 0 via mir2
// assert dir_opposite(1) == 3 via mir2
// assert dir_opposite(3) == 1 via mir2
// assert is_positive(5) == 1 via mir2
// assert is_positive(0) == 0 via mir2
// assert sum_range(0, 5) == 10 via mir2
// assert sum_range(1, 4) == 6 via mir2
// assert byte_add(3, 7) == 10 via mir2
// assert byte_add(0, 0) == 0 via mir2
