unsigned char source[64], target[64], n;
unsigned short result;
void copy_bytes(unsigned char *dst, unsigned char *src, unsigned char count) {
    unsigned char i;
    for (i = 0; i < count; i++) dst[i] = src[i];
}
void execute(void) { copy_bytes(target, source, n); result = n; }
