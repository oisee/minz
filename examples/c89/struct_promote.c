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

// Out-param pattern: void f(..., SSS *out) → promoted to tuple return.
void divmod_out(uint8_t a, uint8_t b, DivResult *out) {
    out->q = a / b;
    out->r = a % b;
}

uint8_t use_outparam(uint8_t a, uint8_t b) {
    DivResult res;
    divmod_out(a, b, &res);
    return res.q + res.r;
}

// Pointer-return pattern: SSS* f() → promoted to tuple return.
DivResult* make_pair(uint8_t a, uint8_t b) {
    DivResult tmp;
    tmp.q = a;
    tmp.r = b;
    return &tmp;
}

uint8_t use_ptrreturn(uint8_t a, uint8_t b) {
    DivResult *p = make_pair(a, b);
    return p->q + p->r;
}

// assert direct_init() == 13
// assert use_outparam(17, 5) == 5
// assert use_ptrreturn(10, 20) == 30
