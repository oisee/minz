// Struct-Return Promotion (ADR-0025)
// Small structs returned from functions are promoted to tuple returns,
// enabling PFCCO to assign each field an optimal register — 0T overhead.

typedef struct { uint8_t q; uint8_t r; } DivResult;

// Direct struct literal return — eligible for promotion.
DivResult divmod(uint8_t a, uint8_t b) {
    DivResult res = { a / b, a % b };
    return res;
}

uint8_t get_quotient(uint8_t a, uint8_t b) {
    DivResult d = divmod(a, b);
    return d.q;
}

uint8_t get_remainder(uint8_t a, uint8_t b) {
    DivResult d = divmod(a, b);
    return d.r;
}

uint8_t sum_qr(uint8_t a, uint8_t b) {
    DivResult d = divmod(a, b);
    return d.q + d.r;
}

// assert get_quotient(17, 5) == 3
// assert get_remainder(17, 5) == 2
// assert sum_qr(17, 5) == 5
// assert get_quotient(100, 10) == 10
// assert get_remainder(100, 10) == 0
// assert sum_qr(255, 16) == 31
