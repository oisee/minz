unsigned char g0 = 152;
unsigned char g2 = 41;
static unsigned char h4(unsigned char x, unsigned char y) { unsigned char s = x; g2 = (unsigned char)((unsigned char)(s & x) ^ (unsigned char)(x + y)); s = (unsigned char)(s + g0); return (unsigned char)((unsigned char)(x & 247) | (unsigned char)(y & s)); }
unsigned int t4(unsigned char a, unsigned char b) { unsigned char acc = a ^ b; unsigned char r = h4((unsigned char)(a + 1), acc); return (unsigned int)r * 256u + (unsigned char)(acc + r); }
