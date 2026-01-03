// 16-bit arithmetic in C for SDCC Z80
// Equivalent to arithmetic_16bit.minz

unsigned int add16(unsigned int a, unsigned int b) {
    return a + b;
}

unsigned int sub16(unsigned int a, unsigned int b) {
    return a - b;
}

unsigned int mul8(unsigned char a, unsigned char b) {
    return (unsigned int)a * (unsigned int)b;
}

void main(void) {
    unsigned int x = add16(1000, 2000);
    unsigned int y = sub16(5000, 1000);
    unsigned int z = mul8(25, 10);
}
