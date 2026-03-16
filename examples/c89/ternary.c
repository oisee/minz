// Ternary operator and comma expression test

int test_ternary_true(void) {
    return 1 ? 42 : 99;
}
// assert test_ternary_true() == 42 via mir2

int test_ternary_false(void) {
    return 0 ? 42 : 99;
}
// assert test_ternary_false() == 99 via mir2

int test_ternary_expr(void) {
    int x = 10;
    return (x > 5) ? x + 1 : x - 1;
}
// assert test_ternary_expr() == 11 via mir2

int test_ternary_nested(void) {
    int x = 2;
    return (x == 1) ? 10 : (x == 2) ? 20 : 30;
}
// assert test_ternary_nested() == 20 via mir2

// Comma operator: evaluate all, return last
int test_comma_basic(void) {
    return (1, 2, 42);
}
// assert test_comma_basic() == 42 via mir2

// Comma with side effects
int test_comma_side_effect(void) {
    int x = 0;
    int y = (x = 10, x + 5);
    return y;
}
// assert test_comma_side_effect() == 15 via mir2
