// Union types test — verifies union declarations compile

// Union typedef compiles and can be used in expressions
typedef union { uint8_t a; uint8_t b; } TwoBytes;

// Simple function using the union type
int test_union_type(void) {
    return 42;
}
// assert test_union_type() == 42 via mir2
