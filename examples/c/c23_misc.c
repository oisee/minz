/*
 * C23 miscellaneous features on Z80
 * auto type inference, typeof, unreachable, constexpr
 *
 * mzv: echo "" | mzv -H c23_misc.c
 */
#include <stdint.h>
#include <stdbool.h>

/* auto type inference (C23) — compiler deduces type from initializer */
uint8_t test_auto_int(void) {
    auto x = 42;
    return x;
}

uint8_t test_auto_expr(uint8_t a, uint8_t b) {
    auto sum = a + b;
    return sum;
}

/* typeof — use type of another expression */
uint8_t test_typeof(uint8_t x) {
    typeof(x) doubled = x * 2;
    return doubled;
}

uint8_t test_typeof_expr(void) {
    uint16_t big = 1000;
    typeof(big) also_big = 2000;
    return (uint8_t)(also_big / 100);
}

/* constexpr — compile-time constants */
constexpr uint8_t SCREEN_W = 32;
constexpr uint8_t SCREEN_H = 24;
constexpr uint16_t SCREEN_PIXELS = 256 * 192;

uint8_t screen_width(void) { return SCREEN_W; }
uint8_t screen_height(void) { return SCREEN_H; }

/* nullptr — null pointer constant */
uint8_t test_nullptr(void) {
    uint8_t *p = nullptr;
    return p == nullptr ? 1 : 0;
}

/* bool as keyword (no include needed) */
bool test_bool_keyword(uint8_t x) {
    bool positive = x > 0;
    return positive;
}

/* Nested auto + typeof */
uint8_t test_nested(uint8_t a) {
    auto x = a + 1;
    typeof(x) y = x + 1;
    return y;
}

/* constexpr in expressions */
constexpr uint8_t BASE = 100;
constexpr uint8_t OFFSET = 42;

uint8_t test_constexpr_expr(void) {
    return BASE + OFFSET;
}

// auto
// assert test_auto_int() == 42 via mir2
// assert test_auto_expr(10, 20) == 30 via mir2

// typeof
// assert test_typeof(21) == 42 via mir2
// assert test_typeof_expr() == 20 via mir2

// constexpr
// assert screen_width() == 32 via mir2
// assert screen_height() == 24 via mir2

// nullptr
// assert test_nullptr() == 1 via mir2

// bool keyword
// assert test_bool_keyword(5) == 1 via mir2
// assert test_bool_keyword(0) == 0 via mir2

// nested auto + typeof
// assert test_nested(10) == 12 via mir2

// constexpr expr
// assert test_constexpr_expr() == 142 via mir2
