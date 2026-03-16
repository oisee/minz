// Type casting test

int test_cast_to_char(void) {
    int x = 300;
    return (char)x;  // truncate to 8 bits: 300 = 0x12C → 0x2C = 44
}
// assert test_cast_to_char() == 44 via mir2

int test_cast_to_int(void) {
    char c = 65;  // 'A'
    return (int)c;
}
// assert test_cast_to_int() == 65 via mir2

int test_cast_noop(void) {
    int x = 42;
    return (int)x;
}
// assert test_cast_noop() == 42 via mir2

int test_cast_unsigned(void) {
    unsigned char u = 200;
    return (int)u;
}
// assert test_cast_unsigned() == 200 via mir2
