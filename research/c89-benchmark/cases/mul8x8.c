unsigned char a, b;
unsigned short result;
void execute(void) {
    unsigned short multiplicand = a;
    unsigned char multiplier = b;
    unsigned short product = 0;
    while (multiplier != 0) {
        if (multiplier & 1) product += multiplicand;
        multiplicand = multiplicand << 1;
        multiplier = multiplier >> 1;
    }
    result = product;
}
