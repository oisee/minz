unsigned char source[256];
unsigned short n, result;
unsigned char target[256];
void execute(void) {
    unsigned short i;
    for (i = 0; i < n; ++i) target[i] = (unsigned char)(source[i] ^ 255);
    result = n;
}
