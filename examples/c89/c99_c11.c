// C99/C11 features test

// --- inline functions (C99) ---
// inline is accepted and ignored (function is always emitted)
inline int square(int x) { return x * x; }
// assert square(7) == 49 via mir2

static inline int cube(int x) { return x * x * x; }
// assert cube(3) == 27 via mir2

// --- _Bool (C99) ---
_Bool is_positive(int x) {
    return x > 0;
}
// assert is_positive(5) == 1 via mir2
// assert is_positive(0) == 0 via mir2

_Bool bool_normalize(int x) {
    // In C99, assigning 42 to _Bool should yield 1
    _Bool b = x;
    return b;
}
// assert bool_normalize(42) == 1 via mir2
// assert bool_normalize(0) == 0 via mir2

// --- _Static_assert (C11) ---
_Static_assert(1, "truth holds");
_Static_assert(sizeof(int) == 2, "int is 16-bit on Z80");

// --- restrict (C99) ---
// restrict is accepted and ignored
int copy_val(int * restrict dst, int * restrict src) {
    *dst = *src;
    return *dst;
}

// --- _Noreturn (C11) ---
// Accepted and ignored (annotation only)
// _Noreturn void halt(void) { while(1) {} }

// --- Designated initializers in function scope (C99) ---
typedef struct { int x; int y; int z; } Vec3;

int test_desig_local(void) {
    Vec3 v = {.z = 30, .x = 10, .y = 20};
    return v.x + v.y + v.z;
}
// assert test_desig_local() == 60 via mir2

// --- Compound literals in expressions (C99) ---
int test_compound_expr(void) {
    return (int){99};
}
// assert test_compound_expr() == 99 via mir2

// --- for-loop declarations (C99) ---
int test_for_decl(void) {
    int sum = 0;
    for (int i = 0; i < 5; i = i + 1) {
        sum = sum + i;
    }
    return sum;
}
// assert test_for_decl() == 10 via mir2

// --- Mixed declarations and code (C99) ---
int test_mixed_decls(void) {
    int a = 1;
    a = a + 1;
    int b = a + 10;  // declaration after statement
    return b;
}
// assert test_mixed_decls() == 12 via mir2

// --- // single-line comments (C99) ---
// This is a C99-style comment (already works)
int test_comments(void) { return 1; }
// assert test_comments() == 1 via mir2

// --- _Generic (C11) ---
int test_generic_int(void) {
    int x = 42;
    return _Generic(x, int: 1, long: 2, default: 0);
}
// assert test_generic_int() == 1 via mir2

int test_generic_long(void) {
    long x = 100;
    return _Generic(x, int: 1, long: 2, default: 0);
}
// assert test_generic_long() == 2 via mir2

int test_generic_default(void) {
    char x = 'A';
    return _Generic(x, int: 1, long: 2, default: 3);
}
// assert test_generic_default() == 3 via mir2
