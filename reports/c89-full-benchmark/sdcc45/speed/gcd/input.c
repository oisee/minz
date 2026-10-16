unsigned char a, b;
unsigned short result;
unsigned char gcd(unsigned char x, unsigned char y) {
    if (x == 0) return y;
    if (y == 0) return x;
    while (x != y) {
        if (x > y) x -= y;
        else y -= x;
    }
    return x;
}
void execute(void) { result = gcd(a, b); }
