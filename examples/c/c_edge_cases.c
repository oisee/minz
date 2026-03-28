// Edge case tests for C frontend stability
// All pure arithmetic — no comparison→return path (VIR condret-sink issue)

unsigned char identity(unsigned char x) { return x; }
unsigned char zero(unsigned char x) { return 0; }
unsigned char add(unsigned char a, unsigned char b) { return a + b; }
unsigned char sub(unsigned char a, unsigned char b) { return a - b; }
unsigned char mul(unsigned char a, unsigned char b) { return a * b; }
unsigned char shl1(unsigned char x) { return x << 1; }
unsigned char shr1(unsigned char x) { return x >> 1; }
unsigned char negate(unsigned char x) { return -x; }
unsigned char complement(unsigned char x) { return ~x; }
unsigned char band(unsigned char a, unsigned char b) { return a & b; }
unsigned char bor(unsigned char a, unsigned char b) { return a | b; }
unsigned char bxor(unsigned char a, unsigned char b) { return a ^ b; }

// Multi-step
unsigned char double_inc(unsigned char x) { return (x + x) + 1; }
unsigned char triple(unsigned char x) { return x + x + x; }
unsigned char square(unsigned char x) { return x * x; }

// assert identity(0) == 0 via mir2
// assert identity(42) == 42 via mir2
// assert identity(255) == 255 via mir2
// assert zero(42) == 0 via mir2
// assert zero(0) == 0 via mir2
// assert add(3, 4) == 7 via mir2
// assert add(200, 55) == 255 via mir2
// assert add(0, 0) == 0 via mir2
// assert sub(10, 3) == 7 via mir2
// assert sub(10, 10) == 0 via mir2
// assert mul(6, 7) == 42 via mir2
// assert mul(0, 255) == 0 via mir2
// assert mul(15, 17) == 255 via mir2
// assert shl1(1) == 2 via mir2
// assert shl1(64) == 128 via mir2
// assert shl1(128) == 0 via mir2
// assert shr1(2) == 1 via mir2
// assert shr1(128) == 64 via mir2
// assert shr1(1) == 0 via mir2
// assert negate(1) == 255 via mir2
// assert negate(0) == 0 via mir2
// assert complement(0) == 255 via mir2
// assert complement(255) == 0 via mir2
// assert complement(170) == 85 via mir2
// assert band(255, 0) == 0 via mir2
// assert band(170, 85) == 0 via mir2
// assert band(255, 255) == 255 via mir2
// assert bor(0, 0) == 0 via mir2
// assert bor(170, 85) == 255 via mir2
// assert bxor(255, 255) == 0 via mir2
// assert bxor(170, 85) == 255 via mir2
// assert double_inc(5) == 11 via mir2
// assert triple(10) == 30 via mir2
// assert square(5) == 25 via mir2
// assert square(0) == 0 via mir2
// assert square(15) == 225 via mir2
