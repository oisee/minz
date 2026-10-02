// Signed arithmetic regression: VM and production PBQP Z80 must agree.
// Assert results are stored bits: i8(-2)=254, i16(-4)=65532.
#include <stdint.h>
int8_t sar(int8_t a, uint8_t k) { return a >> k; }
int16_t sar16(int16_t a, uint8_t k) { return a >> k; }
int8_t sar_one(int8_t a) { return a >> 1; }
int8_t quotient(int8_t a, int8_t b) { return a / b; }
int8_t remainder(int8_t a, int8_t b) { return a % b; }
int8_t div_two(int8_t a) { return a / (int8_t)2; }
int8_t mod_two(int8_t a) { return a % (int8_t)2; }
int16_t widen(int8_t a) { return (int16_t)a; }
uint8_t logical(uint8_t a, uint8_t k) { return a >> k; }
// assert sar(-4, 1) == 254 via mir2
// assert sar(-128, 7) == 255 via mir2
// assert sar(-7, 0) == 249 via mir2
// assert sar(127, 3) == 15 via mir2
// assert sar_one(-4) == 254 via mir2
// assert sar16(-32768, 1) == 49152 via mir2
// assert sar16(-1, 15) == 65535 via mir2
// assert sar16(32767, 8) == 127 via mir2
// assert quotient(-7, 2) == 253 via mir2
// assert quotient(7, -2) == 253 via mir2
// assert quotient(-7, -2) == 3 via mir2
// assert remainder(-7, 2) == 255 via mir2
// assert remainder(7, -2) == 1 via mir2
// assert remainder(-7, -2) == 255 via mir2
// assert div_two(-7) == 253 via mir2
// assert mod_two(-7) == 255 via mir2
// assert widen(-4) == 65532 via mir2
// assert logical(252, 1) == 126 via mir2
// assert sar(-4, 1) == 254 via z80
// assert sar(-128, 7) == 255 via z80
// assert sar(-7, 0) == 249 via z80
// assert sar(127, 3) == 15 via z80
// assert sar_one(-4) == 254 via z80
// assert sar16(-32768, 1) == 49152 via z80
// assert sar16(-1, 15) == 65535 via z80
// assert sar16(32767, 8) == 127 via z80
// assert quotient(-7, 2) == 253 via z80
// assert quotient(7, -2) == 253 via z80
// assert quotient(-7, -2) == 3 via z80
// assert remainder(-7, 2) == 255 via z80
// assert remainder(7, -2) == 1 via z80
// assert remainder(-7, -2) == 255 via z80
// assert div_two(-7) == 253 via z80
// assert mod_two(-7) == 255 via z80
// assert widen(-4) == 65532 via z80
// assert logical(252, 1) == 126 via z80
