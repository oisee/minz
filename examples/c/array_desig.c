/*
 * C99 Array Designated Initializers on Z80
 * int arr[5] = {[2] = 42, [4] = 99}
 *
 * mzv: echo "" | mzv -H array_desig.c
 * CP/M: mz array_desig.c --target=cpm -o out.a80
 */
#include <stdint.h>

/* Positional init — classic C89 */
uint8_t test_positional(void) {
    uint8_t arr[4] = {10, 20, 30, 40};
    return arr[2];
}

/* Designated init — C99 */
uint8_t test_designated(void) {
    uint8_t arr[5] = {[2] = 42, [4] = 99};
    return arr[2];
}

uint8_t test_designated_last(void) {
    uint8_t arr[5] = {[2] = 42, [4] = 99};
    return arr[4];
}

/* Zeros for unspecified — C99 */
uint8_t test_zeros(void) {
    uint8_t arr[5] = {[2] = 42, [4] = 99};
    return arr[0];
}

/* Mixed positional + designated */
uint8_t test_mixed(void) {
    uint8_t arr[4] = {10, 20, [3] = 99};
    return arr[3];
}

/* Sparse: only specific slots filled */
uint8_t test_sparse_sum(void) {
    uint8_t arr[8] = {[1] = 10, [3] = 20, [7] = 30};
    return arr[1] + arr[3] + arr[7];
}

// assert test_positional() == 30 via mir2
// assert test_designated() == 42 via mir2
// assert test_designated_last() == 99 via mir2
// assert test_zeros() == 0 via mir2
// assert test_mixed() == 99 via mir2
// assert test_sparse_sum() == 60 via mir2
