unsigned char source[256];
unsigned short n, result;
void execute(void) {
    unsigned short s = 0, i;
    for (i = 0; i < n; ++i) if (source[i] >= 128) s += (unsigned char)(source[i] ^ 255);
    result = s;
}
