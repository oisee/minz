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
u16 fibonacci_tail_fib_tail$u8$u16$u16(u8 n, u16 a, u16 b);
u16 fibonacci_tail_fibonacci$u8(u8 n);
void fibonacci_tail_main(void);

u16 fibonacci_tail_fib_tail$u8$u16$u16(u8 n, u16 a, u16 b) {
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
    
    r4 = n;
    r5 = 0;
    r6 = (r4 == r5);
    if (!r6) goto else_1;
    r7 = a;
    return r7;
    goto end_if_2;
else_1:
end_if_2:
    r8 = n;
    r9 = 1;
    r10 = (r8 == r9);
    if (!r10) goto else_3;
    r11 = b;
    return r11;
    goto end_if_4;
else_3:
end_if_4:
    r12 = n;
    r13 = 1;
    r14 = r12 - r13;
    r15 = b;
    r16 = a;
    r17 = b;
    r18 = r16 + r17;
    r19 = n;
    r20 = 1;
    r21 = r19 - r20;
    r22 = b;
    r23 = a;
    r24 = b;
    r25 = r23 + r24;
    r26 = fibonacci_tail_fib_tail$u8$u16$u16(r21, r22, r25);
    return r26;
}

u16 fibonacci_tail_fibonacci$u8(u8 n) {
    u32 r1 = 0;
    u32 r2 = 0;
    u32 r3 = 0;
    u32 r4 = 0;
    u32 r5 = 0;
    u32 r6 = 0;
    u32 r7 = 0;
    u32 r8 = 0;
    
    r2 = n;
    r3 = 0;
    r4 = 1;
    r5 = n;
    r6 = 0;
    r7 = 1;
    r8 = fibonacci_tail_fib_tail$u8$u16$u16(r5, r6, r7);
    return r8;
}

void fibonacci_tail_main(void) {
    u32 r1 = 0;
    u32 r2 = 0;
    u32 r3 = 0;
    u32 r4 = 0;
    
    // Local variables
    u16 result = 0;
    
    r2 = 10;
    r3 = 10;
    r4 = fibonacci_tail_fibonacci$u8(r3);
    // Skipping store to empty variable name
    return;
}

// C main wrapper
int main(int argc, char** argv) {
    fibonacci_tail_main();
    return 0;
}
