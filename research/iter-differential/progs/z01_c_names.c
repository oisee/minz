unsigned short sum(unsigned char *p, unsigned char n) { unsigned short s = 0; while (n) { s += *p++; n--; } return s; }
unsigned char count(unsigned char x) { return x + 1; }
unsigned char take(unsigned char a, unsigned char b) { return a * b; }
unsigned char skip(unsigned char a) { return a - 1; }
unsigned char zip(unsigned char a, unsigned char b) { return a ^ b; }
unsigned char map(unsigned char a, unsigned char b) { return a + b; }
unsigned char filter(unsigned char a) { return a > 3; }
unsigned short fold(unsigned short a, unsigned char b, unsigned char c, unsigned char d) { return a + b + c + d; }
unsigned char buf[4] = {1, 2, 3, 4};
unsigned short f(void) { return sum(buf, 4) + count(5) + take(2, 3) + skip(9) + zip(6, 3) + map(1, 2) + filter(7) + fold(100, 1, 2, 3); }
// assert f() == 145 via mir2
// assert f() == 145 via z80
