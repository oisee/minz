// Test #include with local headers and system libc
#include <string.h>
#include <stdint.h>
#include <stddef.h>
#include <limits.h>
#include "helper.h"

int test_local_include(void) {
    return helper_add(10, 20);
}
// assert test_local_include() == 30 via mir2

// string.h declares memcmp — just verify it compiles
// (we can't call memcmp yet without a runtime, but the decl should parse)
int test_sizeof_size_t(void) {
    return sizeof(size_t);
}
// assert test_sizeof_size_t() == 2 via mir2

// stdint.h types
int test_stdint_sizes(void) {
    return sizeof(uint8_t) + sizeof(uint16_t) + sizeof(uint32_t);
}
// assert test_stdint_sizes() == 7 via mir2

// limits.h constants
int test_int_max(void) {
    return INT_MAX;
}
// assert test_int_max() == 32767 via mir2
