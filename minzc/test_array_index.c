// MinZ C generated code
// Generated: 2025-08-06 18:01:15
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
u8 test_array_index_test_array_index(void);
u8 test_array_index_main(void);

// Global variables
u8* test_array_index.numbers;

u8 test_array_index_test_array_index(void) {
    u32 r1 = 0;
    u32 r2 = 0;
    u32 r3 = 0;
    u32 r4 = 0;
    u32 r5 = 0;
    u32 r6 = 0;
    u32 r7 = 0;
    
    // Local variables
    u8 idx = 0;
    u8 value = 0;
    
    r2 = 2;
    idx = r2;
    r4 = (u32)&test_array_index.numbers;
    r5 = idx;
    r6 = ((u8*)r4)[r5];
    value = r6;
    r7 = value;
    return r7;
}

u8 test_array_index_main(void) {
    u32 r1 = 0;
    u32 r2 = 0;
    u32 r3 = 0;
    
    // Local variables
    u8 result = 0;
    
    r2 = test_array_index_test_array_index();
    result = r2;
    r3 = result;
    return r3;
}

// C main wrapper
int main(int argc, char** argv) {
    return (int)test_array_index_main();
}
