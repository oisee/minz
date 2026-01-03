// Fibonacci in C for SDCC Z80
// Equivalent to fibonacci.minz

unsigned char fibonacci(unsigned char n) {
    if (n <= 1) {
        return n;
    }
    return fibonacci(n - 1) + fibonacci(n - 2);
}

void main(void) {
    unsigned char result = fibonacci(10);
    // Result in A register
}
