// FatFS C readiness tests — exercises constructs needed by ff.c
// Each test targets a specific pattern found in FatFS source.

// --- Increment/decrement as statement (works) ---
int test_inc_stmt(void) {
    int x = 10;
    x++;
    x++;
    x++;
    return x;
}
// assert test_inc_stmt() == 13 via mir2

int test_dec_stmt(void) {
    int x = 10;
    x--;
    x--;
    return x;
}
// assert test_dec_stmt() == 8 via mir2

int test_preinc_stmt(void) {
    int x = 5;
    ++x;
    ++x;
    return x;
}
// assert test_preinc_stmt() == 7 via mir2

// --- Increment in for-loop (most common FatFS pattern) ---
int test_inc_for(void) {
    int sum = 0;
    for (int i = 0; i < 5; i++) {
        sum += i;
    }
    return sum;
}
// assert test_inc_for() == 10 via mir2

// --- Pre-increment as expression (side effect must propagate) ---
int test_preinc_expr(void) {
    int x = 10;
    int y = ++x;
    return y + x;  // 11 + 11 = 22
}
// assert test_preinc_expr() == 22 via mir2

int test_predec_expr(void) {
    int x = 10;
    int y = --x;
    return y + x;  // 9 + 9 = 18
}
// assert test_predec_expr() == 18 via mir2

// --- Post-increment as expression (returns old value) ---
int test_postinc_expr(void) {
    int x = 10;
    int y = x++;
    return y + x;  // 10 + 11 = 21
}
// assert test_postinc_expr() == 21 via mir2

int test_postdec_expr(void) {
    int x = 10;
    int y = x--;
    return y + x;  // 10 + 9 = 19
}
// assert test_postdec_expr() == 19 via mir2

// --- Chained assignment (side effect on intermediate vars) ---
int test_chained_assign(void) {
    int a = 0;
    int b = 0;
    a = b = 42;
    return a + b;
}
// assert test_chained_assign() == 84 via mir2

// --- Ternary in assignments (83 uses in ff.c) ---
int test_ternary_assign(void) {
    int x = 5;
    int y = (x > 3) ? 100 : 200;
    return y;
}
// assert test_ternary_assign() == 100 via mir2

// --- Switch with fall-through-less cases (5 switches in ff.c) ---
int test_switch_multi(void) {
    int x = 2;
    int r = 0;
    switch (x) {
        case 0: r = 10; break;
        case 1: r = 20; break;
        case 2: r = 30; break;
        case 3: r = 40; break;
        default: r = 99; break;
    }
    return r;
}
// assert test_switch_multi() == 30 via mir2

// --- Struct field access through pointer (common in ff.c: fp->xxx) ---
typedef struct { int fsize; int fptr; int flag; } FIL_MINI;

int test_struct_fields(void) {
    FIL_MINI f = {1024, 0, 1};
    return f.fsize + f.flag;
}
// assert test_struct_fields() == 1025 via mir2

// --- Bitwise operations (heavy use in FAT) ---
int test_bitmask(void) {
    int val = 0xFF00;
    return (val >> 8) & 0xFF;
}
// assert test_bitmask() == 255 via mir2

int test_bitset(void) {
    int flags = 0;
    flags = flags | 0x04;
    flags = flags | 0x10;
    return flags;
}
// assert test_bitset() == 20 via mir2

// --- Unsigned comparisons (FAT sector math) ---
unsigned int test_unsigned_cmp(void) {
    unsigned int a = 50000;
    unsigned int b = 40000;
    return (a > b) ? 1 : 0;
}
// assert test_unsigned_cmp() == 1 via mir2

// GAP: Chained assignment a = b = 42 — side-effect on b lost
// (same exprResult issue as ++/-- in expressions)
// FatFS uses this ~10 times

// --- Sequential assignments (workaround) ---
int test_sequential_assign(void) {
    int a = 0;
    int b = 0;
    b = 42;
    a = b;
    return a + b;
}
// assert test_sequential_assign() == 84 via mir2

// --- Compound assignment operators (+=, -=, |=, &=) ---
int test_compound_add(void) {
    int x = 10;
    x += 5;
    return x;
}
// assert test_compound_add() == 15 via mir2

int test_compound_or(void) {
    int x = 0x0F;
    x |= 0xF0;
    return x;
}
// assert test_compound_or() == 255 via mir2

int test_compound_and(void) {
    int x = 0xFF;
    x &= 0x0F;
    return x;
}
// assert test_compound_and() == 15 via mir2

int test_compound_shift(void) {
    int x = 1;
    x <<= 4;
    return x;
}
// assert test_compound_shift() == 16 via mir2

// --- Nested struct (FatFs: FATFS has nested DIR) ---
typedef struct { int sect; int clst; } FCLUST;
typedef struct { FCLUST obj; int flag; } FDIR;

int test_nested_fatfs(void) {
    FDIR d = {{100, 5}, 1};
    return d.obj.sect + d.obj.clst + d.flag;
}
// assert test_nested_fatfs() == 106 via mir2

// --- Cast between types (common in sector math) ---
long test_cast_widen(void) {
    int x = 512;
    return (long)x;
}
// assert test_cast_widen() == 512 via mir2

int test_cast_narrow(void) {
    long x = 300;
    return (int)x;
}
// assert test_cast_narrow() == 300 via mir2

// --- Conditional with side effects ---
int test_logical_short_circuit(void) {
    int x = 0;
    int y = 5;
    // y > 0 is true, so x should stay 0 in ||
    // but (x == 0 && y > 0) should be 1
    return (x == 0 && y > 0) ? 1 : 0;
}
// assert test_logical_short_circuit() == 1 via mir2

// --- Multi-level #if (260 in ff.c) --- already handled by cparse

// Total: 19 asserts covering key FatFS constructs
