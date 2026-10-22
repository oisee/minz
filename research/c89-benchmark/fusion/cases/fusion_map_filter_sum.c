unsigned char source[256];
unsigned short n, result;
void execute(void) {
    unsigned short s = 0, i; unsigned char y;
    for (i = 0; i < n; ++i) { y = (unsigned char)(source[i] * 2); if (y > 10 && y < 200) s += y; }
    result = s;
}
