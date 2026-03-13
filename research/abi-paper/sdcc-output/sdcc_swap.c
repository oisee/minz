#include <stdint.h>

// SDCC can't return structs — must use out-params or globals
void swap(uint16_t a, uint16_t b, uint16_t *out_a, uint16_t *out_b) {
    *out_a = b;
    *out_b = a;
}
