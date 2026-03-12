#include <stdint.h>

uint8_t gcd(uint8_t a, uint8_t b) {
    while (a != b) {
        if (a > b) a = a - b;
        else       b = b - a;
    }
    return a;
}
