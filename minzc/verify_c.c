// MinZ C generated code
// Generated: 2025-08-06 18:02:49
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
u8 verify_load_index_test_byte_indexing$u8(u8 idx);
u16 verify_load_index_test_word_indexing$u8(u8 idx);
u8 verify_load_index_test_dynamic_indexing(void);
u8 verify_load_index_main(void);

// Global variables
u8* verify_load_index.byte_array;
u16* verify_load_index.word_array;

u8 verify_load_index_test_byte_indexing$u8(u8 idx) {
    u32 r1 = 0;
    u32 r2 = 0;
    u32 r3 = 0;
    u32 r4 = 0;
    
    r2 = (u32)&verify_load_index.byte_array;
    r3 = idx;
    r4 = ((u8*)r2)[r3];
    return r4;
}

u16 verify_load_index_test_word_indexing$u8(u8 idx) {
    u32 r1 = 0;
    u32 r2 = 0;
    u32 r3 = 0;
    u32 r4 = 0;
    
    r2 = (u32)&verify_load_index.word_array;
    r3 = idx;
    r4 = ((u16*)r2)[r3];
    return r4;
}

u8 verify_load_index_test_dynamic_indexing(void) {
    u32 r1 = 0;
    u32 r2 = 0;
    u32 r3 = 0;
    u32 r4 = 0;
    u32 r5 = 0;
    u32 r6 = 0;
    u32 r7 = 0;
    
    // Local variables
    u8 i = 0;
    u8 result = 0;
    
    r2 = 2;
    i = r2;
    r4 = (u32)&verify_load_index.byte_array;
    r5 = i;
    r6 = ((u8*)r4)[r5];
    result = r6;
    r7 = result;
    return r7;
}

u8 verify_load_index_main(void) {
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
    
    // Local variables
    u8 byte_val = 0;
    u16 word_val = 0;
    u8 dynamic_val = 0;
    
    r2 = 1;
    r3 = 1;
    r4 = verify_load_index_test_byte_indexing$u8(r3);
    byte_val = r4;
    r6 = 0;
    r7 = 0;
    r8 = verify_load_index_test_word_indexing$u8(r7);
    word_val = r8;
    r10 = verify_load_index_test_dynamic_indexing();
    dynamic_val = r10;
    r11 = byte_val;
    return r11;
}

// C main wrapper
int main(int argc, char** argv) {
    return (int)verify_load_index_main();
}
