/*
 * C11 Anonymous Structs and Unions on Z80
 * Fields promoted directly into parent — no prefix needed.
 *
 * mzv: echo "" | mzv -H c11_anon_struct.c
 * CP/M: mz c11_anon_struct.c --target=cpm -o out.a80
 */
#include <stdint.h>

/* Anonymous struct — fields promoted into parent */
struct Pair {
    struct { uint8_t a; uint8_t b; };
};

uint8_t pair_first(void) {
    struct Pair p;
    p.a = 42;
    p.b = 99;
    return p.a;
}

uint8_t pair_second(void) {
    struct Pair p;
    p.a = 42;
    p.b = 99;
    return p.b;
}

/* Anonymous union — overlapping access */
struct Register {
    union {
        uint16_t hl;
        struct { uint8_t l; uint8_t h; };
    };
};

uint8_t reg_low(void) {
    struct Register r;
    r.l = 0x34;
    r.h = 0x12;
    return r.l;
}

uint8_t reg_high(void) {
    struct Register r;
    r.l = 0x34;
    r.h = 0x12;
    return r.h;
}

/* _Alignof — always 1 on Z80 (byte-addressed) */
uint8_t test_alignof(void) {
    return _Alignof(uint16_t);
}

/* typeof — infer type from expression */
uint8_t test_typeof(uint8_t x) {
    typeof(x) y = x * 2;
    return y;
}

// assert pair_first() == 42 via mir2
// assert pair_second() == 99 via mir2
// assert reg_low() == 0x34 via mir2
// assert reg_high() == 0x12 via mir2
// assert test_alignof() == 1 via mir2
// assert test_typeof(21) == 42 via mir2
