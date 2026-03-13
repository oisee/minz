#include <stdint.h>

uint8_t abs_diff(uint8_t a, uint8_t b) {
    if (a >= b) return a - b;
    return b - a;
}

uint8_t main(void) {
    return abs_diff(10, 3);
}
