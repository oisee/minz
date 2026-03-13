#include <stdint.h>

uint16_t abs_diff_u16(uint16_t a, uint16_t b) {
    if (a < b) return b - a;
    return a - b;
}
