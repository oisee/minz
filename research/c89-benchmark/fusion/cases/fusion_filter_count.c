unsigned char source[256];
unsigned short n, result;
unsigned char k;
void execute(void) {
    unsigned short s = 0, i;
    for (i = 0; i < n; ++i) if (source[i] == k) ++s;
    result = s;
}
