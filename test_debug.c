// MinZ C generated code
// Generated: 2025-08-06 23:02:19
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
void examples_test_print_all_backends_print_test(void);
u8 examples_test_print_all_backends_main(void);

// String literals
const char str_0[] = "=== MinZ Print Test ===\n";
const char str_1[] = "Testing all backends!\n\n";
const char str_2[] = "Numbers: ";
const char str_3[] = "=== Test Complete ===\n";

void examples_test_print_all_backends_print_test(void) {
    uintptr_t r1 = 0;
    uintptr_t r2 = 0;
    uintptr_t r3 = 0;
    uintptr_t r4 = 0;
    uintptr_t r5 = 0;
    uintptr_t r6 = 0;
    uintptr_t r7 = 0;
    uintptr_t r8 = 0;
    uintptr_t r9 = 0;
    uintptr_t r10 = 0;
    uintptr_t r11 = 0;
    uintptr_t r12 = 0;
    uintptr_t r13 = 0;
    uintptr_t r14 = 0;
    uintptr_t r15 = 0;
    
    // Local variables
    u8 x = 0;
    u16 y = 0;
    u8 ch = 0;
    bool flag = 0;
    
    r1 = (uintptr_t)str_0;
    printf("%s", (const char*)r1);
    r2 = (uintptr_t)str_1;
    printf("%s", (const char*)r2);
    r3 = (uintptr_t)str_2;
    printf("%s", (const char*)r3);
    r5 = 42;
    x = r5;
    r6 = x;
    printf("%u", (unsigned)r6);
    printf(", ");
    r8 = 1234;
    y = r8;
    r9 = y;
    printf("%u", (unsigned)r9);
    printf("\n");
    printf("Chars: ");
    r11 = 65;
    ch = r11;
    printf(" ");
    printf("\n");
    r13 = 1;
    flag = r13;
    printf("Bool: ");
    r14 = flag;
    printf("%s", r14 ? "true" : "false");
    printf("\n");
    r15 = (uintptr_t)str_3;
    printf("%s", (const char*)r15);
    return;
}

u8 examples_test_print_all_backends_main(void) {
    uintptr_t r1 = 0;
    uintptr_t r2 = 0;
    
    examples_test_print_all_backends_print_test();
    r1 = 0; // void function call result
    r2 = 0;
    return r2;
}

// C main wrapper
int main(int argc, char** argv) {
    return (int)examples_test_print_all_backends_main();
}
