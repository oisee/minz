/* Shared math helpers — imported by other C files */

unsigned char square(unsigned char x) {
    return x * x;
}

unsigned char cube(unsigned char x) {
    return x * x * x;
}

unsigned char min2(unsigned char a, unsigned char b) {
    return (a < b) ? a : b;
}

unsigned char max2(unsigned char a, unsigned char b) {
    return (a > b) ? a : b;
}
