// Struct-Return Promotion (ADR-0025)
// Small structs returned from functions are promoted to tuple returns,
// enabling PFCCO to assign each field an optimal register — 0T overhead.

typedef struct { uint8_t q; uint8_t r; } DivResult;

// Brace-initialized struct return — eligible for promotion.
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

// Inline struct init, no function call.
uint8_t direct_init(void) {
    DivResult d = { 10, 3 };
    return d.q + d.r;
}

// assert direct_init() == 13
