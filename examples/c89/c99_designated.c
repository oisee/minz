// C99 designated initializers test
//
// Uses struct-return pattern to enable promotion (ADR-0025).
// Each test function returns a struct that gets promoted to tuple.

typedef struct { uint8_t a; uint8_t b; } Pair;

// Designated: .field = val (basic)
Pair desig_basic(void) {
    Pair p = {.a = 10, .b = 20};
    return p;
}
uint8_t desig_basic_a(void) { Pair p = desig_basic(); return p.a; }
uint8_t desig_basic_b(void) { Pair p = desig_basic(); return p.b; }
// assert desig_basic_a() == 10 via mir2
// assert desig_basic_b() == 20 via mir2

// Designated: out of order
Pair desig_reverse(void) {
    Pair p = {.b = 7, .a = 3};
    return p;
}
uint8_t desig_rev_a(void) { Pair p = desig_reverse(); return p.a; }
uint8_t desig_rev_b(void) { Pair p = desig_reverse(); return p.b; }
// assert desig_rev_a() == 3 via mir2
// assert desig_rev_b() == 7 via mir2

// Mixed: positional + designated
Pair desig_mixed(void) {
    Pair p = {5, .b = 15};
    return p;
}
uint8_t desig_mixed_a(void) { Pair p = desig_mixed(); return p.a; }
uint8_t desig_mixed_b(void) { Pair p = desig_mixed(); return p.b; }
// assert desig_mixed_a() == 5 via mir2
// assert desig_mixed_b() == 15 via mir2

typedef struct { uint8_t x; uint8_t y; uint8_t z; } Triple;

// Three fields, designated out-of-order
Triple desig_triple(void) {
    Triple t = {.z = 100, .x = 1, .y = 50};
    return t;
}
uint8_t desig_triple_x(void) { Triple t = desig_triple(); return t.x; }
uint8_t desig_triple_y(void) { Triple t = desig_triple(); return t.y; }
uint8_t desig_triple_z(void) { Triple t = desig_triple(); return t.z; }
// assert desig_triple_x() == 1 via mir2
// assert desig_triple_y() == 50 via mir2
// assert desig_triple_z() == 100 via mir2
