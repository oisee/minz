#include <stdint.h>

uint8_t sum_array(uint8_t *buf, uint8_t n) {
    uint8_t s = 0;
    for (uint8_t i = 0; i < n; i++) {
        s += buf[i];
    }
    return s;
}

uint8_t max_array(uint8_t *buf, uint8_t n) {
    uint8_t m = 0;
    for (uint8_t i = 0; i < n; i++) {
        if (buf[i] > m) m = buf[i];
    }
    return m;
}
