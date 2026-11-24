unsigned char reduce(unsigned char a) { return a / 2; }
unsigned char forEach(unsigned char a) { return a + 10; }
unsigned char mapInPlace(unsigned char a) { return a + 20; }
unsigned short g(void) { return reduce(9) + forEach(1) + mapInPlace(2); }
// assert g() == 37 via mir2
// assert g() == 37 via z80
