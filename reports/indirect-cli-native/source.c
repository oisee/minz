// Function pointers test — C89 indirect calls

int add(int a, int b) { return a + b; }
int sub(int a, int b) { return a - b; }
int double_it(int x) { return x + x; }
int negate(int x) { return 0 - x; }

// Basic function pointer: assign and call
int test_basic_fp(void) {
    int (*fp)(int, int) = add;
    return fp(3, 4);
}
// assert test_basic_fp() == 7 via mir2

// Reassign function pointer
int test_reassign_fp(void) {
    int (*fp)(int, int) = add;
    int a = fp(10, 5);
    fp = sub;
    int b = fp(10, 5);
    return a + b;
}
// assert test_reassign_fp() == 20 via mir2

// Function pointer with explicit &
int test_addr_of_func(void) {
    int (*fp)(int, int) = &add;
    return fp(6, 7);
}
// assert test_addr_of_func() == 13 via mir2

// Unary function pointer
int test_unary_fp(void) {
    int (*fp)(int) = double_it;
    return fp(21);
}
// assert test_unary_fp() == 42 via mir2

// Function pointer passed as argument (callback pattern)
int apply(int (*fn)(int), int x) {
    return fn(x);
}

int test_callback(void) {
    return apply(double_it, 10);
}
// assert test_callback() == 20 via mir2

int test_callback_negate(void) {
    return apply(negate, 7);
}
// assert test_callback_negate() == 65529 via mir2
