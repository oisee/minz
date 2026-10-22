unsigned char source[256];
unsigned short n, result;
void execute(void) {
    unsigned short i; unsigned char s = 255;
    for (i = 0; i < n; ++i) if (source[i] < s) s = source[i];
    result = s;
}
