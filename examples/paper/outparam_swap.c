#include <stdint.h>

// C pattern: swap via output pointers (SDCC can't return multiple values)
void swap_out(uint8_t a, uint8_t b, uint8_t *out_a, uint8_t *out_b) {
    *out_a = b;
    *out_b = a;
}

// Caller must allocate stack space for outputs
uint8_t use_swap(uint8_t x, uint8_t y) {
    uint8_t a, b;
    swap_out(x, y, &a, &b);
    return a + b;
}

int main(void) { return use_swap(3, 7); }
