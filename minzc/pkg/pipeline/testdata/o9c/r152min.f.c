unsigned char g0 = 172;
unsigned char g1 = 70;
static unsigned char h1(unsigned char x, unsigned char y) { unsigned char s = x; g1 = (unsigned char)(29 - y); s = (unsigned char)(s + g0); return s; }
unsigned int t1(unsigned char a, unsigned char b) { unsigned char acc = a ^ b; unsigned char r = h1((unsigned char)(a + 1), acc); return (unsigned int)r * 256u + (unsigned char)(acc + r); }
// assert t1(1, 1) == 44718 via z80
