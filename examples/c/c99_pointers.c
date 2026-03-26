/*
 * C99 Pointer operations — Z80 16-bit pointer testing
 *
 * mzv: echo "" | mzv -H c99_pointers.c
 */
#include <stdint.h>
#include <stdbool.h>

/* Basic pointer read/write */
uint8_t deref_test(void) {
    uint8_t val = 42;
    uint8_t *p = &val;
    return *p;
}

uint8_t write_via_ptr(void) {
    uint8_t val = 0;
    uint8_t *p = &val;
    *p = 99;
    return val;
}

/* Pointer arithmetic with arrays */
uint8_t array_via_ptr(void) {
    uint8_t arr[4] = {10, 20, 30, 40};
    uint8_t *p = arr;
    return *(p + 2);
}

uint8_t ptr_index(void) {
    uint8_t arr[4] = {10, 20, 30, 40};
    uint8_t *p = arr;
    return p[3];
}

/* Swap via pointers */
uint8_t swap_test(void) {
    uint8_t a = 10, b = 20;
    uint8_t *pa = &a, *pb = &b;
    uint8_t tmp = *pa;
    *pa = *pb;
    *pb = tmp;
    return a;
}

/* Sizeof pointer — always 2 on Z80 */
uint8_t ptr_size(void) {
    return sizeof(uint8_t *);
}

/* Function pointer */
uint8_t apply(uint8_t (*fn)(uint8_t), uint8_t x) {
    return fn(x);
}

uint8_t double_val(uint8_t x) { return x * 2; }
uint8_t inc_val(uint8_t x) { return x + 1; }

uint8_t test_fptr(void) {
    return apply(double_val, 21);
}

uint8_t test_fptr2(void) {
    return apply(inc_val, 41);
}

// assert deref_test() == 42 via mir2
// assert write_via_ptr() == 99 via mir2
// assert array_via_ptr() == 30 via mir2
// assert ptr_index() == 40 via mir2
// assert swap_test() == 20 via mir2
// assert ptr_size() == 2 via mir2
// assert test_fptr() == 42 via mir2
// assert test_fptr2() == 42 via mir2
