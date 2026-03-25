#include <stdint.h>

// C pattern: minmax via output pointers
void minmax(uint8_t a, uint8_t b, uint8_t *out_min, uint8_t *out_max) {
    if (a <= b) { *out_min = a; *out_max = b; }
    else        { *out_min = b; *out_max = a; }
}

uint8_t get_min(uint8_t a, uint8_t b) {
    uint8_t mn, mx;
    minmax(a, b, &mn, &mx);
    return mn;
}

int main(void) { return get_min(10, 3); }
