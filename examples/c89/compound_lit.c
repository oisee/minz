// C99 compound literals test

typedef struct { uint8_t q; uint8_t r; } DivResult;

// Compound literal struct return
DivResult make_div(uint8_t a, uint8_t b) {
    return (DivResult){a / b, a % b};
}

uint8_t test_div_q(void) {
    DivResult d = make_div(10, 3);
    return d.q;
}
// assert test_div_q() == 3 via mir2

uint8_t test_div_r(void) {
    DivResult d = make_div(10, 3);
    return d.r;
}
// assert test_div_r() == 1 via mir2

// Compound literal with designated init
DivResult make_pair(uint8_t a, uint8_t b) {
    return (DivResult){.q = a, .r = b};
}

uint8_t test_pair_q(void) {
    DivResult d = make_pair(7, 2);
    return d.q;
}
// assert test_pair_q() == 7 via mir2

uint8_t test_pair_r(void) {
    DivResult d = make_pair(7, 2);
    return d.r;
}
// assert test_pair_r() == 2 via mir2

// Scalar compound literal
int test_scalar(void) {
    return (int){42};
}
// assert test_scalar() == 42 via mir2
