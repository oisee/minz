// typeof (GCC extension, widely supported) test

int test_typeof_basic(void) {
    int x = 42;
    typeof(x) y = x + 1;
    return y;
}
// assert test_typeof_basic() == 43 via mir2

long test_typeof_long(void) {
    long a = 1000;
    typeof(a) b = 234;
    return b;
}
// assert test_typeof_long() == 234 via mir2
