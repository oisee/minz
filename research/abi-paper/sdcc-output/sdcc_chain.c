#include <stdint.h>

uint8_t leaf(uint8_t x) {
    return x + x;
}

uint8_t middle(uint8_t y) {
    return leaf(y);
}

uint8_t top(uint8_t z) {
    return middle(z);
}

uint8_t main(void) {
    return top(5);
}
