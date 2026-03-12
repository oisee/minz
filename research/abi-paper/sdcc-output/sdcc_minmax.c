#include <stdint.h>

void minmax(uint16_t a, uint16_t b, uint16_t *lo, uint16_t *hi) {
    if (a <= b) { *lo = a; *hi = b; }
    else        { *lo = b; *hi = a; }
}

uint16_t min_of(uint16_t a, uint16_t b) {
    uint16_t lo, hi;
    minmax(a, b, &lo, &hi);
    return lo;
}
