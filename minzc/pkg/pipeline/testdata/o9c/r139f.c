unsigned char g1 = 107;
unsigned char g3 = 180;
static unsigned char h0(unsigned char x, unsigned char y) { unsigned char s = x; g3 = (unsigned char)((unsigned char)(130 + 71) + x); s = (unsigned char)(s + g1); return s; }
unsigned int t0(unsigned char a, unsigned char b) { unsigned char acc = a ^ b; unsigned char r = h0((unsigned char)(a + 1), acc); return (unsigned int)r * 256u + (unsigned char)(acc + r); }
// assert t0(146, 0) == 65168 via z80
// assert t0(146, 0) == 65168 via mir2
