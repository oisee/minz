unsigned char g1 = 107;
unsigned char g3 = 180;
unsigned int t0(unsigned char a, unsigned char b) { unsigned char acc = a ^ b; unsigned char x = (unsigned char)(a + 1); unsigned char r; g3 = (unsigned char)(201 + x); r = (unsigned char)(x + g1); return (unsigned int)r * 256u + (unsigned char)(acc + r); }
// assert t0(146, 0) == 65168 via z80
// assert t0(146, 0) == 65168 via mir2
