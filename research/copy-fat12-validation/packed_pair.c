unsigned char values[512], n;
unsigned short result;
unsigned short unpack_pair(unsigned char *base, unsigned short index) {
    unsigned short pos = index + (index >> 1);
    unsigned short combined = (unsigned short)base[pos] | ((unsigned short)base[pos + 1] << 8);
    if ((index & 1) == 0) return combined & 4095;
    return (combined >> 4) & 4095;
}
void execute(void) { result = unpack_pair(values, n); }
