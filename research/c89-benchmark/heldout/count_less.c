unsigned char values[256], n, b;
unsigned short result;
unsigned short count_less(unsigned char *p, unsigned char length, unsigned char threshold) {
    unsigned short count = 0;
    unsigned char i;
    for (i = 0; i < length; ++i) {
        if (p[i] < threshold) ++count;
    }
    return count;
}
void execute(void) { result = count_less(values, n, b); }
