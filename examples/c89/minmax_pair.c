// MinMax with pair return via struct promotion (ADR-0025)
// Compares with: Nanz native (ex11_minmax_multiret.nanz)
//                SDCC 4.2.0 (sdcc_minmax.c — pointer-based, no multi-return)

typedef struct { uint16_t lo; uint16_t hi; } Pair;

Pair minmax(uint16_t a, uint16_t b) {
    if (a <= b) {
        Pair r = { a, b };
        return r;
    }
    Pair r = { b, a };
    return r;
}

uint16_t smaller(uint16_t a, uint16_t b) {
    Pair p = minmax(a, b);
    return p.lo;
}

uint16_t larger(uint16_t a, uint16_t b) {
    Pair p = minmax(a, b);
    return p.hi;
}

// assert smaller(10, 20) == 10 via mir2
// assert smaller(20, 10) == 10 via mir2
// assert larger(10, 20) == 20 via mir2
// assert larger(20, 10) == 20 via mir2
// assert smaller(5, 5) == 5 via mir2
// assert larger(5, 5) == 5 via mir2
