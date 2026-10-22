unsigned char source[256];
unsigned short n, result;
unsigned short limit;
void execute(void) {
    unsigned short s = 0, i, count = 0;
    if (n > 4) count = n - 4;
    if (count > limit) count = limit;
    for (i = 0; i < count; ++i) s += source[i + 4];
    result = s;
}
