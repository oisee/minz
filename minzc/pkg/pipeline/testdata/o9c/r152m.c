unsigned char g0 = 172;
unsigned char g1 = 70;
unsigned int t1(unsigned char a, unsigned char b) { unsigned char acc = a ^ b; unsigned char x = (unsigned char)(a + 1); unsigned char r; g1 = (unsigned char)(29 - acc); r = (unsigned char)(x + g0); return (unsigned int)r * 256u + (unsigned char)(acc + r); }
// assert t1(1, 1) == 44718 via z80
