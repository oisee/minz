// MinZ C generated code
// Generated: 2025-08-06 11:42:14
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
void test_stdlib_simple_print_char$u8(u8 ch);
void test_stdlib_simple_print_u8$u8(u8 value);
void test_stdlib_simple_print_newline(void);
void test_stdlib_simple_main(void);

void test_stdlib_simple_print_char$u8(u8 ch) {
    return;
}

void test_stdlib_simple_print_u8$u8(u8 value) {
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
    u32 r38 = 0;
    u32 r39 = 0;
    u32 r40 = 0;
    u32 r41 = 0;
    u32 r42 = 0;
    u32 r43 = 0;
    u32 r44 = 0;
    
    // Local variables
    u8 temp = 0;
    
    r3 = value;
    // Skipping store to empty variable name
    r4 = temp;
    r5 = 100;
    r6 = (r4 >= r5);
    if (!r6) goto else_1;
    r7 = 48;
    r8 = temp;
    r9 = 100;
    r10 = r8 / r9;
    r11 = r7 + r10;
    r12 = 48;
    r13 = temp;
    r14 = 100;
    r15 = r13 / r14;
    r16 = r12 + r15;
    test_stdlib_simple_print_char$u8(r16);
    r17 = 0; // void function call result
    r18 = temp;
    r19 = 100;
    r20 = r18 % r19;
    temp = r20;
    goto end_if_2;
else_1:
end_if_2:
    r21 = temp;
    r22 = 10;
    r23 = (r21 >= r22);
    if (!r23) goto else_3;
    r24 = 48;
    r25 = temp;
    r26 = 10;
    r27 = r25 / r26;
    r28 = r24 + r27;
    r29 = 48;
    r30 = temp;
    r31 = 10;
    r32 = r30 / r31;
    r33 = r29 + r32;
    test_stdlib_simple_print_char$u8(r33);
    r34 = 0; // void function call result
    r35 = temp;
    r36 = 10;
    r37 = r35 % r36;
    temp = r37;
    goto end_if_4;
else_3:
end_if_4:
    r38 = 48;
    r39 = temp;
    r40 = r38 + r39;
    r41 = 48;
    r42 = temp;
    r43 = r41 + r42;
    test_stdlib_simple_print_char$u8(r43);
    r44 = 0; // void function call result
    return;
}

void test_stdlib_simple_print_newline(void) {
    u32 r1 = 0;
    u32 r2 = 0;
    u32 r3 = 0;
    u32 r4 = 0;
    u32 r5 = 0;
    u32 r6 = 0;
    
    r1 = 13;
    r2 = 13;
    test_stdlib_simple_print_char$u8(r2);
    r3 = 0; // void function call result
    r4 = 10;
    r5 = 10;
    test_stdlib_simple_print_char$u8(r5);
    r6 = 0; // void function call result
    return;
}

void test_stdlib_simple_main(void) {
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
    
    r1 = 0;
    r2 = 0;
    test_stdlib_simple_print_u8$u8(r2);
    r3 = 0; // void function call result
    test_stdlib_simple_print_newline();
    r4 = 0; // void function call result
    r5 = 42;
    r6 = 42;
    test_stdlib_simple_print_u8$u8(r6);
    r7 = 0; // void function call result
    test_stdlib_simple_print_newline();
    r8 = 0; // void function call result
    r9 = 123;
    r10 = 123;
    test_stdlib_simple_print_u8$u8(r10);
    r11 = 0; // void function call result
    test_stdlib_simple_print_newline();
    r12 = 0; // void function call result
    r13 = 255;
    r14 = 255;
    test_stdlib_simple_print_u8$u8(r14);
    r15 = 0; // void function call result
    test_stdlib_simple_print_newline();
    r16 = 0; // void function call result
    return;
}

// C main wrapper
int main(int argc, char** argv) {
    test_stdlib_simple_main();
    return 0;
}
