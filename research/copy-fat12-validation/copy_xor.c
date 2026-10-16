unsigned char source[256], target[256], n;
unsigned short result;
unsigned char transform_bytes(unsigned char *dst, unsigned char *src, unsigned char count) {
    unsigned char i;
    for (i = 0; i < count; i++) dst[i] = src[i] ^ 90;
    return i;
}
void execute(void) { result = transform_bytes(target, source, n); }
