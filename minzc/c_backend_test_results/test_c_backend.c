// MinZ C generated code
// Generated: 2025-08-06 11:42:13
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
void test_c_backend_main(void);
u8 test_c_backend_add$u8$u8(u8 a, u8 b);
void test_c_backend_print_char$u8(u8 ch);
void test_c_backend_print_u8$u8(u8 value);
void test_c_backend_print_newline(void);

void test_c_backend_main(void) {
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
    u32 r25 = 0;
    u32 r26 = 0;
    u32 r27 = 0;
    u32 r28 = 0;
    u32 r29 = 0;
    u32 r30 = 0;
    u32 r31 = 0;
    u32 r32 = 0;
    u32 r33 = 0;
    u32 r34 = 0;
    u32 r35 = 0;
    u32 r36 = 0;
    u32 r37 = 0;
    
    // Local variables
    u8 x = 0;
    u8 y = 0;
    u8 sum = 0;
    u8 i = 0;
    u8 result = 0;
    
    r2 = 10;
    // Skipping store to empty variable name
    r4 = 20;
    // Skipping store to empty variable name
    r6 = x;
    r7 = y;
    r8 = r6 + r7;
    // Skipping store to empty variable name
    r9 = sum;
    r10 = sum;
    test_c_backend_print_u8$u8(r10);
    r11 = 0; // void function call result
    test_c_backend_print_newline();
    r12 = 0; // void function call result
    r14 = 0;
    // Skipping store to empty variable name
loop_1:
    r15 = i;
    r16 = 5;
    r17 = (r15 < r16);
    if (!r17) goto end_loop_2;
    r18 = i;
    r19 = i;
    test_c_backend_print_u8$u8(r19);
    r20 = 0; // void function call result
    r21 = 32;
    r22 = 32;
    test_c_backend_print_char$u8(r22);
    r23 = 0; // void function call result
    r24 = i;
    r25 = 1;
    r26 = r24 + r25;
    i = r26;
    goto loop_1;
end_loop_2:
    test_c_backend_print_newline();
    r27 = 0; // void function call result
    r29 = 15;
    r30 = 25;
    r31 = 15;
    r32 = 25;
    r33 = test_c_backend_add$u8$u8(r31, r32);
    // Skipping store to empty variable name
    r34 = result;
    r35 = result;
    test_c_backend_print_u8$u8(r35);
    r36 = 0; // void function call result
    test_c_backend_print_newline();
    r37 = 0; // void function call result
    return;
}

u8 test_c_backend_add$u8$u8(u8 a, u8 b) {
    u32 r1 = 0;
    u32 r2 = 0;
    u32 r3 = 0;
    u32 r4 = 0;
    u32 r5 = 0;
    
    r3 = a;
    r4 = b;
    r5 = r3 + r4;
    return r5;
}

void test_c_backend_print_char$u8(u8 ch) {
    return;
}

void test_c_backend_print_u8$u8(u8 value) {
    return;
}

void test_c_backend_print_newline(void) {
    return;
}

// C main wrapper
int main(int argc, char** argv) {
    test_c_backend_main();
    return 0;
}
