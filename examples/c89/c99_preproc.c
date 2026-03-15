/* C preprocessor features: #define, #if, #ifdef, macros */

#define U8 unsigned char
#define MAX_VAL 255
#define SQUARE(x) ((x) * (x))
#define MIN(a, b) ((a) < (b) ? (a) : (b))
#define MAX(a, b) ((a) > (b) ? (a) : (b))
#define ABS(x) ((x) < 0 ? -(x) : (x))
#define CLAMP(x, lo, hi) MIN(MAX(x, lo), hi)
#define BIT(n) (1 << (n))
#define SET_BIT(x, n) ((x) | BIT(n))
#define CLR_BIT(x, n) ((x) & ~BIT(n))
#define TST_BIT(x, n) (((x) >> (n)) & 1)

#ifdef __Z80__
#define PLATFORM 1
#else
#define PLATFORM 0
#endif

#ifdef __MINZ__
#define COMPILER 1
#else
#define COMPILER 0
#endif

U8 square(U8 x) {
    return SQUARE(x);
}

U8 min_of(U8 a, U8 b) {
    return MIN(a, b);
}

U8 max_of(U8 a, U8 b) {
    return MAX(a, b);
}

U8 clamped(U8 x, U8 lo, U8 hi) {
    return CLAMP(x, lo, hi);
}

U8 set_bit(U8 x, U8 n) {
    return SET_BIT(x, n);
}

U8 clr_bit(U8 x, U8 n) {
    return CLR_BIT(x, n);
}

U8 tst_bit(U8 x, U8 n) {
    return TST_BIT(x, n);
}

U8 get_platform(void) {
    return PLATFORM;
}

U8 get_compiler(void) {
    return COMPILER;
}

U8 get_max_val(void) {
    return MAX_VAL;
}

// assert square(5) == 25 via mir2
// assert square(0) == 0 via mir2
// assert square(3) == 9 via mir2
// assert min_of(3, 7) == 3 via mir2
// assert min_of(10, 2) == 2 via mir2
// assert max_of(3, 7) == 7 via mir2
// assert max_of(10, 2) == 10 via mir2
// assert clamped(50, 10, 100) == 50 via mir2
// assert clamped(5, 10, 100) == 10 via mir2
// assert clamped(200, 10, 100) == 100 via mir2
// assert set_bit(0, 3) == 8 via mir2
// assert set_bit(1, 7) == 129 via mir2
// assert clr_bit(255, 0) == 254 via mir2
// assert clr_bit(255, 7) == 127 via mir2
// assert tst_bit(128, 7) == 1 via mir2
// assert tst_bit(128, 0) == 0 via mir2
// assert get_platform() == 1 via mir2
// assert get_compiler() == 1 via mir2
// assert get_max_val() == 255 via mir2
