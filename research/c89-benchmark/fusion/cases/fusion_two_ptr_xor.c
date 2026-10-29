unsigned char source[256];
unsigned short n, result;
unsigned char other[256];
void execute(void) {
    unsigned short s = 0, i;
    unsigned char *p = source, *q = other;
    i = n;
    while (i != 0) { s += (unsigned short)(*p ^ *q); ++p; ++q; --i; }
    result = s;
}
