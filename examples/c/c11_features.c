/*
 * C11 features on Z80
 * Mixed declarations, designated init, compound literals, inline
 *
 * mzv: echo "" | mzv -H c11_features.c
 * CP/M: mz c11_features.c --target=cpm -o out.a80
 */
#include <stdint.h>
#include <stdbool.h>

/* Designated initializers (C99) — out-of-order fields */
typedef struct { uint8_t x; uint8_t y; uint8_t z; } Vec3;

uint8_t vec3_sum(void) {
    Vec3 v = {.z = 30, .x = 10, .y = 20};
    return v.x + v.y + v.z;
}

/* Compound literals (C99) */
uint8_t compound_lit(void) {
    Vec3 v = (Vec3){.x = 5, .y = 10, .z = 15};
    return v.x + v.y + v.z;
}

/* Mixed declarations and code (C99) */
uint8_t mixed_decls(uint8_t a, uint8_t b) {
    uint8_t sum = a + b;
    if (sum > 100) return 100;
    uint8_t doubled = sum * 2;
    return doubled;
}

/* for-loop inline declaration (C99) */
uint8_t loop_sum(uint8_t n) {
    uint8_t total = 0;
    for (int i = 0; i < n; i++) {
        total += 1;
    }
    return total;
}

/* bool without #include (C23-style, via predefined macros) */
bool is_between(uint8_t val, uint8_t lo, uint8_t hi) {
    return val >= lo && val <= hi;
}

/* inline function (C99) */
static inline uint8_t double_it(uint8_t x) {
    return x * 2;
}

uint8_t test_inline(uint8_t x) {
    return double_it(x);
}

// assert vec3_sum() == 60 via mir2
// assert compound_lit() == 30 via mir2
// assert mixed_decls(10, 20) == 60 via mir2
// assert mixed_decls(60, 60) == 100 via mir2
// assert loop_sum(5) == 5 via mir2
// assert loop_sum(0) == 0 via mir2
// assert is_between(50, 10, 100) == 1 via mir2
// assert is_between(5, 10, 100) == 0 via mir2
// assert test_inline(21) == 42 via mir2
