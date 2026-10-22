unsigned char source[256];
unsigned short n, result;
unsigned char other[256];
void execute(void) {
    unsigned short s = 0, i;
    for (i = 0; i < n; ++i) s = (unsigned short)(s + (unsigned short)((unsigned short)source[i] * other[i]));
    result = s;
}
