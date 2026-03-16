// sizeof operator test (Z80: int=2, char=1, long=4, pointer=2)

int test_sizeof_int(void) {
    return sizeof(int);
}
// assert test_sizeof_int() == 2 via mir2

int test_sizeof_char(void) {
    return sizeof(char);
}
// assert test_sizeof_char() == 1 via mir2

int test_sizeof_long(void) {
    return sizeof(long);
}
// assert test_sizeof_long() == 4 via mir2

int test_sizeof_ptr(void) {
    return sizeof(int *);
}
// assert test_sizeof_ptr() == 2 via mir2

int test_sizeof_expr(void) {
    int x = 42;
    return sizeof(x);
}
// assert test_sizeof_expr() == 2 via mir2

typedef struct { int x; int y; } SzPoint;

int test_sizeof_struct(void) {
    return sizeof(SzPoint);
}
// assert test_sizeof_struct() == 4 via mir2
