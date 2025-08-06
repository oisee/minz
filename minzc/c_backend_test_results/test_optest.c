// MinZ C generated code
// Generated: 2025-08-06 11:42:12
// Target: Standard C (C99)

#include <stdio.h>
#include <stdint.h>
#include <stdbool.h>
#include <stdlib.h>
#include <string.h>

// Type definitions
typedef uint8_t u8;
typedef uint16_t u16;
typedef uint32_t u24; // 24-bit emulated as 32-bit
typedef uint32_t u32;
typedef int8_t i8;
typedef int16_t i16;
typedef int32_t i24; // 24-bit emulated as 32-bit
typedef int32_t i32;

// Fixed-point arithmetic helpers
typedef int16_t f8_8;   // 8.8 fixed-point
typedef int16_t f_8;    // .8 fixed-point
typedef int16_t f_16;   // .16 fixed-point
typedef int32_t f16_8;  // 16.8 fixed-point
typedef int32_t f8_16;  // 8.16 fixed-point

#define F8_8_SHIFT 8
#define F_8_SHIFT 8
#define F_16_SHIFT 16
#define F16_8_SHIFT 8
#define F8_16_SHIFT 16

// String type (length-prefixed)
typedef struct {
    uint16_t len;
    char* data;
} String;

// Print helper functions
void print_char(u8 ch) {
    putchar(ch);
}

void print_u8(u8 value) {
    printf("%u", value);
}

void print_u8_decimal(u8 value) {
    printf("%u", value);
}

void print_u16(u16 value) {
    printf("%u", value);
}

void print_u24(u24 value) {
    printf("%u", value);
}

void print_i8(i8 value) {
    printf("%d", value);
}

void print_i16(i16 value) {
    printf("%d", value);
}

void print_newline() {
    printf("\n");
}

void print_string(String* str) {
    if (str && str->data) {
        printf("%.*s", str->len, str->data);
    }
}

// Function declarations
bool examples_test_optest_test_zero_comparison$u8(u8 x);
bool examples_test_optest_test_nonzero_check$u16(u16 x);
u8 examples_test_optest_test_conditional_assignment$u8(u8 x);
void examples_test_optest_main(void);

bool examples_test_optest_test_zero_comparison$u8(u8 x) {
    u32 r1 = 0;
    u32 r2 = 0;
    u32 r3 = 0;
    u32 r4 = 0;
    u32 r5 = 0;
    u32 r6 = 0;
    
    r2 = x;
    r3 = 0;
    r4 = (r2 == r3);
    if (!r4) goto else_1;
    r5 = 1;
    return r5;
    goto end_if_2;
else_1:
end_if_2:
    r6 = 0;
    return r6;
}

bool examples_test_optest_test_nonzero_check$u16(u16 x) {
    u32 r1 = 0;
    u32 r2 = 0;
    u32 r3 = 0;
    u32 r4 = 0;
    u32 r5 = 0;
    u32 r6 = 0;
    
    r2 = x;
    r3 = 0;
    r4 = (r2 != r3);
    if (!r4) goto else_3;
    r5 = 1;
    return r5;
    goto end_if_4;
else_3:
end_if_4:
    r6 = 0;
    return r6;
}

u8 examples_test_optest_test_conditional_assignment$u8(u8 x) {
    u32 r1 = 0;
    u32 r2 = 0;
    u32 r3 = 0;
    u32 r4 = 0;
    u32 r5 = 0;
    u32 r6 = 0;
    u32 r7 = 0;
    u32 r8 = 0;
    
    // Local variables
    u8 result = 0;
    
    r3 = x;
    r4 = 0;
    r5 = (r3 != r4);
    if (!r5) goto else_5;
    r6 = 10;
    result = r6;
    goto end_if_6;
else_5:
    r7 = 20;
    result = r7;
end_if_6:
    r8 = result;
    return r8;
}

void examples_test_optest_main(void) {
    u32 r1 = 0;
    u32 r2 = 0;
    u32 r3 = 0;
    u32 r4 = 0;
    u32 r5 = 0;
    u32 r6 = 0;
    u32 r7 = 0;
    u32 r8 = 0;
    u32 r9 = 0;
    u32 r10 = 0;
    u32 r11 = 0;
    u32 r12 = 0;
    u32 r13 = 0;
    u32 r14 = 0;
    u32 r15 = 0;
    u32 r16 = 0;
    u32 r17 = 0;
    u32 r18 = 0;
    u32 r19 = 0;
    u32 r20 = 0;
    u32 r21 = 0;
    u32 r22 = 0;
    u32 r23 = 0;
    u32 r24 = 0;
    
    // Local variables
    u16 a = 0;
    u16 b = 0;
    u16 c = 0;
    u16 d = 0;
    u16 e = 0;
    u16 f = 0;
    
    r2 = 0;
    r3 = 0;
    r4 = examples_test_optest_test_zero_comparison$u8(r3);
    // Skipping store to empty variable name
    r6 = 5;
    r7 = 5;
    r8 = examples_test_optest_test_zero_comparison$u8(r7);
    // Skipping store to empty variable name
    r10 = 0;
    r11 = 0;
    r12 = examples_test_optest_test_nonzero_check$u16(r11);
    // Skipping store to empty variable name
    r14 = 100;
    r15 = 100;
    r16 = examples_test_optest_test_nonzero_check$u16(r15);
    // Skipping store to empty variable name
    r18 = 0;
    r19 = 0;
    r20 = examples_test_optest_test_conditional_assignment$u8(r19);
    // Skipping store to empty variable name
    r22 = 1;
    r23 = 1;
    r24 = examples_test_optest_test_conditional_assignment$u8(r23);
    // Skipping store to empty variable name
    return;
}

// C main wrapper
int main(int argc, char** argv) {
    examples_test_optest_main();
    return 0;
}
