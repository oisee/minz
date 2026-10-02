#include <stdint.h>
// C byte arithmetic runs at promoted int width, then truncates on return.
unsigned char add(unsigned char a, unsigned char b) { return a + b; }
uint8_t byte_add(uint8_t a, uint8_t b) { return a + b; }
uint8_t add8(uint8_t a, uint8_t b) { return a + b; }
uint8_t add_bytes(uint8_t a, uint8_t b) { return a + b; }
uint8_t swap_nibbles(uint8_t a) { uint8_t hi = a >> 4; uint8_t lo = (a << 4) & 255; return hi | lo; }
const uint8_t popcount_lut[] = {0,1,1,2,1,2,2,3,1,2,2,3,2,3,3,4};
uint8_t popcount4(uint8_t a) { return popcount_lut[a & 15]; }
uint16_t test_ld_word(uint8_t lo, uint8_t hi) { return lo | ((uint16_t)hi << 8); }
uint8_t promoted_mod(uint8_t a) { return a % 2; }
int16_t pressure(int16_t a,int16_t b,int16_t c,int16_t d) {
 int16_t x=a+b, y=c-d, z=a-d, w=b+c;
 return x/y + z%w;
}
// assert add(3,4) == 7 via mir2
// assert byte_add(200,55) == 255 via mir2
// assert add8(200,55) == 255 via mir2
// assert add_bytes(100,42) == 142 via mir2
// assert swap_nibbles(18) == 33 via mir2
// assert popcount4(0) == 0 via mir2
// assert popcount4(5) == 2 via mir2
// assert popcount4(15) == 4 via mir2
// assert test_ld_word(52,18) == 4660 via mir2
// assert test_ld_word(255,255) == 65535 via mir2
// assert promoted_mod(255) == 1 via mir2
// assert pressure(20,4,10,2) == 7 via mir2
// assert add(3,4) == 7 via z80
// assert byte_add(200,55) == 255 via z80
// assert add8(200,55) == 255 via z80
// assert add_bytes(100,42) == 142 via z80
// assert swap_nibbles(18) == 33 via z80
// assert popcount4(0) == 0 via z80
// assert popcount4(5) == 2 via z80
// assert popcount4(15) == 4 via z80
// assert test_ld_word(52,18) == 4660 via z80
// assert test_ld_word(255,255) == 65535 via z80
// assert promoted_mod(255) == 1 via z80
// assert pressure(20,4,10,2) == 7 via z80
