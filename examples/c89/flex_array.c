// C99 flexible array members test

typedef struct {
    int len;
    int data[];  // flexible array member
} FlexArray;

// sizeof should not include the flexible member
int test_sizeof_flex(void) {
    return sizeof(FlexArray);
}
// assert test_sizeof_flex() == 2 via mir2
