#include <stdint.h>
void swap_out(uint8_t a, uint8_t b, uint8_t *out_a, uint8_t *out_b) {
    *out_a = b;
    *out_b = a;
}
uint8_t use_swap(uint8_t x, uint8_t y) {
    uint8_t a, b;
    swap_out(x, y, &a, &b);
    return a + b;
}
int main(void) { return use_swap(3, 7); }
